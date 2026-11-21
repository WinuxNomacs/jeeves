package lib

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// LookPath is bash's `command -v <name>`: a name with a slash is that file, otherwise the first match in
// PATH. On Windows a file without .exe counts when it is a script (starts with #!), as Git Bash runs one
// (the tests' stand-ins are such scripts); <name>.exe is tried too. Returns "" when none is found.
func LookPath(name string) string {
	if name == "" {
		return ""
	}
	if strings.ContainsAny(name, `/\`) {
		return runnable(NativePath(name))
	}
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if d == "" {
			d = "."
		}
		if p := runnable(filepath.Join(d, name)); p != "" {
			return p
		}
	}
	return ""
}

func runnable(p string) string {
	if runtime.GOOS == "windows" {
		// Lstat, not Stat: an app alias (winget.exe in WindowsApps) is a reparse point Stat cannot open.
		for _, c := range []string{p, p + ".exe"} {
			if fi, err := os.Lstat(c); err == nil && !fi.IsDir() && (strings.HasSuffix(strings.ToLower(c), ".exe") || isScript(c)) {
				return c
			}
		}
		return ""
	}
	if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
		return p
	}
	return ""
}

func isScript(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 2)
	n, _ := f.Read(b)
	return n == 2 && string(b) == "#!"
}

// Command is exec.Command for a program found by LookPath (name unchanged when none is found, so the run
// fails as bash's would). On Windows a script is run by Git Bash's bash.
func Command(name string, args ...string) *exec.Cmd {
	p := LookPath(name)
	if p == "" {
		return exec.Command(name, args...)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(p), ".exe") {
		return exec.Command("bash", append([]string{p}, args...)...)
	}
	return exec.Command(p, args...)
}

// RunDeadline runs cmd with a deadline in seconds, as lib.sh's with_deadline: past it the command is
// stopped by its own PID, gracefully and then by force a second later, and the status is 124. Otherwise the
// status is the command's own (127 when it cannot start).
func RunDeadline(secs int, cmd *exec.Cmd) int {
	cmd.WaitDelay = 2 * time.Second // a grandchild holding the output open does not hold this up
	if err := cmd.Start(); err != nil {
		return 127
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return ExitCode(err)
	case <-time.After(time.Duration(secs) * time.Second):
	}
	if cmd.Process.Signal(syscall.SIGTERM) != nil {
		cmd.Process.Kill()
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		cmd.Process.Kill()
		<-done
	}
	return 124
}

// ExitCode is the shell's $? for a finished command's error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if c := ee.ExitCode(); c >= 0 {
			return c
		}
		return 128 + 15 // ended by a signal
	}
	return 127
}

// FirstLine is the first line of s, without its line break.
func FirstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

// OSName is this machine's OS as Jeeves names it: linux, wsl, macos or windows (Git Bash).
func OSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	}
	if b, err := os.ReadFile("/proc/version"); err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft") {
		return "wsl"
	}
	return "linux"
}

// ShellArg is a path argument as the bash version would print it: on Windows, Git Bash hands a native
// program an argument that looks like a path in Windows form (C:\... or C:/...), which ShellPath turns
// back; anything else (a relative path, a URL) is returned as it is, with no cygpath run.
func ShellArg(p string) string {
	if runtime.GOOS == "windows" && len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return ShellPath(p)
	}
	return p
}
