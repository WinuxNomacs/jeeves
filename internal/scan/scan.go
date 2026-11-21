// Package scan is `jeeves scan`, the Go port of bin/jeeves-scan: secrets in what a push of the data repo's
// main would send (each commit in origin/main..main, or all of main before the first push), and conflict
// markers. Only added lines are checked, and the matched text itself is never printed. In an encrypted data
// repo it reads the plain text (with the key), and also refuses any file under home/ or hosts/ that main
// would send unencrypted.
package scan

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

var kinds = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"private key", regexp.MustCompile(`-----BEGIN ([A-Z]+ )*PRIVATE KEY`)},
	{"Anthropic API key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"GitHub token", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}`)},
	{"AWS access key", regexp.MustCompile(`A(KIA|SIA)[0-9A-Z]{16}`)},
	{"Slack token", regexp.MustCompile(`xox[abprs]-[A-Za-z0-9-]{10,}`)},
	// Not a secret, but an unfinished merge would reach every device.
	{"conflict marker (an unfinished merge)", regexp.MustCompile(`^(<<<<<<<|>>>>>>>)( |$)`)},
	{"key, token, secret or password value", regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?key|token|secret|passw(or)?d)["']?[[:space:]]*[:=][[:space:]]*["']?[A-Za-z0-9_./+=-]{16,}`)},
}

// Run returns 0 when nothing is found, 1 with each finding listed as file:line and its kind.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	if len(args) != 0 {
		fmt.Fprintln(stderr, "scan: takes no options")
		return 2
	}
	say := func(f string, a ...any) { fmt.Fprintf(stdout, "jeeves scan: "+f+"\n", a...) }
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		say("%s", err)
		return 1
	}
	rng := r.OutgoingRange()

	found := false
	if r.EncryptedAt("main") {
		if !e.LoadKey(r) {
			say("the data repo's key is not in this device's key store: run /jeeves:encryption")
			return 1
		}
		// Each blob under home/ or hosts/ in the outgoing commits, once, with the first commit that holds it.
		commits, _ := r.Git("rev-list", "--abbrev-commit", rng)
		seen := map[string]bool{}
		for _, c := range strings.Fields(commits) {
			tree, _ := r.Git("-c", "core.quotepath=false", "ls-tree", "-r", c, "--", "home", "hosts")
			for _, l := range strings.Split(strings.TrimRight(tree, "\n"), "\n") {
				meta, path, ok := strings.Cut(l, "\t")
				m := strings.Fields(meta)
				if !ok || len(m) < 3 || seen[m[2]] {
					continue
				}
				seen[m[2]] = true
				blob, _ := r.Git("cat-file", "blob", m[2])
				if !strings.HasPrefix(blob, lib.AgeMagic) {
					say("not encrypted: %s (commit %s)", strings.TrimSpace(path), c)
					found = true
				}
			}
		}
	}

	added := r.OutgoingAdded(rng)
	for _, k := range kinds {
		for _, l := range added {
			if k.re.MatchString(l.Text) {
				say("possible %s at %s", k.kind, l.Where())
				found = true
			}
		}
	}
	if found {
		say("do not push; fix the commits first (rewrite only commits not yet pushed)")
		return 1
	}
	say("nothing found in %d added lines", len(added))
	return 0
}
