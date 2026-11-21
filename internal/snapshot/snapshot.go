// Package snapshot is `jeeves snapshot`, the Go port of bin/jeeves-snapshot: the ~/.claude preferences
// recorded in the data repo and committed on its main.
//
// It never pushes (at session start it runs the automatic cleanup, which may). The hosts/<host>/ backup is
// only ever written by that host and is never applied back. A shared file whose name another OS can't hold
// is not taken. What is committed is checked to be what was copied (decrypted, byte for byte) before the
// commit, and that is what the sync record keeps: a copy that fails stops the run with nothing recorded,
// and a file still being written is left for the next snapshot.
package snapshot

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/apply"
	"github.com/WinuxNomacs/jeeves/internal/cleanup"
	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/updatecheck"
)

// hostMark is the file in hosts/<host>/ that holds the machine ID of the host that wrote it.
const hostMark = ".jeeves-machine"

// oldHostFiles are the per-device files backups held before 2.0 (Jeeves keeps knowledge files only, the
// user's decision of 2026-11-20). A copy already in hosts/<host>/ stays as it is, never removed silently,
// until the user deletes it.
var oldHostFiles = []string{"settings.json", "keybindings.json"}

type options struct {
	hook, force, pull, replaceHost bool
	paths                          []string
	allowDelete                    map[string]bool
}

func (o options) forced(p string) bool {
	if !o.force {
		return false
	}
	if len(o.paths) == 0 {
		return true
	}
	for _, f := range o.paths {
		if f == p {
			return true
		}
	}
	return false
}

func parse(args []string, stderr io.Writer) (options, int) {
	o := options{pull: true, allowDelete: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--hook":
			o.hook = true
		case a == "--force":
			o.force = true
		case a == "--no-pull":
			o.pull = false
		case a == "--replace-host":
			o.replaceHost = true
		case a == "--allow-delete":
			d := ""
			if i+1 < len(args) {
				d = strings.TrimSuffix(args[i+1], "/")
			}
			ok := false
			for _, t := range lib.TrackedDirs {
				ok = ok || d == t
			}
			if !ok {
				fmt.Fprintf(stderr, "snapshot: --allow-delete takes one of: %s\n", strings.Join(lib.TrackedDirs, " "))
				return o, 2
			}
			o.allowDelete[d] = true
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "snapshot: unknown option %s\n", a)
			return o, 2
		default:
			if !o.force {
				fmt.Fprintln(stderr, "snapshot: a path goes after --force")
				return o, 2
			}
			o.paths = append(o.paths, a)
		}
	}
	return o, 0
}

type run struct {
	e      lib.Env
	r      *lib.Repo
	o      options
	stdout io.Writer
	stderr io.Writer
}

func (s *run) say(f string, v ...any) { fmt.Fprintf(s.stdout, "jeeves snapshot: "+f+"\n", v...) }

// Run:
//
//	jeeves snapshot           pull (fast-forward only), copy changed shared files into home/, back up this
//	                          device's memories to hosts/<host>/, commit
//	jeeves snapshot --force [<path>...]
//	                          also take the disk copy of files changed on both sides (conflicts), or holding
//	                          conflict markers; with paths, only of those (relative to ~/.claude)
//	jeeves snapshot --hook    for the hooks: always exits 0, prints only when something needs attention
//	jeeves snapshot --no-pull skip contacting the remote
//	jeeves snapshot --allow-delete <folder>
//	                          record that agents/, commands/ or skills/ is gone from ~/.claude (a folder that
//	                          vanishes whole is otherwise held back: it may be a moved folder or a dead link)
//	jeeves snapshot --replace-host
//	                          back up into hosts/<host>/ even though another machine of that name wrote it
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	o, rc := parse(args, stderr)
	if rc != 0 {
		return rc
	}
	s := &run{e: e, o: o, stdout: stdout, stderr: stderr}
	return s.finish(s.main(a))
}

func (s *run) finish(rc int) int {
	if s.o.hook {
		return 0
	}
	return rc
}

