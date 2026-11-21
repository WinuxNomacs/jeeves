package version

import (
	"io"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const commitUsage = `jeeves-commit [--branch pref/<name>|conf/<name>|review/<name>] -m <message> -- <path>...
       jeeves-commit --fold -m <message>`

// Commit is `jeeves commit`: the paths named, as they are on disk (a deletion included), committed on the
// current branch (main, or a branch a skill made); with --branch, that branch started from main first. With
// --fold, the commits not yet pushed are folded into one (a secret that must leave the history before a
// push). Nothing else the index holds is committed: it must hold nothing beforehand.
func Commit(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	// --fold only as the first argument: an allow rule for `jeeves-commit -m ...` must never match a fold,
	// which rewrites commits and is run only with the user seeing it.
	fold := len(args) > 0 && args[0] == "--fold"
	if fold {
		args = args[1:]
	}
	vals, rest, ok := parse(args, map[string]bool{"-m": true, "--branch": true})
	msg, branch := vals["-m"], vals["--branch"]
	if !ok || strings.TrimSpace(msg) == "" || (!fold && (len(rest) < 2 || rest[0] != "--")) || (fold && (branch != "" || len(rest) != 0)) {
		return usage(stderr, commitUsage)
	}
	var paths []string
	if !fold {
		paths = rest[1:]
	}
	s := open(e, "commit", a, true, stdout, stderr) // a commit may remove a link
	if s == nil {
		return 1
	}
	defer s.close()
	if s.r.Merging() {
		s.say("%s is in the middle of a merge: finish it with jeeves-merge --continue or undo it with jeeves-merge --abort; nothing committed", s.r.ShellDir)
		return 1
	}
	if st := s.staged(); len(st) > 0 {
		s.say("the index already holds staged changes (%s); nothing committed", strings.Join(st, ", "))
		return 1
	}
	if fold {
		return s.fold(msg)
	}
	for _, p := range paths {
		if why := committable(p); why != "" {
			s.say("%s: %s; nothing committed", p, why)
			return 1
		}
		if l := linkUnder(s.r.Dir, p); l != "" {
			s.say("%s is a link, which Jeeves never commits; nothing committed", l)
			return 1
		}
	}
	cur := s.branch()
	if branch != "" {
		if !validBranch(branch) {
			s.say("%s is not a branch name this takes (pref/, conf/ or review/, then a plain name); nothing committed", branch)
			return 1
		}
		if cur != "main" {
			s.say("a branch starts from main, and %s is on %s; nothing committed", s.r.ShellDir, cur)
			return 1
		}
		if s.r.RevParse("refs/heads/"+branch) != "" {
			s.say("the branch %s exists already; nothing committed", branch)
			return 1
		}
		if !s.mustGit("switch", "-q", "-c", branch) {
			s.say("could not start the branch %s; nothing committed", branch)
			return 1
		}
		cur = branch
	} else if cur != "main" && !validBranch(cur) {
		s.say("%s is on %s, not main or a branch a skill made; nothing committed", s.r.ShellDir, cur)
		return 1
	}
	undoBranch := func() {
		if branch != "" {
			s.git("switch", "-q", "main")
			s.git("branch", "-q", "-D", branch)
		}
	}
	if !s.mustGit(append([]string{"--literal-pathspecs", "add", "-A", "--"}, paths...)...) {
		s.git(append([]string{"--literal-pathspecs", "reset", "-q", "--"}, paths...)...)
		undoBranch()
		s.say("could not stage %s; nothing committed", strings.Join(paths, ", "))
		return 1
	}
	if len(s.staged()) == 0 {
		undoBranch()
		s.say("nothing changed in %s; nothing committed", strings.Join(paths, ", "))
		return 1
	}
	if !s.mustGit("commit", "-q", "-m", msg) {
		s.git(append([]string{"--literal-pathspecs", "reset", "-q", "--"}, paths...)...)
		undoBranch()
		s.say("the commit failed; nothing committed")
		return 1
	}
	s.say("committed %s on %s: %s", s.short("HEAD"), cur, lib.FirstLine(msg))
	return 0
}

// fold replaces the commits main has that origin/main lacks with one commit of the same files.
func (s *session) fold(msg string) int {
	if s.branch() != "main" {
		s.say("folding is done on main, and %s is on %s; nothing changed", s.r.ShellDir, s.branch())
		return 1
	}
	origin := s.r.RevParse("refs/remotes/origin/main")
	if origin == "" {
		s.say("there is no origin/main to fold onto (nothing was pushed yet); nothing changed")
		return 1
	}
	if st := s.out("-c", "core.quotepath=false", "status", "--porcelain", "--", "home", "hosts", "jeeves.conf", "CLAUDE.md", ".claude"); st != "" {
		s.say("%s has uncommitted changes (commit them first with jeeves-commit, so the fold holds them): %s; nothing changed",
			s.r.ShellDir, strings.ReplaceAll(strings.TrimSpace(st), "\n", "; "))
		return 1
	}
	if s.r.RevParse("main") == origin {
		s.say("main has no commits that are not pushed; nothing changed")
		return 0
	}
	if s.r.GitQuiet("merge-base", "--is-ancestor", origin, "main") != nil {
		s.say("main does not hold origin/main (merge it first with jeeves-merge origin); nothing changed")
		return 1
	}
	before := s.r.RevParse("main")
	if !s.mustGit("reset", "-q", "--soft", origin) {
		return 1
	}
	if len(s.staged()) == 0 {
		s.say("the commits not yet pushed changed nothing in the end; main is now origin/main")
		return 0
	}
	if !s.mustGit("commit", "-q", "-m", msg) {
		s.git("reset", "-q", "--soft", before)
		s.say("the commit failed; main is as it was")
		return 1
	}
	s.say("folded the commits not yet pushed into %s: %s", s.short("HEAD"), lib.FirstLine(msg))
	return 0
}
