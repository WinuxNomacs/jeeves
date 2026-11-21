// Package apply is `jeeves apply`, the Go port of bin/jeeves-apply: the data repo's home/ copies installed
// into ~/.claude (after a merge or a pull).
//
// A new or changed skill, agent or command carries instructions or code into every session, so it is held
// until the user has seen it and accepted that version (--accept, with the hash the held line prints); a
// later change to it is held again. Deletions are not held. Every file installed or held is checked for
// lines that could make a session run outside code, gain permissions or hooks, or obey text it reads; each
// one found is printed under the file, as "line <n>: <kind>".
//
// A file edited on disk since the last sync is never overwritten without --force. Memories are never
// applied. Only from a data repo on main with no merge in progress and nothing uncommitted under home/, so
// an unfinished merge or another branch never reaches ~/.claude. A file is recorded as synced only once its
// copy is in place.
package apply

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// HeldPath: a shared file that carries instructions or code into every session on every device, installed
// only once the user has accepted that exact version.
func HeldPath(p string) bool {
	return strings.HasPrefix(p, "skills/") || strings.HasPrefix(p, "agents/") || strings.HasPrefix(p, "commands/")
}

type options struct {
	force, show bool
	paths       []string // named after --force
	accepted    []string // <hash>:<path>
}

// forced: --force covers the path (no paths given, or it is one of them).
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

// named: the user named the path after --force.
func (o options) named(p string) bool { return len(o.paths) > 0 && o.forced(p) }

func (o options) acceptedAs(p, hash string) bool {
	for _, a := range o.accepted {
		if a == hash+":"+p {
			return true
		}
	}
	return false
}

func parse(args []string, stderr io.Writer) (options, int) {
	var o options
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--force":
			o.force = true
		case a == "--show":
			o.show = true
		case a == "--accept":
			v := ""
			if i+1 < len(args) {
				v = args[i+1]
			}
			if k := strings.Index(v, ":"); k < 1 || k == len(v)-1 {
				fmt.Fprintln(stderr, "apply: --accept takes <hash>:<path>")
				return o, 2
			}
			o.accepted = append(o.accepted, v)
			i++
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "apply: unknown option %s\n", a)
			return o, 2
		default:
			if !o.force {
				fmt.Fprintln(stderr, "apply: a path goes after --force")
				return o, 2
			}
			o.paths = append(o.paths, a)
		}
	}
	if o.show && (o.force || len(o.accepted) > 0) {
		fmt.Fprintln(stderr, "apply: --show takes no other option")
		return o, 2
	}
	return o, 0
}

