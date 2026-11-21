// Package updatecheck is `jeeves update-check`, the Go port of bin/jeeves-update-check: tells the user when
// a newer Jeeves release is out than the one running. It reads the release the marketplace pins new
// installs to (the "ref" in .claude-plugin/marketplace.json on the public repo's main, fetched anonymously
// with curl, with a deadline) and compares it with .claude-plugin/plugin.json. Not the newest tag: a tag
// comes before its pin. Any failure is silent and waits for the next day. Skipped on a development
// checkout and with JEEVES_UPDATE_CHECK=off. Always exits 0: it runs from the session-start hook.
package updatecheck

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const defaultURL = "https://raw.githubusercontent.com/WinuxNomacs/jeeves/main/.claude-plugin/marketplace.json"

var (
	versionRe = regexp.MustCompile(`(?m)^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"([0-9]*\.[0-9]*\.[0-9]*)"`)
	// Only a ref of exactly vX.Y.Z counts, so nothing else from the remote reaches the message.
	refRe = regexp.MustCompile(`(?m)^[[:space:]]*"ref"[[:space:]]*:[[:space:]]*"v([0-9]{1,9}\.[0-9]{1,9}\.[0-9]{1,9})"`)
)

// Run: jeeves update-check [--now]. At most once a day (--now: without the limit) one line when a newer
// release is out, otherwise nothing.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	_, args = lib.ParseDataRepoArg(args)
	now := len(args) > 0 && args[0] == "--now"
	if os.Getenv("JEEVES_UPDATE_CHECK") == "off" {
		return 0
	}
	root := e.Root()
	if root == "" {
		return 0
	}
	phys, err := filepath.EvalSymlinks(lib.NativePath(root))
	if err != nil || e.IsDevCheckout(phys) {
		return 0
	}
	state := e.ClaudeHome + "/jeeves"
	record := state + "/update-check"
	url := os.Getenv("JEEVES_RELEASES_URL")
	if url == "" {
		url = defaultURL
	}

	// Once a day: the record holds the time of the last check, in seconds. One that does not read, or lies
	// in the future (a clock set back), counts as never checked.
	t := time.Now().Unix()
	if b, err := os.ReadFile(record); err == nil && !now {
		if last, err := strconv.ParseInt(strings.TrimSuffix(lib.FirstLine(string(b)), "\r"), 10, 64); err == nil &&
			isDigits(lib.FirstLine(string(b))) && last <= t && t-last < 86400 {
			return 0
		}
	}
	// Recorded before the check, so a failing network costs one try a day, not one per session.
	if os.MkdirAll(state, 0o777) == nil {
		os.WriteFile(record, []byte(strconv.FormatInt(t, 10)+"\n"), 0o666)
	}

	pj, _ := os.ReadFile(lib.NativePath(root) + "/.claude-plugin/plugin.json")
	m := versionRe.FindSubmatch(pj)
	if m == nil || len(m[1]) == 0 {
		return 0
	}
	ours := string(m[1])

	var pinned []byte
	if strings.HasPrefix(url, "https://") {
		deadline := os.Getenv("JEEVES_UPDATE_DEADLINE")
		if deadline == "" {
			deadline = "10"
		}
		secs, err := strconv.Atoi(deadline)
		if err != nil {
			return 0
		}
		// curl -q skips the user's .curlrc; --proto =https keeps redirects to https.
		cmd := lib.Command("curl", "-q", "-fsSL", "--proto", "=https", "--max-time", deadline, url)
		var out bytes.Buffer
		cmd.Stdout = &out
		lib.RunDeadline(secs, cmd)
		pinned = out.Bytes()
	} else {
		pinned, _ = os.ReadFile(lib.NativePath(url))
	}
	rm := refRe.FindSubmatch(pinned)
	if rm == nil {
		return 0
	}
	latest := string(rm[1])
	if latest == ours || lib.VersionAtLeast(ours, latest) {
		return 0
	}
	fmt.Fprintf(stdout, "jeeves: Jeeves %s is out (this is %s). To update: 'claude plugin marketplace update jeeves', then "+
		"'claude plugin update jeeves@jeeves', then /reload-plugins; or turn on auto-update in /plugin > Marketplaces. "+
		"What changed: https://github.com/WinuxNomacs/jeeves/blob/main/CHANGELOG.md\n", latest, ours)
	return 0
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
