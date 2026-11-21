// Package github is `jeeves github`, the Go port of bin/jeeves-github: find or create the user's data repo
// on GitHub, through the GitHub CLI (gh) the user is signed in to. JEEVES_GH names the gh command.
package github

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const topic = "jeeves-data"

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Run: jeeves github find | create <name>. find prints the user's data repos, "<owner/name>\t<clone url>"
// each (a data repo has the GitHub topic jeeves-data, or is named jeeves-data); create makes a private repo
// with that topic and prints its line. The clone URL follows gh's git_protocol. Each call to GitHub has a
// deadline.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	_, args = lib.ParseDataRepoArg(args)
	say := func(f string, a ...any) { fmt.Fprintf(stderr, "jeeves github: "+f+"\n", a...) }
	usage := func() int { fmt.Fprintln(stderr, "usage: jeeves-github find | create <name>"); return 2 }
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "find":
		if len(args) != 1 {
			return usage()
		}
	case "create":
		if len(args) != 2 {
			return usage()
		}
		if !nameRe.MatchString(args[1]) || strings.HasPrefix(args[1], "-") {
			say("a repo name uses only letters, digits, '.', '_' and '-', and does not start with '-'")
			return 2
		}
	default:
		return usage()
	}
	gh := os.Getenv("JEEVES_GH")
	if gh == "" {
		gh = "gh"
	}
	if lib.LookPath(gh) == "" {
		say("the GitHub CLI (gh) is not installed")
		return 1
	}
	proto, _ := lib.Command(gh, "config", "get", "git_protocol").Output()
	field := "url"
	if strings.TrimRight(string(proto), "\n") == "ssh" {
		field = "sshUrl"
	}
	// run runs gh with a deadline: stdout without its trailing line breaks, and the status. errOut gets
	// gh's stderr (nil drops it).
	run := func(secs int, errOut io.Writer, a ...string) (string, int) {
		cmd := lib.Command(gh, a...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, errOut
		rc := lib.RunDeadline(secs, cmd)
		return strings.TrimRight(out.String(), "\n"), rc
	}
	repoLine := func(name string, errOut io.Writer) (string, int) {
		return run(30, errOut, "repo", "view", name, "--json", "nameWithOwner,"+field, "--jq", "[.nameWithOwner, ."+field+"] | @tsv")
	}

	if args[0] == "find" {
		found, rc := run(30, nil, "repo", "list", "--topic", topic, "--limit", "1000",
			"--json", "nameWithOwner,"+field, "--jq", ".[] | [.nameWithOwner, ."+field+"] | @tsv")
		if rc != 0 {
			say("could not list your repositories (not signed in, or GitHub could not be reached)")
			return 1
		}
		named, _ := repoLine(topic, nil)
		seen := map[string]bool{}
		for _, l := range strings.Split(found+"\n"+named, "\n") {
			fs := strings.Fields(l)
			if len(fs) == 0 || seen[fs[0]] {
				continue
			}
			seen[fs[0]] = true
			fmt.Fprintln(stdout, l)
		}
		return 0
	}

	name := args[1]
	var errBuf bytes.Buffer
	if _, rc := run(60, &errBuf, "repo", "create", name, "--private",
		"--description", "Claude Code preferences, versioned by Jeeves"); rc != 0 {
		lines := strings.Split(strings.TrimRight(errBuf.String(), "\n"), "\n")
		if len(lines) > 2 {
			lines = lines[:2]
		}
		say("could not create %s: %s", name, strings.Join(lines, " "))
		return 1
	}
	created, rc := repoLine(name, stderr)
	if rc != 0 {
		say("created %s, but could not read it back", name)
		return 1
	}
	owner, _, _ := strings.Cut(created, "\t")
	if _, rc := run(30, nil, "repo", "edit", owner, "--add-topic", topic); rc != 0 {
		say("created %s, but could not add the topic %s (other devices won't find it by itself)", name, topic)
	}
	fmt.Fprintln(stdout, created)
	return 0
}
