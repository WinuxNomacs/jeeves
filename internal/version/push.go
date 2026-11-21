package version

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/risk"
	"github.com/WinuxNomacs/jeeves/internal/scan"
)

const pushUsage = `jeeves-push [--accept '<file>:<line> (commit <c>)'...]`

// Push is `jeeves push`: main pushed to origin, never forced, from a clean main and only once the secrets
// check (jeeves scan) and the risk check (jeeves risk) pass over what it would send. A finding or flag the
// user has looked at and agreed to push is named with --accept, as the check printed its place; a file
// left unencrypted in an encrypted data repo is never pushed.
func Push(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	accepted := map[string]bool{}
	for i := 0; i < len(args); i++ {
		if args[i] != "--accept" || i+1 >= len(args) {
			return usage(stderr, pushUsage)
		}
		accepted[args[i+1]] = true
		i++
	}
	s := open(e, "push", a, false, stdout, stderr)
	if s == nil {
		return 1
	}
	defer s.close()
	if s.branch() != "main" || s.r.Merging() {
		s.say("only a finished main is pushed, and %s is on %s%s; nothing pushed", s.r.ShellDir, s.branch(),
			map[bool]string{true: " mid-merge", false: ""}[s.r.Merging()])
		return 1
	}
	if st := s.out("-c", "core.quotepath=false", "status", "--porcelain", "--", "home", "hosts", "jeeves.conf", "CLAUDE.md", ".claude"); st != "" {
		s.say("%s has uncommitted changes (commit them with jeeves-commit, or undo them): %s; nothing pushed", s.r.ShellDir,
			strings.ReplaceAll(strings.TrimSpace(st), "\n", "; "))
		return 1
	}
	sha := s.r.RevParse("main")
	origin := s.r.RevParse("refs/remotes/origin/main")
	if origin == sha {
		s.say("main is even with origin/main; nothing to push")
		return 0
	}
	rng := s.r.OutgoingRange()
	found, _, err := scan.Findings(s.r, rng)
	if err != nil {
		s.say("%s; nothing pushed", err)
		return 1
	}
	var stop []scan.Finding
	for _, f := range found {
		if f.Plain || !accepted[f.Where] {
			stop = append(stop, f)
		}
	}
	if len(stop) > 0 {
		for _, f := range stop {
			io.WriteString(stdout, "jeeves scan: "+f.Msg+"\n")
		}
		s.say("nothing pushed: a secret is taken out where it lives, then the commits not yet pushed are folded (jeeves-commit --fold); a finding the user confirms is not a secret is pushed with --accept '<its place>'")
		return 1
	}
	// A flag counts only while its line is still in what main would bring every device: a line added and
	// taken out again in commits not yet pushed never reaches one.
	net, err := s.netAdded(origin)
	if err != nil {
		s.say("%s; nothing pushed", err)
		return 1
	}
	all, err := risk.Flags(s.r, rng)
	if err != nil {
		s.say("%s; nothing pushed", err)
		return 1
	}
	var flags []lib.Flag
	for _, f := range all {
		if !accepted[f.Where] && net[flagFile(f.Where)+"\x00"+f.Text] {
			flags = append(flags, f)
		}
	}
	if len(flags) > 0 {
		risk.Print(flags, stdout)
		s.say("nothing pushed: ask the user about each flagged line (it reaches every device); push the ones they agree to with --accept '<its place>'")
		return 1
	}
	var errOut bytes.Buffer
	n := s.out("rev-list", "--count", rng)
	switch rc := s.r.RemoteGit(nil, &errOut, "push", "-q", "origin", sha+":refs/heads/main"); {
	case rc == 124:
		s.say("the push to origin timed out; main is unchanged here, run jeeves-push again later")
		return 1
	case rc != 0 && regexp.MustCompile(`rejected|fetch first|non-fast-forward`).MatchString(errOut.String()):
		s.say("origin has commits this device lacks: run jeeves-snapshot (it fetches), jeeves-merge origin if main diverged, then jeeves-push again; nothing pushed")
		return 1
	case rc != 0:
		io.WriteString(stderr, errOut.String())
		s.say("the push failed; nothing pushed")
		return 1
	}
	s.git("update-ref", "refs/remotes/origin/main", sha)
	s.say("pushed main (%s commits) to origin", n)
	return 0
}

// flagFile is the file of a place printed as "<file>:<line> (commit <c>)".
func flagFile(where string) string {
	w, _, _ := strings.Cut(where, " (commit ")
	if i := strings.LastIndex(w, ":"); i >= 0 {
		return w[:i]
	}
	return w
}

// netAdded lists the lines under home/ that main adds over origin (over nothing before the first push),
// as file NUL text: the change every device would install, decrypted when the data repo is encrypted. The
// diff is pinned as lib.Repo.OutgoingAdded's is, so its names match the flags'; an error when it fails (no
// flag may be dropped because the diff could not be read).
func (s *session) netAdded(origin string) (map[string]bool, error) {
	base := origin
	if base == "" {
		base = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // git's empty tree
	}
	out, err := s.r.Git("-c", "core.quotepath=false", "-c", "diff.mnemonicPrefix=false", "-c", "diff.noprefix=false",
		"diff", "--no-color", "--no-ext-diff", "--textconv", "--text", "--src-prefix=a/", "--dst-prefix=b/",
		"-U0", "--no-renames", base, "main", "--", "home")
	if err != nil {
		return nil, fmt.Errorf("could not compare main with origin/main: %w", err)
	}
	set := map[string]bool{}
	file := ""
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ "):
			file = lib.DiffName(l[4:])
		case strings.HasPrefix(l, "+"):
			set[file+"\x00"+strings.ReplaceAll(l[1:], "\x00", "")] = true
		}
	}
	return set, nil
}
