// Package check is `jeeves check`, the Go port of bin/jeeves-check: what /jeeves:setup needs to know about
// this machine, one fact per line (tab-separated), read-only. JEEVES_GH and JEEVES_SSH name the gh and ssh
// commands (tests use them). Each network check has a deadline.
package check

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

var (
	firstWordRe = regexp.MustCompile(`^[^0-9]*([0-9][0-9.]*)`)
	sshLoginRe  = regexp.MustCompile(`(?m)^Hi ([^!\n]*)! You.ve successfully authenticated`)
)

// firstWord is the first version-like number on the first line.
func firstWord(s string) string {
	if m := firstWordRe.FindStringSubmatch(lib.FirstLine(s)); m != nil {
		return m[1]
	}
	return ""
}

// Run prints, one fact per line (tab-separated):
//
//	os           linux | wsl | macos | windows
//	git          ok <version> | missing
//	gh           ok <version> | missing                 the GitHub CLI
//	gh-auth      ok <login> | fail <why> | skip          signed in to GitHub through gh
//	gh-protocol  https | ssh | skip                      how gh clones (its git_protocol setting)
//	ssh-github   ok <login> | fail <why> | skip          an SSH key GitHub accepts
//	gh-install   <command> | none                        how to install gh here (run only after the user agrees)
//	age          ok <version> | old <version> | missing  age, for encryption (1.3.0 or later, with age-plugin-batchpass)
//	age-install  <command> | none                        how to install age here (none: the release download)
//	keystore     keychain | wincred | secret-service | none  where an encryption key would be kept
//	datarepo     <path> repo | empty | missing | other
//
// datarepo: the data repo folder (JEEVES_DATA or --data-repo, the plugin option data_repo, or
// ~/jeeves-data), ~/ expanded; repo: a git repo; empty: an empty folder; missing: nothing there; other: a
// folder with files that is not a git repo. Exit status 0, or 2 for an argument.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	if len(args) != 0 {
		fmt.Fprintln(stderr, "check: takes no options")
		return 2
	}
	line := func(f ...string) { fmt.Fprintln(stdout, strings.Join(f, "\t")) }
	output := func(name string, a ...string) string {
		out, _ := lib.Command(name, a...).Output()
		return string(out)
	}
	has := func(name string) bool { return lib.LookPath(name) != "" }
	gh, ssh := os.Getenv("JEEVES_GH"), os.Getenv("JEEVES_SSH")
	if gh == "" {
		gh = "gh"
	}
	if ssh == "" {
		ssh = "ssh"
	}

	osName := lib.OSName()
	line("os", osName)

	if has("git") {
		line("git", "ok", firstWord(output("git", "--version")))
	} else {
		line("git", "missing")
	}

	if has(gh) {
		line("gh", "ok", firstWord(output(gh, "--version")))
		cmd := lib.Command(gh, "api", "user", "--jq", ".login")
		var out bytes.Buffer
		cmd.Stdout = &out
		if lib.RunDeadline(15, cmd) == 0 && strings.TrimRight(out.String(), "\n") != "" {
			line("gh-auth", "ok", strings.TrimRight(out.String(), "\n"))
		} else {
			line("gh-auth", "fail", "not signed in, or GitHub could not be reached")
		}
		if strings.TrimRight(output(gh, "config", "get", "git_protocol"), "\n") == "ssh" {
			line("gh-protocol", "ssh")
		} else {
			line("gh-protocol", "https")
		}
	} else {
		line("gh", "missing")
		line("gh-auth", "skip")
		line("gh-protocol", "skip")
	}

	// GitHub answers an accepted key with "Hi <login>! You've successfully authenticated" and exit status
	// 1. StrictHostKeyChecking=yes: never adds GitHub's host key to known_hosts on the user's behalf.
	if has(ssh) {
		cmd := lib.Command(ssh, "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=yes", "git@github.com")
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		rc := lib.RunDeadline(10, cmd)
		switch m := sshLoginRe.FindStringSubmatch(out.String()); {
		case m != nil && m[1] != "":
			line("ssh-github", "ok", m[1])
		case rc == 124:
			line("ssh-github", "fail", "no answer from github.com within 10 seconds")
		case strings.Contains(strings.ToLower(out.String()), "host key verification failed"):
			line("ssh-github", "fail", "github.com is not in known_hosts")
		default:
			line("ssh-github", "fail", "no SSH key that GitHub accepts")
		}
	} else {
		line("ssh-github", "skip")
	}

	winget := has("winget") || has("winget.exe")
	install := "none"
	switch osName {
	case "macos":
		if has("brew") {
			install = "brew install gh"
		}
	case "windows":
		if winget {
			install = "winget install --id GitHub.cli -e"
		}
	default:
		switch {
		case has("apt-get"):
			install = "sudo apt install gh"
		case has("dnf"):
			install = "sudo dnf install gh"
		case has("pacman"):
			install = "sudo pacman -S github-cli"
		case has("brew"):
			install = "brew install gh"
		}
	}
	line("gh-install", install)

	switch v := lib.AgeVersion(); {
	case v == "":
		line("age", "missing")
	case lib.VersionAtLeast(v, lib.AgeMinVersion) && has("age-plugin-batchpass"):
		line("age", "ok", v)
	default:
		line("age", "old", v)
	}
	// Linux packages are often older than 1.3 (Ubuntu 24.04 apt: 1.2.1), so only Homebrew is offered there.
	install = "none"
	if osName == "windows" {
		if winget {
			install = "winget install --id FiloSottile.age -e"
		}
	} else if has("brew") {
		install = "brew install age"
	}
	line("age-install", install)
	line("keystore", lib.KeystoreKind(""))

	dir := a.JeevesData
	if dir == "" {
		dir = lib.NativePath(os.Getenv("CLAUDE_PLUGIN_OPTION_DATA_REPO"))
	}
	if dir == "" {
		dir = e.Home + "/" + lib.DefaultDataRepoName
	}
	dir = e.ExpandPath(dir)
	shown := lib.ShellPath(dir)
	if _, err := os.Stat(dir); err != nil {
		line("datarepo", shown, "missing")
	} else if _, err := os.Stat(dir + "/.git"); err == nil {
		line("datarepo", shown, "repo")
	} else if ents, err := os.ReadDir(dir); err == nil && len(ents) == 0 {
		line("datarepo", shown, "empty")
	} else {
		line("datarepo", shown, "other")
	}
	return 0
}
