// Package memories is `jeeves memories`, the Go port of bin/jeeves-memories: the memories /jeeves:update's
// session review reads, and the record of those it has reviewed, shared by all devices
// (hosts/.memory-review in the data repo). Memories themselves are never changed.
package memories

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Run:
//
//	jeeves memories list      memory files not reviewed yet, or changed since, one per line:
//	                          <hash> TAB <source> TAB <project> TAB <file>
//	                          source: "this-machine", or the host whose backup in hosts/ holds it; project:
//	                          the project folder's name under projects/; file: the absolute path to read.
//	                          This machine's live memories come first; MEMORY.md (an index) is left out, and
//	                          a file found in several places is listed once.
//	jeeves memories record <hash> <outcome> [<hash> <outcome>...]
//	                          adds each reviewed memory and what came of it (such as "promoted 2, kept 3,
//	                          left out 1") to the record, and commits it on main
//	jeeves memories list --all
//	                          every memory file, reviewed or not, in the same form (for /jeeves:review)
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	say := func(f string, a ...any) { fmt.Fprintf(stdout, "jeeves memories: "+f+"\n", a...) }
	usage := func() int {
		fmt.Fprintln(stderr, "usage: jeeves-memories list [--all] | record <hash> <outcome> [<hash> <outcome>...]")
		return 2
	}
	if len(args) < 1 {
		return usage()
	}
	cmd, args := args[0], args[1:]
	all := false
	switch cmd {
	case "list":
		if len(args) == 1 && args[0] == "--all" {
			all = true
		} else if len(args) != 0 {
			return usage()
		}
	case "record":
		if len(args) < 2 || len(args)%2 != 0 {
			return usage()
		}
	}
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return 1
	}
	record := r.Dir + "/hosts/.memory-review"
	switch cmd {
	case "list":
		return list(e, r, record, all, stdout)
	case "record":
		return rec(e, r, record, args, say)
	}
	return usage()
}

// memoryFiles lists "<hash>\t<source>\t<project>\t<file>" for each memory file under projects (native
// form; shown is the same folder as the scripts print it).
func memoryFiles(projects, shown, source string) []string {
	var res []string
	dirs, _ := filepath.Glob(projects + "/*/memory")
	sort.Strings(dirs)
	for _, dir := range dirs {
		key := filepath.Base(filepath.Dir(dir))
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || strings.HasPrefix(key, ".") {
			continue // bash's glob leaves out names that start with a dot
		}
		var files []string
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.Type().IsRegular() && strings.HasSuffix(d.Name(), ".md") && d.Name() != "MEMORY.md" &&
				!strings.Contains(p, "\n") {
				rel, _ := filepath.Rel(projects, p)
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		sort.Strings(files)
		for _, f := range files {
			h, err := lib.HashOf(projects + "/" + f)
			if err != nil {
				h = ""
			}
			res = append(res, h+"\t"+source+"\t"+key+"\t"+shown+"/"+f)
		}
	}
	return res
}

func list(e lib.Env, r *lib.Repo, record string, all bool, stdout io.Writer) int {
	lines := memoryFiles(e.ClaudeHome+"/projects", lib.ShellPath(e.ClaudeHome)+"/projects", "this-machine")
	hosts, _ := filepath.Glob(r.Dir + "/hosts/*")
	sort.Strings(hosts)
	for _, h := range hosts {
		name := filepath.Base(h)
		if fi, err := os.Stat(h); err != nil || !fi.IsDir() || name == e.Host || strings.HasPrefix(name, ".") {
			continue // this machine's backup: its live memories are listed already
		}
		lines = append(lines, memoryFiles(h+"/projects", r.ShellDir+"/hosts/"+name+"/projects", name)...)
	}
	done := map[string]bool{}
	if !all {
		if b, err := os.ReadFile(record); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				h, _, _ := strings.Cut(l, "\t")
				done[h] = true
			}
		}
	}
	for _, l := range lines {
		h, _, _ := strings.Cut(l, "\t")
		if done[h] {
			continue
		}
		done[h] = true
		fmt.Fprintln(stdout, l)
	}
	return 0
}

