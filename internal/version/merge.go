package version

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const mergeUsage = `jeeves-merge <pref|conf|review>/<name> -m <summary>
       jeeves-merge origin -m <what came in>
       jeeves-merge --continue | --abort`

// Merge is `jeeves merge`:
//
//	<branch> -m <summary>   a branch a skill made, merged into main (--no-ff, "Merge branch '<branch>':
//	                        <summary>"), then removed
//	origin -m <what>        origin/main merged into main ("Merge origin/main: <what>"). Conflicts under
//	                        hosts/ are settled by rule (each host's folder is written by that host alone:
//	                        this one's side for hosts/<this host>/, the incoming side for any other; the
//	                        review record keeps every memory's newest line); any left under home/ stop it
//	                        with exit 3 and their list, for the skill to settle by editing the files
//	--continue              once each listed file holds no conflict markers: staged and committed
//	--abort                 main put back as it was before the merge
func Merge(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	vals, rest, ok := parse(args, map[string]bool{"-m": true})
	if !ok || len(rest) != 1 {
		return usage(stderr, mergeUsage)
	}
	what, msg := rest[0], vals["-m"]
	if (what == "--continue" || what == "--abort") != (msg == "") {
		return usage(stderr, mergeUsage)
	}
	s := open(e, "merge", a, what == "--abort", stdout, stderr)
	if s == nil {
		return 1
	}
	defer s.close()
	switch what {
	case "--abort":
		return s.abort()
	case "--continue":
		return s.continueMerge()
	}
	if s.r.Merging() {
		s.say("%s is in the middle of a merge already: jeeves-merge --continue or --abort; nothing merged", s.r.ShellDir)
		return 1
	}
	if len(s.staged()) > 0 || s.out("status", "--porcelain", "--untracked-files=no") != "" {
		s.say("%s has uncommitted changes; nothing merged", s.r.ShellDir)
		return 1
	}
	// A skill merges its branch from that branch: main is checked out first.
	if cur := s.branch(); cur != "main" && cur == what && validBranch(cur) {
		if !s.mustGit("switch", "-q", "main") {
			s.say("could not check out main; nothing merged")
			return 1
		}
	}
	if s.branch() != "main" {
		s.say("merges go into main, and %s is on %s; nothing merged", s.r.ShellDir, s.branch())
		return 1
	}
	if what == "origin" {
		return s.mergeOrigin(msg)
	}
	return s.mergeBranch(what, msg)
}

func (s *session) abort() int {
	if !s.r.Merging() {
		s.say("%s is not in the middle of a merge; nothing changed", s.r.ShellDir)
		return 0
	}
	if !s.mustGit("merge", "--abort") {
		s.say("could not undo the merge")
		return 1
	}
	s.say("the merge is undone: main is as it was before it")
	return 0
}

func (s *session) mergeBranch(b, summary string) int {
	if !validBranch(b) {
		s.say("%s is not a branch this merges (pref/, conf/ or review/, then a plain name); nothing merged", b)
		return 1
	}
	if s.r.RevParse("refs/heads/"+b) == "" {
		s.say("there is no branch %s; nothing merged", b)
		return 1
	}
	if rc, _ := s.git("merge", "-q", "--no-ff", "-m", "Merge branch '"+b+"': "+summary, "refs/heads/"+b); rc != 0 {
		files := s.unmerged()
		if s.r.Merging() {
			s.git("merge", "--abort")
		}
		s.say("merging %s into main conflicts in %s (main moved since the branch began); nothing merged", b, strings.Join(files, ", "))
		return 1
	}
	s.git("branch", "-q", "-D", b) // merged: nothing on it is lost
	s.say("merged %s into main (%s) and removed the branch", b, s.short("HEAD"))
	return 0
}

func (s *session) mergeOrigin(what string) int {
	origin := s.r.RevParse("refs/remotes/origin/main")
	if origin == "" {
		s.say("there is no origin/main to merge (run jeeves-snapshot, which fetches it); nothing merged")
		return 1
	}
	if s.r.GitQuiet("merge-base", "--is-ancestor", origin, "main") == nil {
		s.say("main already holds origin/main; nothing to merge")
		return 0
	}
	if why := s.r.TreeProblem(origin); why != "" {
		s.say("origin/main is not merged: %s; remove them there (git rm, then commit and push from that device); nothing merged", why)
		return 1
	}
	rc, out := s.git("merge", "-q", "--no-ff", "-m", "Merge origin/main: "+what, origin)
	if rc == 0 {
		s.say("merged origin/main into main (%s)", s.short("HEAD"))
		return 0
	}
	if !s.r.Merging() {
		io.WriteString(s.stderr, out)
		s.say("the merge failed; nothing merged")
		return 1
	}
	for _, f := range s.unmerged() {
		if strings.HasPrefix(f, "hosts/") && !s.settleHost(f) {
			s.git("merge", "--abort")
			s.say("could not settle %s by rule; nothing merged", f)
			return 1
		}
	}
	return s.stopOrCommit()
}

