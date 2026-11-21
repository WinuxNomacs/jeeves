// Package risk is `jeeves risk`, the Go port of bin/jeeves-risk: lines that a push of the data repo's main
// would add under home/ (the files every device installs) and that could make a session run outside code,
// gain permissions or hooks, or obey text it reads. Run after jeeves scan, before every push: a flag is for
// the user to look at and decide on, not proof of harm.
package risk

import (
	"fmt"
	"io"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Run returns 0 when nothing is flagged, 1 with each flag listed as file:line (commit) and its kind.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	if len(args) != 0 {
		fmt.Fprintln(stderr, "risk: takes no options")
		return 2
	}
	say := func(f string, a ...any) { fmt.Fprintf(stdout, "jeeves risk: "+f+"\n", a...) }
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return 1
	}
	rng := r.OutgoingRange()
	if r.EncryptedAt("main") && !e.LoadKey(r) {
		say("the data repo's key is not in this device's key store: run /jeeves:encryption")
		return 1
	}
	var lines []lib.Flag
	for _, l := range r.OutgoingAdded(rng) {
		if strings.HasPrefix(l.File, "home/") {
			lines = append(lines, lib.Flag{Where: l.Where(), Text: l.Text})
		}
	}
	flags := lib.RiskFlags(lines)
	if len(flags) == 0 {
		say("nothing flagged")
		return 0
	}
	for _, f := range flags {
		say("flagged %s: %s", f.Where, f.Kind)
		// The line itself (jeeves scan, run first, has stopped any secret), cut to 200 characters. The
		// bash version looks it up by the text before the first ": ", so a file name holding one shows none.
		where, _, _ := strings.Cut(f.Where+": "+f.Kind, ": ")
		for _, l := range lines {
			if l.Where == where {
				t := []rune(l.Text)
				if len(t) > 200 {
					t = t[:200]
				}
				fmt.Fprintf(stdout, "    %s\n", string(t))
				break
			}
		}
	}
	say("ask the user about each flagged line before pushing: it reaches every device")
	return 1
}