// Run:
//
//	jeeves apply                    install files that changed in the repo since the last sync on this device
//	jeeves apply --force [<path>...]
//	                                also overwrite files changed on both sides, and install files holding
//	                                conflict markers; with paths (relative to ~/.claude), only those, and
//	                                whatever their state (the disk copy is saved first)
//	jeeves apply --accept <hash>:<path>
//	                                (repeatable) also install that held file, only while the repo copy is
//	                                still that version
//	jeeves apply --show             what would be installed or removed (and files changed on both sides),
//	                                each with its diff against the disk copy and its flags; installs nothing
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	o, rc := parse(args, stderr)
	if rc != 0 {
		return rc
	}
	say := func(f string, v ...any) { fmt.Fprintf(stdout, "jeeves apply: "+f+"\n", v...) }
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return 1
	}
	lock := lib.TakeLock(r.CryptDir, 30)
	if lock == nil {
		say("another Jeeves run is still at work on %s; nothing installed, try again shortly", r.ShellDir)
		return 1
	}
	defer lock.Release()
	if msg := r.LinkProblem(); msg != "" {
		say("%s; nothing installed", msg)
		return 1
	}

	if b := r.Branch(); b != "main" {
		if b == "" {
			b = "detached HEAD"
		}
		say("%s is on '%s', not main; nothing installed (merge it into main first)", r.ShellDir, b)
		return 1
	}
	if u, _ := r.Out("diff", "--name-only", "--diff-filter=U"); r.Merging() || u != "" {
		say("%s is in the middle of a merge; nothing installed (finish or abort it first)", r.ShellDir)
		return 1
	}
	if st, _ := r.Out("-c", "core.quotepath=false", "status", "--porcelain", "--untracked-files=all", "--", "home"); st != "" {
		say("%s has uncommitted changes under home/; nothing installed (commit them, or run jeeves-snapshot)", r.ShellDir)
		return 1
	}

	bases := r.Bases()
	// A first sync of skills, agents and commands on this device: none of them synced here before.
	first := true
	for p := range bases {
		if HeldPath(p) {
			first = false
		}
	}
	stamp := time.Now().Format("20060102-150405")
	savedDir := r.StateDir + "/overwritten/" + stamp
	var installed, removed, skipped, held []string
	flagged := map[string][]string{}
	status, saved, shown := 0, false, false
	diskOf := func(p string) string { return e.ClaudeHome + "/" + p }
	repoOf := func(p string) string { return r.HomeCopy + "/" + p }
	flagsOf := func(p string) []string { return lib.FlagLines(lib.AddedLines(diskOf(p), repoOf(p))) }
	// hold holds a skill, agent or command in a version never synced on this device, unless the user
	// accepted it; false when it is not held.
	hold := func(p, d, rh, b string) bool {
		if !(rh != "-" && rh != b && HeldPath(p) && !o.acceptedAs(p, rh)) {
			return false
		}
		kind := "changed"
		if d == "-" {
			kind = "new"
		}
		held = append(held, rh+":"+p+" ("+kind+")")
		if f := flagsOf(p); len(f) > 0 {
			flagged[p] = f
		}
		return true
	}

	for _, p := range lib.AllTracked(e.ClaudeHome, r.HomeCopy) {
		d, _ := lib.HashOf(diskOf(p))
		rh, _ := lib.HashOf(repoOf(p))
		b := r.BaseOf(p)
		state := lib.Classify(d, rh, b)
		// Named after --force: the repo copy, whatever the state (a disk deletion held back, a merged conflict).
		if !o.show && state != "insync" && state != "repo" && (o.named(p) || (state == "conflict" && o.forced(p))) {
			if hold(p, d, rh, b) {
				continue
			}
			if d != "-" {
				if lib.CopyFile(diskOf(p), savedDir+"/"+p) != nil {
					skipped = append(skipped, p+" (its disk copy could not be saved first, so it was left as it is)")
					continue
				}
				saved = true
			}
			state = "repo"
		}
		if o.show {
			if state != "repo" && state != "conflict" {
				continue
			}
			shown = true
			if rh == "-" {
				fmt.Fprintf(stdout, "=== would remove %s (deleted in the data repo)\n", p)
				continue
			}
			what := "would install"
			if HeldPath(p) && rh != b {
				what = "held: accept with --accept " + rh + ":" + p
				if d == "-" {
					what = "new, " + what
				}
			}
			if state == "conflict" {
				what = "changed on both sides, installed only with --force " + p + "; " + what
			}
			fmt.Fprintf(stdout, "=== %s (%s)\n", p, what)
			for _, f := range flagsOf(p) {
				fmt.Fprintf(stdout, "flag: %s\n", f)
			}
			old := diskOf(p)
			if fi, err := os.Stat(old); err != nil || !fi.Mode().IsRegular() {
				old = "/dev/null"
			}
			diff := exec.Command("git", "-c", "core.autocrlf=false", "-c", "core.quotepath=false", "diff", "--no-index",
				"--no-color", "--no-ext-diff", "--no-textconv", "--", old, repoOf(p))
			diff.Stdout, diff.Stderr = stdout, stderr
			diff.Run()
			continue
		}
		switch state {
		case "insync":
			r.SetBase(p, d, diskOf(p))
		case "repo":
			if rh != "-" && lib.HasConflictMarkers(repoOf(p)) && !o.forced(p) {
				skipped = append(skipped, p+" (the repo copy holds conflict markers from an unfinished merge; if they belong there, run jeeves-apply --force "+p+")")
				continue
			}
			if hold(p, d, rh, b) {
				continue
			}
			var flags []string
			if rh != "-" {
				flags = flagsOf(p)
			}
			if rh == "-" {
				if now, _ := lib.HashOf(diskOf(p)); now != d {
					skipped = append(skipped, p+" (changed on disk just now; run jeeves-snapshot first)")
					continue
				}
				if err := os.Remove(diskOf(p)); err != nil && !os.IsNotExist(err) {
					skipped = append(skipped, p+" (could not be removed)")
					continue
				}
				removed = append(removed, p)
			} else {
				switch install(diskOf(p), repoOf(p), d) {
				case lib.Copied:
					installed = append(installed, p)
					if len(flags) > 0 {
						flagged[p] = flags
					}
				case lib.Settling:
					skipped = append(skipped, p+" (changed on disk just now; run jeeves-snapshot first)")
					continue
				default:
					skipped = append(skipped, p+" (could not be written; disk full or read-only?)")
					continue
				}
			}
			if r.SetBase(p, rh, repoOf(p)) != nil {
				say("could not update the sync record for %s (disk full?)", p)
			}
		case "disk":
			skipped = append(skipped, p+" (changed on disk; run jeeves-snapshot first)")
		case "conflict":
			skipped = append(skipped, p+" (changed on both sides; compare, then jeeves-snapshot --force "+p+" or jeeves-apply --force "+p+")")
		}
	}
	if o.show {
		if !shown {
			say("nothing to install")
		}
		return 0
	}
	r.KeepBases()

	if len(installed) > 0 {
		say("installed %s", strings.Join(installed, " "))
	}
	for _, p := range installed {
		if f := flagged[p]; len(f) > 0 {
			say("flagged in %s:", p)
			for _, l := range f {
				fmt.Fprintf(stdout, "  %s\n", l)
			}
		}
	}
	if len(removed) > 0 {
		say("removed (deleted in the data repo) %s", strings.Join(removed, " "))
	}
	if saved {
		say("previous disk copies saved in %s", r.ShellDir+"/.sync-state/overwritten/"+stamp)
	}
	if len(held) > 0 {
		if first {
			say("first sync of skills, agents and commands on this device: each is held until the user accepts it")
		}
		for _, h := range held {
			_, p, _ := strings.Cut(h, ":")
			if k := strings.LastIndex(p, " ("); k >= 0 {
				p = p[:k]
			}
			say("held %s", h)
			for _, l := range flagged[p] {
				fmt.Fprintf(stdout, "  %s\n", l)
			}
		}
		say("held files are installed only once the user accepts them: jeeves-apply --show shows each one, then " +
			"jeeves-apply --accept <hash>:<path> installs it")
	}
	for _, s := range skipped {
		say("skipped %s", s)
		status = 1
	}
	if len(installed) == 0 && len(removed) == 0 && len(held) == 0 && status == 0 {
		say("nothing to install")
	}
	return status
}

// install puts the repo copy into ~/.claude through a temp file; the disk file must still be the one
// classified (hash want) when the copy goes into place, as a session may have written it meanwhile.
func install(dst, src, want string) int {
	tmp := dst + ".sync-tmp"
	if lib.CopyFile(src, tmp) != nil {
		os.Remove(tmp)
		return lib.Failed
	}
	if now, _ := lib.HashOf(dst); now != want {
		os.Remove(tmp)
		return lib.Settling
	}
	if os.Rename(tmp, dst) != nil {
		os.Remove(tmp)
		return lib.Failed
	}
	return lib.Copied
}
