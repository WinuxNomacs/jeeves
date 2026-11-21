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

// Finding is one thing a push must not send: a possible secret or conflict marker at Where (file:line
// (commit c)), or a file under home/ or hosts/ that is not encrypted in an encrypted data repo (Plain;
// never to be accepted).
type Finding struct {
	Msg   string // as Run prints it, after "jeeves scan: "
	Where string
	Plain bool
}

// Findings checks what a push of main would send (the commits in rng) and how many lines they add. The key
// must be loaded (lib.Env.LoadKey) when the data repo is encrypted. An error when the commits could not be
// read: nothing is known about them then, and nothing may be pushed.
func Findings(r *lib.Repo, rng string) ([]Finding, int, error) {
	var found []Finding
	if r.EncryptedAt("main") {
		// Each blob under home/ or hosts/ in the outgoing commits, once, with the first commit that holds it.
		commits, err := r.Git("rev-list", "--abbrev-commit", rng)
		if err != nil {
			return nil, 0, fmt.Errorf("could not list the outgoing commits (%s): %w", rng, err)
		}
		seen := map[string]bool{}
		for _, c := range strings.Fields(commits) {
			tree, err := r.Git("-c", "core.quotepath=false", "ls-tree", "-r", "-z", c, "--", "home", "hosts")
			if err != nil {
				return nil, 0, fmt.Errorf("could not read commit %s: %w", c, err)
			}
			for _, l := range strings.Split(strings.TrimRight(tree, "\x00"), "\x00") {
				meta, path, ok := strings.Cut(l, "\t")
				m := strings.Fields(meta)
				if !ok || len(m) < 3 || seen[m[2]] {
					continue
				}
				seen[m[2]] = true
				blob, err := r.Git("cat-file", "blob", m[2])
				if err != nil {
					return nil, 0, fmt.Errorf("could not read %s in commit %s: %w", path, c, err)
				}
				if !lib.IsAge([]byte(blob)) {
					found = append(found, Finding{Msg: fmt.Sprintf("not encrypted: %s (commit %s)", strings.TrimSpace(path), c), Plain: true})
				}
			}
		}
	}
	added, err := r.OutgoingAdded(rng)
	if err != nil {
		return nil, 0, err
	}
	for _, k := range kinds {
		for _, l := range added {
			if k.re.MatchString(l.Text) {
				found = append(found, Finding{Msg: fmt.Sprintf("possible %s at %s", k.kind, l.Where()), Where: l.Where()})
			}
		}
	}
	return found, len(added), nil
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
	if r.EncryptedAt("main") && !e.LoadKey(r) {
		say("the data repo's key is not in this device's key store: run /jeeves:encryption")
		return 1
	}
	found, added, err := Findings(r, r.OutgoingRange())
	if err != nil {
		say("%s; do not push until it reads", err)
		return 1
	}
	for _, f := range found {
		say("%s", f.Msg)
	}
	if len(found) > 0 {
		say("do not push; fix the commits first (rewrite only commits not yet pushed)")
		return 1
	}
	say("nothing found in %d added lines", added)
	return 0
}