func (s *run) main(a lib.DataRepoArg) int {
	e, o := s.e, s.o
	// From a hook, first the check that the plugin is its signed release: on a difference, its notice and
	// nothing else; when the signature cannot be checked here, its notice, then on. A background update's
	// own session touches nothing: the run that started it does the work.
	if o.hook && os.Getenv("JEEVES_AUTO_CHILD") == "1" {
		return 0
	}
	if o.hook {
		verify := lib.Command(lib.NativePath(e.Root())+"/bin/jeeves-verify", "--hook")
		var out bytes.Buffer
		verify.Stdout, verify.Stderr = &out, &out
		vrc := lib.ExitCode(verify.Run())
		text := strings.TrimRight(out.String(), "\n")
		switch vrc {
		case 0:
		case 3:
			fmt.Fprintln(s.stdout, text)
		default:
			fmt.Fprintln(s.stdout, text)
			return 0
		}
		// At session start (not after an edit), at most once a day: is a newer release out?
		if o.pull {
			updatecheck.Run(e, nil, s.stdout, s.stderr)
		}
	}

	r, err := lib.FindDataRepo(e, a, s.stderr)
	if err != nil {
		s.say("skipped, %s", err)
		return 1
	}
	s.r = r
	// Another session's snapshot is running: it records the same changes, so the hook stays quiet.
	lock := lib.TakeLock(r.CryptDir, 30)
	if lock == nil {
		if !o.hook {
			s.say("skipped, another Jeeves run is still at work on %s; try again shortly", r.ShellDir)
		}
		return 1
	}
	defer lock.Release()

	// Guard: only commit onto a clean main, never in the middle of other work.
	if b := r.Branch(); b != "main" {
		if b == "" {
			b = "detached HEAD"
		}
		s.say("skipped, %s is on '%s', not main; ~/.claude changes are not recorded yet", r.ShellDir, b)
		return 1
	}
	if !s.noLinks() {
		return 1
	}
	if st, _ := r.Out("status", "--porcelain"); r.Merging() || (st != "" && !s.onlyOwnLeftovers()) {
		s.say("skipped, %s has uncommitted changes; ~/.claude changes are not recorded yet", r.ShellDir)
		return 1
	}

	s.pullOrigin()
	// What the pull brought in is checked too.
	if !s.noLinks() {
		return 1
	}

	// A ~/.claude with none of the shared files, on a device that has synced them: most likely the folder
	// moved or was emptied, and taking that would delete them on every device. Record nothing and say so.
	if len(lib.TrackedUnder(e.ClaudeHome)) == 0 && len(lib.TrackedUnder(r.HomeCopy)) > 0 && s.syncedShared() {
		s.say("skipped, %s holds none of the shared files (CLAUDE.md, agents/, commands/, skills/) that "+
			"this device synced before; recording that would delete them on every device. If that is what you "+
			"want, remove them from home/ in the data repo with /jeeves:update", lib.ShellPath(e.ClaudeHome))
		return 1
	}
	if r.EncryptedAt("origin/main") && r.PlainHistoryAt("origin/main") {
		s.say("%s", lib.PlainHistoryMsg)
	}
	if !s.byteForByte() {
		return 1
	}
	return s.record()
}

// noLinks: the data repo holds no link (said, nothing recorded, when it does).
func (s *run) noLinks() bool {
	if msg := s.r.LinkProblem(); msg != "" {
		s.say("skipped, %s", msg)
		return false
	}
	return true
}

