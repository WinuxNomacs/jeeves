package version

import (
	"io"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const revertUsage = `jeeves-revert [-m 1] <commit>`

var hexCommit = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// Revert is `jeeves revert`: a commit undone by a new one on the current branch (main, or a branch a skill
// made); `-m 1` for a merge commit. A revert that conflicts is undone and its files listed (exit 3).
func Revert(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	vals, rest, ok := parse(args, map[string]bool{"-m": true})
	if !ok || len(rest) != 1 || !hexCommit.MatchString(rest[0]) || (vals["-m"] != "" && vals["-m"] != "1") {
		return usage(stderr, revertUsage)
	}
	c := rest[0]
	s := open(e, "revert", a, false, stdout, stderr)
	if s == nil {
		return 1
	}
	defer s.close()
	if s.r.Merging() || len(s.staged()) > 0 {
		s.say("%s is in the middle of a merge or has staged changes; nothing reverted", s.r.ShellDir)
		return 1
	}
	if b := s.branch(); b != "main" && !validBranch(b) {
		s.say("%s is on %s, not main or a branch a skill made; nothing reverted", s.r.ShellDir, b)
		return 1
	}
	if s.r.RevParse(c+"^{commit}") == "" {
		s.say("there is no commit %s (a history cleanup gives every commit a new hash: find it by its message); nothing reverted", c)
		return 1
	}
	cmd := []string{"revert", "--no-edit"}
	if vals["-m"] != "" {
		cmd = append(cmd, "-m", "1")
	}
	if rc, out := s.git(append(cmd, c)...); rc != 0 {
		files := s.unmerged()
		s.git("revert", "--abort")
		if len(files) == 0 {
			io.WriteString(s.stderr, out)
			s.say("the revert failed; nothing changed")
			return 1
		}
		s.say("reverting %s conflicts in %s (a later change touched the same lines); nothing changed", c, strings.Join(files, ", "))
		return 3
	}
	s.say("reverted %s with %s on %s", c, s.short("HEAD"), s.branch())
	return 0
}
