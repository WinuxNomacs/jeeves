// Package cleanup is `jeeves cleanup`, the Go port of bin/jeeves-cleanup: old history removed from the data
// repo. The commits on main older than <days> days become one parentless commit holding the files as they
// were then; every later commit is recreated on top of it with the same files, authors, dates and messages;
// then main is force-pushed with a lease. No file changes.
//
// jeeves.conf, at the top of the data repo and shared by all devices:
//
//	keep_days=<n>         the days of history a cleanup keeps (1 or more)
//	auto_every_days=<m>   how often the automatic cleanup runs (m >= n); without it, there is none
//
// A cleanup is due when the history's first commit is m days old. The removed history stays on this device
// as .sync-state/history-before-cleanup.bundle (never pushed) until the next cleanup replaces it. This is
// the only force-push in Jeeves: only main, and only with a lease, so a push another device made meanwhile
// is never overwritten. If anything fails, main is put back and nothing is pushed.
package cleanup

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/risk"
	"github.com/WinuxNomacs/jeeves/internal/scan"
)

// count is a whole number of 1 or more, without leading zeros; ok false otherwise.
func count(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1
}

// Run:
//
//	jeeves cleanup <days>            clean up (days: 1 or more), after a fresh snapshot
//	jeeves cleanup --dry-run <days>  say what would be removed, change nothing
//	jeeves cleanup --all             fold all history into one commit (after turning encryption on)
//	jeeves cleanup --auto            for the session-start snapshot: clean up when jeeves.conf says one is
//	                                 due; always exits 0, prints only what needs attention
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	usage := func() int {
		fmt.Fprintln(stderr, "usage: jeeves-cleanup <days> | --dry-run <days> | --all | --auto")
		return 2
	}
	mode, daysArg, all := "run", "", false
	first := ""
	if len(args) > 0 {
		first = args[0]
	}
	switch {
	case first == "--auto":
		mode = "auto"
		if len(args) != 1 {
			return usage()
		}
	case first == "--all":
		all = true
		if len(args) != 1 {
			return usage()
		}
	case first == "--dry-run":
		mode = "dry"
		if len(args) != 2 {
			return usage()
		}
		daysArg = args[1]
	case strings.HasPrefix(first, "-"):
		return usage()
	default:
		if len(args) != 1 {
			return usage()
		}
		daysArg = first
	}
	say := func(f string, v ...any) { fmt.Fprintf(stdout, "jeeves cleanup: "+f+"\n", v...) }
	finish := func(rc int) int {
		if mode == "auto" {
			return 0
		}
		return rc
	}
	// A check the session-start snapshot has already reported (or that will pass later) is silent in --auto.
	skip := func(f string, v ...any) int {
		if mode != "auto" {
			say("skipped, "+f, v...)
		}
		return finish(1)
	}

	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return finish(1)
	}
	now := time.Now().Unix()
	if v := os.Getenv("JEEVES_NOW"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			now = n
		}
	}

	days := 0
	if mode == "auto" {
		everyS := r.ConfValue("auto_every_days")
		if everyS == "" {
			return 0
		}
		every, ok1 := count(everyS)
		d, ok2 := count(r.ConfValue("keep_days"))
		if !ok1 || !ok2 {
			say("automatic cleanup skipped: jeeves.conf needs keep_days and auto_every_days as whole numbers of 1 or more")
			return 0
		}
		days = d
		if every < days {
			say("automatic cleanup skipped: in jeeves.conf, auto_every_days (%d) must be at least keep_days (%d)", every, days)
			return 0
		}
		firsts, _ := r.Out("rev-list", "--first-parent", "main")
		fs := strings.Fields(firsts)
		if len(fs) == 0 {
			return 0
		}
		ct, _ := r.Out("log", "-1", "--format=%ct", fs[len(fs)-1])
		t, _ := strconv.ParseInt(ct, 10, 64)
		if now-t < int64(every)*86400 {
			return 0
		}
	} else if !all {
		d, ok := count(daysArg)
		if !ok {
			say("the days to keep must be a whole number of 1 or more")
			return 2
		}
		days = d
	}

	if mode == "run" {
		snap := lib.Command(lib.NativePath(e.Root()) + "/bin/jeeves-snapshot")
		snap.Stdout, snap.Stderr = stdout, stderr
		snap.Run()
	}
	// One Jeeves run at a time (--auto runs inside the session-start snapshot, which holds the lock already).
	if mode == "run" || all {
		lock := lib.TakeLock(r.CryptDir, 30)
		if lock == nil {
			say("another Jeeves run is still at work on %s; nothing changed, try again shortly", r.ShellDir)
			return 1
		}
		defer lock.Release()
	}

	// Only a clean main that holds everything origin has.
	if b := r.Branch(); b != "main" {
		if b == "" {
			b = "detached HEAD"
		}
		return skip("%s is on '%s', not main", r.ShellDir, b)
	}
	if st, _ := r.Out("status", "--porcelain"); r.Merging() || st != "" {
		return skip("%s has uncommitted changes", r.ShellDir)
	}
	hasOrigin, origin := false, ""
	if r.GitQuiet("remote", "get-url", "origin") == nil {
		hasOrigin = true
		if mode != "dry" && r.RemoteGit(nil, nil, "fetch", "-q", "origin", "main") != 0 {
			return skip("could not reach origin; nothing changed")
		}
		origin = r.RevParse("refs/remotes/origin/main")
		if origin != "" && r.GitQuiet("merge-base", "--is-ancestor", origin, "main") != nil {
			return skip("main does not hold everything origin/main has; sync first (/jeeves:update)")
		}
	}

	// The newest commit on main's first line of history that is older than the cutoff becomes the new root.
	base, keptWhat := "", ""
	if all {
		base, keptWhat = r.RevParse("main"), "keeping no history"
	} else {
		cutoff := now - int64(days)*86400
		log, _ := r.Out("log", "--first-parent", "--format=%H %ct", "main")
		for _, l := range strings.Split(log, "\n") {
			f := strings.Fields(l)
			if len(f) == 2 {
				if t, _ := strconv.ParseInt(f[1], 10, 64); t <= cutoff {
					base = f[0]
					break
				}
			}
		}
		keptWhat = fmt.Sprintf("keeping %d days", days)
	}
	if base == "" || len(parents(r, base)) == 0 {
		if all {
			say("the history is already one commit")
		} else if mode != "auto" {
			say("nothing older than %d days to remove", days)
		}
		return finish(0)
	}
	folded, _ := r.Out("rev-list", "--count", base)
	kept, _ := r.Out("rev-list", "--count", base+"..main")
	when, _ := r.Out("log", "-1", "--format=%ci", base)
	if mode == "dry" {
		say("would fold the %s commits up to %s into one, and keep the %s newer ones", folded, when, kept)
		return 0
	}

	old := r.RevParse("main")
	// What the force-push brings that origin lacks is only what main adds over origin/main (the rebuilt
	// history holds nothing else): it gets the checks jeeves-push runs, before anything changes. Lines
	// origin already has (a finding the user accepted then) are not checked again.
	if hasOrigin && origin != old {
		rng := "main"
		if origin != "" {
			rng = origin + "..main"
		}
		if r.EncryptedAt("main") && !e.LoadKey(r) {
			return skip("the data repo's key is not in this device's key store (run /jeeves:encryption)")
		}
		found, _, err := scan.Findings(r, rng)
		if err != nil {
			return skip("%s", err)
		}
		if len(found) > 0 {
			for _, f := range found {
				fmt.Fprintf(stdout, "jeeves scan: %s\n", f.Msg)
			}
			say("not run, because of the possible secrets above in commits not yet pushed; nothing changed")
			return finish(1)
		}
		flags, err := risk.Flags(r, rng)
		if err != nil {
			return skip("%s", err)
		}
		if len(flags) > 0 {
			risk.Print(flags, stdout)
			say("not run: the commits not yet pushed hold the flagged lines above, which jeeves-push asks the user about first; nothing changed")
			return finish(1)
		}
	}
	os.MkdirAll(r.StateDir, 0o777)
	bundle := r.StateDir + "/" + lib.CleanupBundleName
	fail := func(f string, v ...any) int {
		os.Remove(bundle + ".sync-tmp")
		say(f+"; nothing changed", v...)
		return finish(1)
	}

	msg := fmt.Sprintf("%s history up to %s folded into this commit\n\njeeves-cleanup removed the %s commits up to %s (%s); this commit holds the files as they were then.\n",
		lib.CleanupMark, when, folded, when, keptWhat)
	date := fmt.Sprintf("%d +0000", now)
	root, err := commitTree(r, base+"^{tree}", nil, msg, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date})
	if err != nil {
		return fail("could not write the new first commit")
	}

	// Recreate each later commit, parents first. A parent that was removed maps to the new root.
	mapped := map[string]string{}
	newHead := root
	list, _ := r.Out("rev-list", "--reverse", "--topo-order", base+"..main")
	for _, c := range strings.Fields(list) {
		var ps []string
		for _, p := range parents(r, c) {
			np, ok := mapped[p]
			if !ok {
				np = root
			}
			if !contains(ps, np) {
				ps = append(ps, np)
			}
		}
		who, _ := r.Out("log", "-1", "--format=%an%n%ae%n%ad%n%cn%n%ce%n%cd", "--date=raw", c)
		w := strings.Split(who, "\n")
		for len(w) < 6 {
			w = append(w, "")
		}
		// The message: everything after the headers' blank line.
		raw, _ := r.Raw("cat-file", "commit", c)
		var message []byte
		if k := bytes.Index(raw, []byte("\n\n")); k >= 0 {
			message = raw[k+2:]
		}
		nc, err := commitTree(r, c+"^{tree}", ps, string(message), []string{
			"GIT_AUTHOR_NAME=" + w[0], "GIT_AUTHOR_EMAIL=" + w[1], "GIT_AUTHOR_DATE=" + w[2],
			"GIT_COMMITTER_NAME=" + w[3], "GIT_COMMITTER_EMAIL=" + w[4], "GIT_COMMITTER_DATE=" + w[5]})
		if err != nil {
			return fail("could not recreate commit %s", c)
		}
		mapped[c] = nc
		newHead = nc
	}
	if r.RevParse(newHead+"^{tree}") != r.RevParse("main^{tree}") {
		return fail("the recreated history does not end with the same files")
	}

	// Keep the old history before main moves; it replaces the previous cleanup's only once the push is done.
	if r.GitQuiet("bundle", "create", bundle+".sync-tmp", "main") != nil {
		return fail("could not save the old history")
	}
	// undo puts main back; refused (and said) if something committed onto the new history meanwhile.
	undo := func() {
		if r.GitQuiet("update-ref", "-m", "jeeves-cleanup: undone", "refs/heads/main", old, newHead) == nil {
			os.Remove(bundle + ".sync-tmp")
		} else {
			say("could not put main back (it moved meanwhile); the old history is in .sync-state/%s.sync-tmp", lib.CleanupBundleName)
		}
	}
	if r.GitQuiet("update-ref", "-m", "jeeves-cleanup", "refs/heads/main", newHead, old) != nil {
		return fail("could not move main")
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	stopWatch := make(chan struct{})
	go func() {
		select {
		case <-sig:
			undo()
			os.Exit(1)
		case <-stopWatch:
		}
	}()
	release := func() { signal.Stop(sig); close(stopWatch) }

	if hasOrigin {
		var perr bytes.Buffer
		if r.RemoteGit(&perr, &perr, "push", "-q", "--force-with-lease=main:"+origin, "origin", "main") != 0 {
			release()
			undo()
			lines := strings.Split(strings.TrimRight(perr.String(), "\n"), "\n")
			if len(lines) > 3 {
				lines = lines[:3]
			}
			say("push refused or failed, so main was put back and nothing changed (another device may have pushed " +
				"meanwhile; the cleanup runs again later): " + strings.Join(lines, " "))
			return finish(1)
		}
	}
	release()
	os.Rename(bundle+".sync-tmp", bundle)
	r.ShrinkRepo()
	done := fmt.Sprintf("folded the %s commits up to %s into one and kept the %s newer ones", folded, when, kept)
	if hasOrigin {
		done += "; pushed"
	}
	say("%s (the old history is in .sync-state/%s on this device)", done, lib.CleanupBundleName)
	return finish(0)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// parents are a commit's parents, in order.
func parents(r *lib.Repo, c string) []string {
	out, _ := r.Out("rev-list", "--parents", "-n", "1", c)
	f := strings.Fields(out)
	if len(f) <= 1 {
		return nil
	}
	return f[1:]
}

// commitTree writes a commit of tree with the given parents, message and identity variables.
func commitTree(r *lib.Repo, tree string, ps []string, msg string, env []string) (string, error) {
	args := []string{"-C", r.Dir, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false", "-c", "core.excludesFile=", "commit-tree", tree}
	for _, p := range ps {
		args = append(args, "-p", p)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(msg)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