// onlyOwnLeftovers: the uncommitted changes are only what an earlier snapshot left (it failed or was
// stopped between copying and committing): home/ files that match ~/.claude, and this device's own backup.
func (s *run) onlyOwnLeftovers() bool {
	host := s.e.Host
	for _, p := range s.changed() {
		switch {
		case strings.HasPrefix(p, "hosts/"+host+"/"), strings.HasPrefix(p, "memory/"+host+"/"), p == "hosts/.memory-review":
		case strings.HasPrefix(p, "home/"):
			q := p[5:]
			repo, disk := s.r.HomeCopy+"/"+q, s.e.ClaudeHome+"/"+q
			if fi, err := os.Stat(repo); err == nil && fi.Mode().IsRegular() {
				if !sameFile(repo, disk) {
					return false
				}
			} else if _, err := os.Stat(disk); err == nil {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func sameFile(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// syncedShared: the sync record holds a shared path (not only settings or keybindings).
func (s *run) syncedShared() bool {
	for p := range s.r.Bases() {
		if p != "settings.json" && p != "keybindings.json" {
			return true
		}
	}
	return false
}

// pullOrigin brings in what other devices pushed. Offline or slow is fine: carry on locally.
func (s *run) pullOrigin() {
	r, e := s.r, s.e
	lastOrigin := r.RevParse("refs/remotes/origin/main")
	if !s.o.pull || r.GitQuiet("remote", "get-url", "origin") != nil {
		return
	}
	if r.RemoteGit(nil, nil, "fetch", "-q", "origin", "main") != 0 {
		if !s.o.hook {
			s.say("could not reach origin, recording locally")
		}
		return
	}
	switch {
	case r.RevParse("HEAD") == r.RevParse("origin/main"):
	case r.TreeProblem("origin/main") != "":
		s.say("origin/main is not taken: %s; remove them there (git rm, then commit and push from that device), then sync again", r.TreeProblem("origin/main"))
	case r.EncryptedAt("origin/main") && !r.Unlocked():
		s.say("another device encrypted the data repo; this device is locked, so its changes are not taken: run /jeeves:encryption")
	case r.EncryptedAt("origin/main") && !e.LoadKey(r):
		s.say("the data repo's key is not in this device's key store, so other devices' changes are not taken: run /jeeves:encryption")
	case r.GitQuiet("merge-base", "HEAD", "origin/main") != nil && r.CleanedRoot("origin/main"):
		s.switchToCleanedHistory(lastOrigin)
	case r.GitQuiet("merge-base", "--is-ancestor", "HEAD", "origin/main") == nil:
		if _, err := r.Git("merge", "-q", "--ff-only", "origin/main"); err != nil {
			s.say("could not take origin/main (git could not check out its files here; a name this OS cannot hold?); recording locally")
		}
	case r.GitQuiet("merge-base", "--is-ancestor", "origin/main", "HEAD") != nil:
		s.say("main and origin/main have diverged; recording locally, merge them before pushing")
	}
}

// switchToCleanedHistory: another device cleaned up the history, so origin/main shares no commit with main.
// This device's old history goes into a bundle and the new one is taken. Files this device changed in
// commits it never pushed get, as their last synced version, the one it last shared with origin, so the
// disk loop records them again (or reports a conflict) instead of taking them for older than the repo's.
func (s *run) switchToCleanedHistory(lastOrigin string) {
	r := s.r
	bundle := r.StateDir + "/" + lib.CleanupBundleName
	// The old history is dropped below (reset, then gc): without its bundle, keep everything as it is.
	if os.MkdirAll(r.StateDir, 0o777) != nil || r.GitQuiet("bundle", "create", bundle+".sync-tmp", "main") != nil ||
		os.Rename(bundle+".sync-tmp", bundle) != nil {
		os.Remove(bundle + ".sync-tmp")
		s.say("the history was cleaned up on another device, but this device's old history could not be saved " +
			"(disk full?); nothing switched, recording locally")
		return
	}
	if lastOrigin != "" && r.GitQuiet("cat-file", "-e", lastOrigin+"^{commit}") == nil {
		names, _ := r.Raw("-c", "core.quotepath=false", "diff", "--name-only", "-z", lastOrigin, "HEAD", "--", "home/")
		tmp := r.StateDir + "/base.sync-tmp"
		for _, n := range strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00") {
			if n == "" {
				continue
			}
			p := strings.TrimPrefix(n, "home/")
			if r.GitQuiet("cat-file", "-e", lastOrigin+":home/"+p) == nil {
				if b, err := r.Raw("cat-file", "--filters", lastOrigin+":home/"+p); err == nil && os.WriteFile(tmp, b, 0o666) == nil {
					h, _ := lib.HashOf(tmp)
					r.SetBase(p, h, tmp)
				}
			} else {
				r.SetBase(p, "-", "")
			}
		}
		os.Remove(tmp)
	} else {
		// Nothing known as shared: every file that differs from the repo becomes a conflict to merge.
		for _, p := range lib.AllTracked(s.e.ClaudeHome, r.HomeCopy) {
			r.SetBase(p, "-", "")
		}
	}
	r.Git("reset", "-q", "--hard", "origin/main")
	r.ShrinkRepo()
	s.say("the history was cleaned up on another device; switched to it (this device's old history is in "+
		".sync-state/%s)", lib.CleanupBundleName)
}

// byteForByte: files are kept byte for byte. A data repo made before 1.1 converts line endings (text=auto
// eol=lf), which changes what is committed; its .gitattributes gets "* -text" instead, committed on its
// own, once. False when that failed.
func (s *run) byteForByte() bool {
	r := s.r
	b, err := os.ReadFile(r.Dir + "/.gitattributes")
	if err != nil {
		return true
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n")
	found := false
	for _, l := range lines {
		found = found || l == "* text=auto eol=lf"
	}
	if !found {
		return true
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var out strings.Builder
	for _, l := range lines {
		switch l {
		case "# LF everywhere, also in a Windows checkout (Git for Windows sets core.autocrlf=true).":
			l = "# Byte for byte: no line-ending conversion, also in a Windows checkout (core.autocrlf)."
		case "* text=auto eol=lf":
			l = "* -text"
		}
		out.WriteString(l + "\n")
	}
	tmp := r.Dir + "/.gitattributes.sync-tmp"
	if os.WriteFile(tmp, []byte(out.String()), 0o666) == nil && os.Rename(tmp, r.Dir+"/.gitattributes") == nil &&
		r.GitQuiet("add", "--", ".gitattributes") == nil &&
		r.GitQuiet("commit", "-q", "-m", "Keep files byte for byte (no line-ending conversion)", "--", ".gitattributes") == nil {
		if !s.o.hook {
			s.say("committed .gitattributes: files are now kept byte for byte")
		}
		return true
	}
	os.Remove(tmp)
	s.say("could not update .gitattributes to keep files byte for byte; nothing recorded")
	return false
}

// filesInDir are the tracked files in one folder under root, following the folder itself when it is a link.
func filesInDir(root, dir string) []string {
	base := root + "/" + dir
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil
	}
	var out []string
	filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(real, p)
		f := dir + "/" + filepath.ToSlash(rel)
		if !strings.Contains(f, "\n") && !lib.Excluded(f) {
			out = append(out, f)
		}
		return nil
	})
	return out
}

// changed are the paths with uncommitted changes, whole whatever their names hold (git quotes a name with a
// space or a quote in its plain output).
func (s *run) changed(paths ...string) []string {
	args := append([]string{"status", "--porcelain", "-z", "--no-renames", "--untracked-files=all", "--"}, paths...)
	out, _ := s.r.Raw(args...)
	var res []string
	for _, e := range strings.Split(string(out), "\x00") {
		if len(e) > 3 {
			res = append(res, e[3:])
		}
	}
	return res
}

// undo takes back what a failed run copied into home/ and hosts/<host>/: the files as the last commit has
// them, and none it added. Left in place, a copy that a later edit on disk no longer matches would stop
// every snapshot (and apply) as uncommitted changes. What cannot be put back is left for the next run.
func (s *run) undo() {
	r := s.r
	r.GitQuiet("reset", "-q")
	for _, d := range []string{"home", "hosts/" + s.e.Host} {
		if r.GitQuiet("cat-file", "-e", "HEAD:"+d) == nil {
			r.GitQuiet("checkout", "-q", "HEAD", "--", d)
		}
		r.GitQuiet("clean", "-q", "-f", "-d", "--", d)
	}
}

// record copies what changed on disk into home/, backs up this device into hosts/<host>/, commits, and
// reports what needs attention.
func (s *run) record() int {
	r, e, o := s.r, s.e, s.o
	bases := r.Bases()

	// A shared folder gone from ~/.claude (moved, deleted, or a link to a drive that is not there) while
	// this device synced files in it: its files are not recorded as deleted until the user confirms.
	var held []string
	for _, dir := range lib.TrackedDirs {
		if o.allowDelete[dir] {
			continue
		}
		if fi, err := os.Stat(e.ClaudeHome + "/" + dir); err == nil && fi.IsDir() {
			continue // an empty folder still there: its files were deleted
		}
		synced := false
		for p := range bases {
			synced = synced || strings.HasPrefix(p, dir+"/")
		}
		if synced && len(filesInDir(r.HomeCopy, dir)) > 0 {
			held = append(held, dir)
		}
	}
	isHeld := func(p string) bool {
		for _, h := range held {
			if strings.HasPrefix(p, h+"/") {
				return true
			}
		}
		return false
	}

	type taken struct{ path, hash string }
	var took []taken
	var pending, conflicts, refused, unreadable, deleted, busy []string
	heldCount := 0
	// home/ copies an earlier snapshot left uncommitted: checked and recorded below like a copy of this run's.
	left := map[string]bool{}
	for _, p := range s.changed("home") {
		left[strings.TrimPrefix(p, "home/")] = true
	}
	all := lib.AllTracked(e.ClaudeHome, r.HomeCopy)
	collisions := lib.CaseCollisions(all)
	for _, p := range all {
		disk, repo := e.ClaudeHome+"/"+p, r.HomeCopy+"/"+p
		d, derr := lib.HashOf(disk)
		rh, rerr := lib.HashOf(repo)
		if derr != nil || rerr != nil {
			// One file that cannot be read (its permissions) is reported, and the rest recorded.
			unreadable = append(unreadable, p)
			continue
		}
		state := lib.Classify(d, rh, r.BaseOf(p))
		if state == "conflict" && o.forced(p) {
			state = "disk"
		}
		if state == "disk" && d == "-" && isHeld(p) {
			heldCount++
			continue
		}
		switch state {
		case "insync":
			if left[p] {
				took = append(took, taken{p, d})
			} else {
				r.SetBase(p, d, disk)
			}
		case "disk":
			why := ""
			if d != "-" {
				why = lib.BadName(p)
				if why == "" && collisions[p] {
					why = "another file's name differs from it only in case, which collides on Windows and macOS"
				}
				if why == "" && lib.HasConflictMarkers(disk) && !o.forced(p) {
					why = "it holds conflict markers from an unfinished merge; if they belong there, take it with 'jeeves-snapshot --force " + p + "'"
				}
			}
			if why != "" {
				refused = append(refused, p+" ("+why+")")
				continue
			}
			h := "-"
			if d == "-" {
				if err := os.Remove(repo); err != nil && !os.IsNotExist(err) {
					s.say("could not remove home/%s; nothing recorded", p)
					s.undo()
					return 1
				}
				deleted = append(deleted, p)
			} else {
				var rc int
				h, rc = lib.CopySettled(disk, repo)
				if rc == lib.Settling {
					busy = append(busy, p)
					continue
				}
				if rc != lib.Copied {
					s.say("could not copy %s into the data repo (disk full?); nothing recorded", p)
					s.undo()
					return 1
				}
			}
			took = append(took, taken{p, h})
		case "repo":
			pending = append(pending, p)
		case "conflict":
			conflicts = append(conflicts, p)
		}
	}

	hostClash, ok := s.backup()
	if !ok {
		return 1
	}
	// Layout before per-device backups: per-device files were shared in home/, memories in memory/<host>/.
	for _, f := range oldHostFiles {
		os.Remove(r.HomeCopy + "/" + f)
		r.SetBase(f, "-", "")
	}
	os.RemoveAll(r.Dir + "/memory/" + e.Host)

	var errBuf bytes.Buffer
	add := func(args ...string) bool {
		errBuf.Reset()
		cmd := lib.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false",
			"-c", "core.excludesFile=", "-c", "core.hooksPath=" + r.NoHooksDir(), "-c", "core.fsmonitor=false"}, args...)...)
		cmd.Stdout, cmd.Stderr = &errBuf, &errBuf
		return cmd.Run() == nil
	}
	cannot := func() int {
		s.undo()
		var lines []string
		for _, l := range strings.Split(strings.TrimRight(errBuf.String(), "\n"), "\n") {
			if !strings.HasPrefix(l, "warning") && len(lines) < 2 {
				lines = append(lines, l)
			}
		}
		s.say("could not record the changes: %s", strings.Join(lines, " "))
		return 1
	}
	// Encryption failed (the filter is required), or git could not read a file: record nothing, so the
	// next snapshot tries again.
	if !add("add", "-A") {
		return cannot()
	}
	// What the commit would hold must be what was copied. git compares only size and whole-second times,
	// so a file rewritten with the same size in the same second is added again in full; then each taken
	// file's staged content, decrypted, must hash as its copy did.
	var renorm []string
	for _, t := range took {
		if t.hash != "-" {
			renorm = append(renorm, "home/"+t.path)
		}
	}
	if len(renorm) > 0 && !add(append([]string{"add", "--renormalize", "--"}, renorm...)...) {
		return cannot()
	}
	for _, t := range took {
		if t.hash == "-" {
			if r.RevParse(":home/"+t.path) == "" {
				continue
			}
		} else if r.StagedIs("home/"+t.path, t.hash) {
			continue
		}
		s.undo()
		s.say("the commit would not hold %s as it is in ~/.claude (git would change it: check the data repo's "+
			".gitattributes and filters); nothing recorded", t.path)
		return 1
	}
	if r.GitQuiet("diff", "--cached", "--quiet") != nil {
		names, _ := r.Out("diff", "--cached", "--name-only")
		seen := map[string]bool{}
		var parts []string
		for _, n := range strings.Split(names, "\n") {
			n = strings.TrimPrefix(n, "home/")
			switch {
			case strings.HasPrefix(n, "hosts/"+e.Host+"/projects/"), strings.HasPrefix(n, "memory/"+e.Host+"/"):
				n = "memories"
			case strings.HasPrefix(n, "hosts/"+e.Host+"/"):
				n = "backup " + strings.TrimPrefix(n, "hosts/"+e.Host+"/")
			}
			if !seen[n] {
				seen[n] = true
				parts = append(parts, n)
			}
		}
		sort.Strings(parts)
		changed := strings.Join(parts, " ")
		if r.GitQuiet("commit", "-q", "-m", "snapshot("+e.Host+"): "+changed,
			"-m", "Recorded by jeeves-snapshot from "+lib.ShellPath(e.ClaudeHome)+".") != nil {
			s.undo()
			s.say("commit failed; nothing recorded")
			return 1
		}
		// At session start, a routine commit needs no attention; a deletion does (it reaches every device).
		if !o.hook {
			s.say("committed %s", changed)
		}
		if len(deleted) > 0 {
			s.say("deleted from the shared files, so from every device at its next sync: %s", strings.Join(deleted, " "))
		}
	}
	// The base is what was copied and committed, never the disk file as it is now (it may have changed since).
	for _, t := range took {
		if r.SetBase(t.path, t.hash, r.HomeCopy+"/"+t.path) != nil {
			s.say("could not update the sync record (disk full?); the next snapshot will try again")
		}
	}
	r.KeepBases()

	status := 0
	if len(held) > 0 {
		s.say("%s/ gone from %s (moved or deleted, or a link that points nowhere), so its %d files are not deleted "+
			"from the shared files; to delete them on every device or to put them back, run /jeeves:update",
			strings.Join(held, "/ "), lib.ShellPath(e.ClaudeHome), heldCount)
		status = 1
	}
	if hostClash {
		s.say("backup skipped: hosts/%s/ in the data repo belongs to another machine named %s; set JEEVES_HOST to "+
			"a name of its own for this machine (or, if this machine replaces that one, run jeeves-snapshot --replace-host)", e.Host, e.Host)
		status = 1
	}
	if len(busy) > 0 && !o.hook {
		s.say("still being written, so left for the next snapshot: %s", strings.Join(busy, " "))
	}
	if len(unreadable) > 0 {
		s.say("not taken, as it could not be read (check its permissions): %s", strings.Join(unreadable, " "))
	}
	if len(refused) > 0 {
		s.say("not taken, so every device can hold it: %s; rename it in ~/.claude", strings.Join(refused, "; "))
		status = 1
	}
	// Newer skills, agents and commands are held by jeeves-apply until the user accepts them.
	var newer, waiting []string
	for _, p := range pending {
		if fi, err := os.Stat(r.HomeCopy + "/" + p); apply.HeldPath(p) && err == nil && fi.Mode().IsRegular() {
			waiting = append(waiting, p)
		} else {
			newer = append(newer, p)
		}
	}
	if len(newer) > 0 {
		s.say("the repo has newer %s; run jeeves-apply to install them, or /jeeves:update", strings.Join(newer, " "))
	}
	if len(waiting) > 0 {
		s.say("new or changed skills, agents or commands in the repo wait for the user's OK before they are installed: "+
			"%s; /jeeves:update shows them and asks", strings.Join(waiting, " "))
	}
	if len(conflicts) > 0 {
		s.say("changed both on disk and in the repo: %s; merge them with /jeeves:update, or keep the disk copy with "+
			"'jeeves-snapshot --force <path>' or the repo copy with 'jeeves-apply --force <path>'", strings.Join(conflicts, " "))
		status = 1
	}
	// At session start, the automatic history cleanup, when jeeves.conf turns it on and one is due.
	if o.hook && o.pull {
		key, had := os.LookupEnv("JEEVES_AGE_KEY")
		os.Setenv("JEEVES_AGE_KEY", "")
		cleanup.Run(e, []string{"--auto"}, s.stdout, s.stderr)
		if had {
			os.Setenv("JEEVES_AGE_KEY", key)
		}
	}
	return status
}