// stopOrCommit commits a merge with nothing left in conflict, or lists what is (exit 3).
func (s *session) stopOrCommit() int {
	left := s.unmerged()
	if len(left) == 0 {
		if !s.mustGit("commit", "-q", "--no-edit") {
			s.say("the merge commit failed; jeeves-merge --abort undoes the merge")
			return 1
		}
		s.say("merged origin/main into main (%s)", s.short("HEAD"))
		return 0
	}
	s.say("to settle by hand: %s", strings.Join(left, ", "))
	s.say("edit each file to hold both sides' changes, with no conflict markers left, then run jeeves-merge --continue (or jeeves-merge --abort to undo the merge)")
	return 3
}

func (s *session) continueMerge() int {
	if !s.r.Merging() {
		s.say("%s is not in the middle of a merge; nothing to continue", s.r.ShellDir)
		return 1
	}
	files := s.unmerged()
	for _, f := range files {
		p := filepath.Join(s.r.Dir, filepath.FromSlash(f))
		// The encrypted-merge driver's note (the key could not be read) first: it holds markers too.
		if data, err := os.ReadFile(p); err == nil && strings.HasPrefix(string(data), "<<<<<<< jeeves: not merged") {
			s.say("%s was not merged (the key could not be read); run jeeves-merge --abort, then /jeeves:encryption", f)
			return 1
		}
		if lib.HasConflictMarkers(p) {
			s.say("%s still holds conflict markers; nothing committed", f)
			return 1
		}
	}
	if len(files) > 0 && !s.mustGit(append([]string{"--literal-pathspecs", "add", "-A", "--"}, files...)...) {
		s.say("could not stage the settled files; nothing committed")
		return 1
	}
	return s.stopOrCommit()
}

// settleHost settles a conflict under hosts/ by rule.
func (s *session) settleHost(f string) bool {
	if f == "hosts/.memory-review" {
		return s.settleReviewRecord(f)
	}
	side := 3 // another host's folder: theirs
	if strings.HasPrefix(f, "hosts/"+s.e.Host+"/") {
		side = 2
	}
	if s.r.RevParse(":"+string(rune('0'+side))+":"+f) == "" { // deleted on that side
		return s.mustGit("--literal-pathspecs", "rm", "-q", "--", f)
	}
	flag := map[int]string{2: "--ours", 3: "--theirs"}[side]
	return s.mustGit("--literal-pathspecs", "checkout", flag, "--", f) && s.mustGit("--literal-pathspecs", "add", "--", f)
}

// settleReviewRecord keeps one line per memory hash from both sides, the newest (by its date, then its
// text), sorted by hash: the record's own form.
func (s *session) settleReviewRecord(f string) bool {
	best := map[string]string{}
	for _, stage := range []string{":2:", ":3:"} {
		rc, text := s.git("cat-file", "--filters", stage+f)
		if rc != 0 {
			continue
		}
		for _, l := range strings.Split(text, "\n") {
			if l == "" {
				continue
			}
			h, _, _ := strings.Cut(l, "\t")
			if old, ok := best[h]; !ok || newer(l, old) {
				best[h] = l
			}
		}
	}
	hashes := make([]string, 0, len(best))
	for h := range best {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	var b strings.Builder
	for _, h := range hashes {
		b.WriteString(best[h] + "\n")
	}
	if os.WriteFile(filepath.Join(s.r.Dir, "hosts", ".memory-review"), []byte(b.String()), 0o666) != nil {
		return false
	}
	return s.mustGit("add", "--", f)
}

// newer compares two record lines (hash, outcome, date, host) by date, then text.
func newer(a, b string) bool {
	fa, fb := strings.Split(a, "\t"), strings.Split(b, "\t")
	da, db := "", ""
	if len(fa) > 2 {
		da = fa[2]
	}
	if len(fb) > 2 {
		db = fb[2]
	}
	if da != db {
		return da > db
	}
	return a > b
}
