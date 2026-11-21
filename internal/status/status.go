// Package status is `jeeves status`, the Go port of bin/jeeves-status: what is out of sync between
// ~/.claude and the data repo, changing nothing (except the data repo record, as every command updates it).
package status

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Run prints, tab-separated, one line per tracked path not in sync, then the data repo's state:
//
//	<state> <path> <disk> <repo> <base>     state: disk | repo | conflict
//	datarepo <absolute path of the data repo>
//	branch   <name>  clean | dirty | merging
//	main     even | ahead | behind | diverged | no-remote  <ahead>  <behind>
//
// Returns the exit status.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	if len(args) != 0 {
		fmt.Fprintln(stderr, "status: takes no options")
		return 2
	}
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		fmt.Fprintf(stdout, "jeeves status: %s\n", err)
		return 1
	}
	if msg := r.LinkProblem(); msg != "" {
		fmt.Fprintf(stdout, "jeeves status: %s\n", msg)
		return 1
	}

	bases := r.Bases()
	for _, p := range lib.AllTracked(e.ClaudeHome, r.HomeCopy) {
		d := hash(e.ClaudeHome+"/"+p, stderr)
		rh := hash(r.HomeCopy+"/"+p, stderr)
		b := bases[p]
		if state := lib.Classify(d, rh, b); state != "insync" {
			if b == "" {
				b = "none"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\n", state, p, d, rh, b)
		}
	}

	out, _ := r.Git("symbolic-ref", "--short", "-q", "HEAD")
	branch := strings.TrimRight(out, "\n")
	if branch == "" {
		branch = "detached"
	}
	tree := "clean"
	if fi, err := os.Stat(r.Dir + "/.git/MERGE_HEAD"); err == nil && fi.Mode().IsRegular() {
		tree = "merging"
	} else if out, _ := r.Git("status", "--porcelain"); strings.TrimRight(out, "\n") != "" {
		tree = "dirty"
	}
	fmt.Fprintf(stdout, "datarepo\t%s\n", r.ShellDir)
	fmt.Fprintf(stdout, "branch\t%s\t%s\n", branch, tree)

	if _, err := r.Git("rev-parse", "-q", "--verify", "refs/remotes/origin/main"); err != nil {
		fmt.Fprint(stdout, "main\tno-remote\t0\t0\n")
		return 0
	}
	counts, _ := r.Git("rev-list", "--left-right", "--count", "main...origin/main")
	ahead, behind := "", ""
	if f := strings.Fields(counts); len(f) == 2 {
		ahead, behind = f[0], f[1]
	}
	rel := "diverged"
	switch {
	case ahead == "0" && behind == "0":
		rel = "even"
	case behind == "0":
		rel = "ahead"
	case ahead == "0":
		rel = "behind"
	}
	fmt.Fprintf(stdout, "main\t%s\t%s\t%s\n", rel, ahead, behind)
	return 0
}

func hash(p string, stderr io.Writer) string {
	h, err := lib.HashOf(p)
	if err != nil {
		fmt.Fprintf(stderr, "jeeves status: %s\n", err)
	}
	return h
}