// backup mirrors this device's memory folders into hosts/<host>/ (laid out as in ~/.claude), so deletions
// show up in history too. It is built aside first, so a failed copy leaves the last backup whole; a
// settings.json or keybindings.json an older version backed up is carried over unchanged. hosts/<host>/
// holds the machine's ID: another machine reporting the same name is stopped. Returns whether another
// machine owns the name, and false when the backup failed (said, nothing recorded).
func (s *run) backup() (clash bool, ok bool) {
	r, e := s.r, s.e
	hostDir := r.Dir + "/hosts/" + e.Host
	mine := e.MachineID()
	theirs := ""
	if b, err := os.ReadFile(hostDir + "/" + hostMark); err == nil {
		theirs = strings.TrimRight(string(b), "\n")
	}
	if mine != "" && theirs != "" && theirs != mine && !s.o.replaceHost {
		return true, true
	}
	tmp := r.StateDir + "/host.sync-tmp"
	good := os.RemoveAll(tmp) == nil && os.MkdirAll(tmp, 0o777) == nil
	for _, f := range oldHostFiles {
		if fi, err := os.Lstat(hostDir + "/" + f); good && err == nil && fi.Mode().IsRegular() {
			good = lib.CopyFile(hostDir+"/"+f, tmp+"/"+f) == nil
		}
	}
	dirs, _ := filepath.Glob(e.ClaudeHome + "/projects/*/memory")
	sort.Strings(dirs)
	for _, dir := range dirs {
		if !good {
			break
		}
		key := filepath.Base(filepath.Dir(dir))
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || strings.HasPrefix(key, ".") {
			continue
		}
		if ents, _ := os.ReadDir(dir); len(ents) == 0 {
			continue
		}
		skipped, err := copyTree(dir, tmp+"/projects/"+key+"/memory")
		good = err == nil
		for _, f := range skipped {
			s.say("not backed up: ~/.claude/projects/%s/memory/%s (its name cannot be held on every OS; rename it to back it up)", key, f)
		}
	}
	// The ID goes with a backup that holds something (an empty one has nothing to overwrite).
	if ents, _ := os.ReadDir(tmp); good && mine != "" && len(ents) > 0 {
		good = os.WriteFile(tmp+"/"+hostMark, []byte(mine+"\n"), 0o666) == nil
	}
	if good {
		good = os.RemoveAll(hostDir) == nil && os.MkdirAll(r.Dir+"/hosts", 0o777) == nil && os.Rename(tmp, hostDir) == nil
	}
	if !good {
		os.RemoveAll(tmp)
		s.say("could not back up this device's memories into hosts/%s/ (disk full?); nothing recorded", e.Host)
		s.undo()
		return false, false
	}
	return false, true
}

// copyTree copies a folder's files and folders (cp -pR: modes and times kept). A link inside is not
// followed and not copied: Jeeves never commits a link. A file whose name another OS cannot hold is left
// out (a checkout on that OS would fail, and the device would stop taking changes) and returned.
func copyTree(src, dst string) (skipped []string, err error) {
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel != "." && lib.BadName(filepath.ToSlash(rel)) != "" {
			skipped = append(skipped, filepath.ToSlash(rel))
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o777)
		case d.Type().IsRegular():
			return lib.CopyFile(p, target)
		}
		return nil
	})
	return skipped, err
}
