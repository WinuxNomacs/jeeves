// Package lib is the Go port of lib/lib.sh: finding the data repo, the tracked paths, hashes and the
// three-way classification. Every behaviour here mirrors the bash version, which stays the reference until
// the port is complete; a difference is a bug in one of them.
package lib

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Env is what lib.sh reads from the environment when it is sourced.
type Env struct {
	Home       string // $HOME
	ClaudeHome string // CLAUDE_HOME, else CLAUDE_CONFIG_DIR, else $HOME/.claude
	Host       string // this device's name in the data repo
	// Root gives the plugin folder (bin/ and lib/ are under it) in the scripts' form. A function, so the
	// conversion (a cygpath run on Windows, about 150 ms there) happens only when a command needs it.
	Root func() string
}

// LoadEnv reads the environment as lib.sh does. root gives the plugin folder.
func LoadEnv(root func() string) Env {
	e := Env{Home: os.Getenv("HOME"), Root: root}
	e.ClaudeHome = firstSet(os.Getenv("CLAUDE_HOME"), os.Getenv("CLAUDE_CONFIG_DIR"), e.Home+"/.claude")
	e.Host = hostName()
	return e
}

func firstSet(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// hostName: JEEVES_HOST, else the host name up to its first dot. Native Windows gets "-win", so it stays
// apart from a WSL on the same PC, which reports the same host name.
func hostName() string {
	if h := os.Getenv("JEEVES_HOST"); h != "" {
		return h
	}
	h, _ := os.Hostname()
	h, _, _ = strings.Cut(h, ".")
	if runtime.GOOS == "windows" && h != "" {
		h += "-win"
	}
	return h
}

// ShellPath is a path as the bash scripts write it. On Windows, Git Bash's runtime hands a native program
// its paths converted to Windows form (C:\Users\...), while the scripts print and record /c/Users/... (and
// /tmp/... for Git Bash's own mounts): cygpath turns one back, by the same mount table. Elsewhere, and when
// cygpath is missing, the path is returned as it is.
func ShellPath(p string) string {
	if runtime.GOOS != "windows" || p == "" || strings.HasPrefix(p, "/") {
		return p
	}
	return cygpath("-u", p)
}

// NativePath is a path the bash scripts wrote (read from a file), in the form this program can open.
func NativePath(p string) string {
	if runtime.GOOS != "windows" || !strings.HasPrefix(p, "/") {
		return p
	}
	return cygpath("-w", p)
}

func cygpath(flag, p string) string {
	out, err := exec.Command("cygpath", flag, "--", p).Output()
	if err != nil {
		return p
	}
	return strings.TrimRight(string(out), "\r\n")
}

// ExpandPath makes a leading ~ or ~/ (as plugin options are written) the home folder.
func (e Env) ExpandPath(p string) string {
	switch {
	case p == "~":
		return e.Home
	case strings.HasPrefix(p, "~/"):
		return e.Home + "/" + p[2:]
	}
	return p
}

// absLogical is `cd p && pwd`: absolute, with . and .. removed by name and symlinks kept, as bash's
// logical pwd gives it.
func absLogical(p string) (string, error) {
	a, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(a)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", &os.PathError{Op: "cd", Path: p, Err: os.ErrInvalid}
	}
	return filepath.ToSlash(a), nil
}