func isHash(s string) bool {
	if len(s) < 40 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// outcome is the text recorded for a memory: control characters as spaces, at most 200 characters.
func outcome(s string) string {
	b := []rune(strings.Map(func(c rune) rune {
		if c < 0x20 || c == 0x7f {
			return ' '
		}
		return c
	}, s))
	if len(b) > 200 {
		b = b[:200]
	}
	return string(b)
}

func rec(e lib.Env, r *lib.Repo, record string, args []string, say func(string, ...any)) int {
	if b, _ := r.Git("symbolic-ref", "--short", "-q", "HEAD"); strings.TrimRight(b, "\n") != "main" {
		say("%s is not on main; nothing recorded", r.ShellDir)
		return 1
	}
	if fi, err := os.Stat(r.Dir + "/.git/MERGE_HEAD"); err == nil && fi.Mode().IsRegular() {
		say("%s is in the middle of a merge; nothing recorded", r.ShellDir)
		return 1
	}
	lock := lib.TakeLock(r.CryptDir, 30)
	if lock == nil {
		say("another Jeeves run is still at work on %s; nothing recorded, try again shortly", r.ShellDir)
		return 1
	}
	defer lock.Release()
	newF, oldF := r.CryptDir+"/memory-review.new", r.CryptDir+"/memory-review.old"
	var fresh []string
	for i := 0; i < len(args); i += 2 {
		if !isHash(args[i]) {
			say("not a memory hash: %s; nothing recorded", args[i])
			return 1
		}
		fresh = append(fresh, args[i]+"\t"+outcome(args[i+1]))
	}
	n := len(fresh)
	day := time.Now().Format("2006-01-02")
	host := e.Host
	os.Remove(oldF)
	old, readErr := os.ReadFile(record)
	if readErr == nil {
		if os.WriteFile(oldF, old, 0o666) != nil {
			return 1
		}
	}
	// One line per memory hash, the newest outcome kept, sorted by hash (so two devices rarely touch the
	// same lines; a merge keeps both sides' lines).
	skip := map[string]bool{}
	for _, l := range fresh {
		h, _, _ := strings.Cut(l, "\t")
		skip[h] = true
	}
	var out []string
	if readErr == nil && len(old) > 0 {
		for _, l := range strings.Split(strings.TrimSuffix(string(old), "\n"), "\n") {
			if h, _, _ := strings.Cut(l, "\t"); !skip[h] {
				out = append(out, l)
			}
		}
	}
	for _, l := range fresh {
		out = append(out, l+"\t"+day+"\t"+host)
	}
	key := func(l string) string { h, _, _ := strings.Cut(l, "\t"); return h }
	sort.SliceStable(out, func(i, j int) bool { return key(out[i]) < key(out[j]) })
	var text strings.Builder
	for i, l := range out {
		if i > 0 && key(l) == key(out[i-1]) {
			continue
		}
		text.WriteString(l + "\n")
	}
	if os.WriteFile(newF, []byte(text.String()), 0o666) != nil || os.MkdirAll(r.Dir+"/hosts", 0o777) != nil ||
		os.Rename(newF, record) != nil {
		os.Remove(newF)
		say("could not write the record (disk full?); nothing recorded")
		return 1
	}
	if _, err := r.Git("add", "--", "hosts/.memory-review"); err == nil {
		if _, err := r.Git("commit", "-q", "-m", "memory review("+host+"): "+strconv.Itoa(n)+" memories reviewed", "--", "hosts/.memory-review"); err == nil {
			os.Remove(oldF)
			say("recorded %d reviewed memories", n)
			return 0
		}
	}
	// Put the record back as it was, so no uncommitted change blocks the next snapshot.
	r.Git("reset", "-q", "--", "hosts/.memory-review")
	if readErr == nil {
		os.Rename(oldF, record)
	} else {
		os.Remove(record)
	}
	say("could not commit the record; nothing recorded")
	return 1
}
