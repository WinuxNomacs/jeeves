// Package gitcmd is `jeeves git`: git in the data repo, so the skills need no `cd` into it (the Bash tool's
// folder is not kept outside the project, and a permission rule does not match past such a `cd`). It only
// reads: every change to the data repo's history goes through jeeves commit, merge, revert and push, which
// check what they are given. It works on a locked device too.
package gitcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// readOnly are the git commands jeeves git runs: each only reads. A git option before the command (such
// as -c, which could set an alias or a pager to run) is refused, as is an option that writes a file.
var readOnly = map[string]bool{"log": true, "show": true, "diff": true, "status": true, "cat-file": true,
	"ls-files": true, "ls-tree": true, "rev-parse": true, "rev-list": true, "merge-base": true, "blame": true,
	"shortlog": true, "grep": true, "branch": true}

// refusal says why args are not run ("" when they are).
func refusal(args []string) string {
	if len(args) == 0 {
		return "usage: jeeves-git <" + strings.Join(sortedKeys(), "|") + "> [arguments]"
	}
	if !readOnly[args[0]] {
		return fmt.Sprintf("jeeves git only reads (%s); changes go through jeeves-commit, jeeves-merge, jeeves-revert and jeeves-push", strings.Join(sortedKeys(), ", "))
	}
	for _, a := range args[1:] {
		if a == "--" {
			break // paths from here on
		}
		if why := badOption(args[0], a); why != "" {
			return why
		}
	}
	if args[0] == "branch" {
		for _, a := range args[1:] {
			switch a {
			case "--list", "-l", "-a", "--all", "-r", "--remotes", "-v", "-vv", "--verbose", "--show-current", "--merged", "--no-merged", "--contains", "--no-contains", "--format":
			default:
				if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--format=") && !strings.HasPrefix(a, "--sort=") {
					return "jeeves git branch only lists branches: " + a
				}
				if !strings.HasPrefix(a, "-") && !hasListFlag(args[1:]) {
					return "jeeves git branch only lists branches (a name needs --list or --contains)"
				}
			}
		}
	}
	return ""
}

// Options that write a file, run another program, or read a file outside the data repo (some print what
// they read in an error message), as git spells them in full: git takes any unambiguous abbreviation of a
// long option, so an option that is the start of one of these (or one of these with more after it) is
// refused too.
var badLong = []string{"--output", "--output-directory", "--ext-diff", "--open-files-in-pager", "--no-index",
	"--contents", "--orderfile", "--ignore-revs-file", "--file", "--exclude-from", "--pathspec-from-file"}

// Options spelled in full that start like one of those but only read the repo (git takes an exact name
// before an abbreviation).
var harmless = map[string]bool{"--exclude": true, "--output-indicator-new": true, "--output-indicator-old": true,
	"--output-indicator-context": true}

// Short options of the same kinds, by command, also inside a group (-iO): -O runs a program in grep and
// reads an order file elsewhere; grep's -f, blame's -S and ls-files' -X read a file.
var badShort = map[string]string{"grep": "Of", "blame": "OS", "ls-files": "X"}

// badOption says why one argument of a jeeves git command is refused ("" when it is not).
func badOption(cmd, a string) string {
	refuse := "jeeves git does not write files, run other programs or read files outside the data repo: " + a
	if strings.HasPrefix(a, "--") {
		name, _, _ := strings.Cut(a, "=")
		if len(name) < 3 || harmless[name] {
			return ""
		}
		for _, b := range badLong {
			if strings.HasPrefix(b, name) || strings.HasPrefix(name, b+"-") {
				return refuse
			}
		}
		return ""
	}
	if strings.HasPrefix(a, "-") && len(a) > 1 {
		letters := badShort[cmd]
		if cmd != "grep" && cmd != "blame" && cmd != "ls-files" {
			letters = "O"
		}
		if strings.ContainsAny(a[1:], letters) {
			return refuse
		}
	}
	return ""
}

func hasListFlag(args []string) bool {
	for _, a := range args {
		switch a {
		case "--list", "-l", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at":
			return true
		}
	}
	return false
}

func sortedKeys() []string {
	k := make([]string, 0, len(readOnly))
	for c := range readOnly {
		k = append(k, c)
	}
	sort.Strings(k)
	return k
}

// Run runs a git command that only reads, with args, in the data repo and returns git's exit status.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	if why := refusal(args); why != "" {
		fmt.Fprintf(stderr, "jeeves git: %s\n", why)
		return 2
	}
	os.Setenv("ALLOW_LOCKED", "1")
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "jeeves git: %s\n", err)
		return 1
	}
	// The same options as the scripts' git_repo, so jeeves git and jeeves status agree on what is dirty.
	cmd := exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false",
		"-c", "core.excludesFile="}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	signal.Ignore(os.Interrupt) // git gets the terminal's Ctrl+C itself; this waits for it to end
	return lib.ExitCode(cmd.Run())
}
