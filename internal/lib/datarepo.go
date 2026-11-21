package lib

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DefaultDataRepoName is the folder under $HOME used when nothing else names a data repo (the plugin
// option's default, which an install without --config may leave unset).
const DefaultDataRepoName = "jeeves-data"

// Repo is a data repo found by FindDataRepo.
type Repo struct {
	Dir      string // the folder, native form (what this program opens)
	ShellDir string // the folder as the scripts print and record it
	HomeCopy string // Dir/home
	StateDir string // Dir/.sync-state
	CryptDir string // Dir/.git/jeeves
	Stderr   io.Writer
}

// Problem is why no data repo can be used; its text is what the scripts print after "jeeves <command>: ".
type Problem struct{ Msg string }

func (p *Problem) Error() string { return p.Msg }

// DataRepoArg reads `--data-repo <path>` as a command's first two arguments: the same as JEEVES_DATA, and
// recorded. An empty path (the plugin option left empty) means what it does without one, except that the
// record is not read and is removed. A placeholder left unfilled (${user_config.data_repo}, from a plugin
// loaded from a folder, which gets no options) counts as no --data-repo at all. Returns the arguments left.
type DataRepoArg struct {
	JeevesData string // JEEVES_DATA after the option
	Given      bool   // --data-repo was given with a real value (empty included)
}

func ParseDataRepoArg(args []string) (DataRepoArg, []string) {
	a := DataRepoArg{JeevesData: NativePath(os.Getenv("JEEVES_DATA"))}
	if len(args) > 0 && args[0] == "--data-repo" {
		// The option alone counts as given and empty, as lib.sh reads its missing value.
		v := ""
		if len(args) > 1 {
			v = args[1]
		}
		if !strings.HasPrefix(v, "${user_config.") {
			a.JeevesData, a.Given = NativePath(v), true
		}
		if len(args) > 1 {
			args = args[2:]
		} else {
			args = args[1:]
		}
	}
	return a, args
}

func (e Env) pointerFile() string { return e.ClaudeHome + "/jeeves/data-repo" }

// FindDataRepo finds the data repo as lib.sh's find_data_repo does. Order: JEEVES_DATA (or --data-repo),
// then the plugin option (CLAUDE_PLUGIN_OPTION_DATA_REPO), then the path this device last used (the
// record), then the git repo around the current folder if it is a data repo, then ~/jeeves-data.
// It fails when none is found, or when the repo is encrypted and this device is locked (unless
// ALLOW_LOCKED=1). stderr gets the warnings the scripts print there.
func FindDataRepo(e Env, a DataRepoArg, stderr io.Writer) (*Repo, error) {
	option := NativePath(os.Getenv("CLAUDE_PLUGIN_OPTION_DATA_REPO"))
	candidate := e.ExpandPath(firstSet(a.JeevesData, option))
	told := false
	if a.Given {
		if a.JeevesData != "" {
			told = true
		} else {
			os.Remove(e.pointerFile())
		}
	} else if a.JeevesData == "" && option != "" {
		told = true
	}
	if candidate == "" && !a.Given {
		if fi, err := os.Stat(e.pointerFile()); err == nil && fi.Mode().IsRegular() {
			if b, err := os.ReadFile(e.pointerFile()); err == nil {
				line, _, _ := strings.Cut(string(b), "\n")
				candidate = NativePath(strings.TrimSuffix(line, "\r"))
			}
		}
	}
	if candidate == "" {
		candidate = repoAroundHere()
	}
	if candidate == "" {
		candidate = e.Home + "/" + DefaultDataRepoName
		if _, err := os.Stat(candidate + "/.git"); err != nil { // [ -e ]: a broken link counts as none
			return nil, &Problem{fmt.Sprintf("Jeeves is not set up on this machine yet (no data repo at %s, the default): run /jeeves:setup", ShellPath(candidate))}
		}
	}
	if exec.Command("git", "-C", candidate, "rev-parse", "--git-dir").Run() != nil {
		return nil, &Problem{fmt.Sprintf("the data repo %s is not a git repository: to clone or create it, run /jeeves:setup", ShellPath(candidate))}
	}
	// hosts/<host>/ is replaced whole at each snapshot: a missing or odd name must never point elsewhere.
	// A name starting with a dot is refused too: hosts/.memory-review is the shared review record.
	if e.Host == "" || strings.HasPrefix(e.Host, ".") || strings.ContainsAny(e.Host, `/\`) {
		return nil, &Problem{fmt.Sprintf("this device's name could not be found (hostname and uname -n gave '%s'); set JEEVES_HOST", e.Host)}
	}
	dir, err := absLogical(candidate)
	if err != nil {
		return nil, &Problem{fmt.Sprintf("the data repo %s is not a git repository: to clone or create it, run /jeeves:setup", ShellPath(candidate))}
	}
	r := &Repo{Dir: dir, ShellDir: ShellPath(dir), Stderr: stderr}
	r.HomeCopy, r.StateDir, r.CryptDir = dir+"/home", dir+"/.sync-state", dir+"/.git/jeeves"
	if told && !e.rememberDataRepo(r.ShellDir) && a.Given {
		fmt.Fprintf(stderr, "jeeves: could not record the data repo in %s: later commands without --data-repo may not find it\n", ShellPath(e.pointerFile()))
	}
	if r.Encrypted() || r.Unlocked() {
		r.CryptSetup(e)
	}
	if r.Encrypted() && !r.Unlocked() && os.Getenv("ALLOW_LOCKED") != "1" {
		return nil, &Problem{"the data repo is encrypted and this device is locked: run /jeeves:encryption"}
	}
	return r, nil
}

// repoAroundHere is the git repo around the current folder when it is a data repo: home/, and .sync-state/
// ignored as jeeves-init writes it (a dotfiles repo has home/ too). Empty otherwise.
func repoAroundHere() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	top := strings.TrimRight(string(out), "\n")
	if fi, err := os.Stat(top + "/home"); err != nil || !fi.IsDir() {
		return ""
	}
	ign, err := os.ReadFile(top + "/.gitignore")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(ign), "\r", ""), "\n") {
		if l == ".sync-state/" {
			return top
		}
	}
	return ""
}

// rememberDataRepo records the data repo for later bare commands when the record differs; false when it
// could not.
func (e Env) rememberDataRepo(shellDir string) bool {
	f := e.pointerFile()
	if b, err := os.ReadFile(f); err == nil {
		if line, _, _ := strings.Cut(string(b), "\n"); line == shellDir {
			return true
		}
	}
	tmp := f + ".tmp." + strconv.Itoa(os.Getpid())
	ok := os.MkdirAll(filepath.Dir(f), 0o777) == nil &&
		os.WriteFile(tmp, []byte(shellDir+"\n"), 0o666) == nil &&
		os.Rename(tmp, f) == nil
	os.Remove(tmp)
	return ok
}

// nonEmpty is `[ -s file ]`.
func nonEmpty(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Size() > 0
}

// catTrim is `$(cat file)`: the content without its trailing newlines.
func catTrim(p string) string {
	b, _ := os.ReadFile(p)
	return strings.TrimRight(string(b), "\n")
}

// Encrypted: the working tree is encrypted (has a recipient).
func (r *Repo) Encrypted() bool { return nonEmpty(r.Dir + "/recipient") }

// Unlocked: this device holds the key (the marker names the recipient, or encryption is coming from
// origin).
func (r *Repo) Unlocked() bool {
	m := r.CryptDir + "/unlocked"
	return nonEmpty(m) && (!r.Encrypted() || catTrim(m) == catTrim(r.Dir+"/recipient"))
}

// CryptSetup points git at this plugin's jeeves-crypt (the path changes with each plugin version), as
// lib.sh's crypt_setup does, writing the settings only when they differ.
func (r *Repo) CryptSetup(e Env) {
	bin := e.Root() + "/bin/jeeves-crypt"
	want := []string{
		`filter.jeeves.process "` + bin + `" process`,
		`filter.jeeves.clean "` + bin + `" clean %f`,
		`filter.jeeves.smudge "` + bin + `" smudge %f`,
		`filter.jeeves.required true`,
		`diff.jeeves.textconv "` + bin + `" textconv`,
		`merge.jeeves.name Jeeves (encrypted)`,
		`merge.jeeves.driver "` + bin + `" merge %O %A %B %L`,
		`core.checkstat minimal`,
		`core.trustctime false`,
	}
	out, _ := exec.Command("git", "-C", r.Dir, "config", "--get-regexp",
		`^(filter|diff|merge)\.jeeves\.|^core\.(checkstat|trustctime)$`).Output()
	have := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	sort.Strings(have)
	if strings.Join(sorted, "\n") == strings.Join(have, "\n") {
		return
	}
	for _, l := range want {
		k, v, _ := strings.Cut(l, " ")
		exec.Command("git", "-C", r.Dir, "config", k, strings.TrimSpace(v)).Run()
	}
}

// Git runs git in the data repo with the settings every script uses (lib.sh's git_repo): files are copied
// byte for byte, so no line-ending conversion, and the user's own ignore list never applies. Its stderr
// goes to the repo's stderr, as the scripts let git's own messages through. Returns stdout.
func (r *Repo) Git(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false",
		"-c", "core.safecrlf=false", "-c", "core.excludesFile="}, args...)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, r.Stderr
	err := cmd.Run()
	return out.String(), err
}
