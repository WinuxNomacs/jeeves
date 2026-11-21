package lib

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// RemoteGit runs a git command that contacts origin, as lib.sh's remote_git: it never waits for a password
// prompt or a credential window, gives up on a dead ssh host fast (unless the user set their own ssh
// command), and stops after a deadline (JEEVES_REMOTE_DEADLINE, 30 seconds). Fetch and push never decrypt,
// so the key stays out of them. stdout and stderr go where the caller says (nil drops them). Returns the
// exit status, 124 past the deadline.
func (r *Repo) RemoteGit(stdout, stderr io.Writer, args ...string) int {
	cmd := exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false",
		"-c", "core.hooksPath=" + r.NoHooksDir(), "-c", "core.fsmonitor=false"}, args...)...)
	// git's messages in English, as callers read them (a refused push, an origin with no main yet): the
	// user's language would translate them. LC_ALL and LANGUAGE would override LC_MESSAGES.
	env := []string{"LC_MESSAGES=C"}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if k != "JEEVES_AGE_KEY" && k != "LC_ALL" && k != "LANGUAGE" && k != "LC_MESSAGES" {
			env = append(env, kv)
		}
	}
	if os.Getenv("GIT_SSH_COMMAND") == "" && os.Getenv("GIT_SSH") == "" {
		if out, _ := exec.Command("git", "-C", r.Dir, "config", "core.sshCommand").Output(); len(bytes.TrimSpace(out)) == 0 {
			env = append(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=5")
		}
	}
	cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	secs, err := strconv.Atoi(os.Getenv("JEEVES_REMOTE_DEADLINE"))
	if err != nil || secs <= 0 {
		secs = 30
	}
	return RunDeadline(secs, cmd)
}

// RevParse is `git rev-parse -q --verify <rev>`: the object ID, or "" when there is none.
func (r *Repo) RevParse(rev string) string {
	out, err := r.gitCmd("rev-parse", "-q", "--verify", rev).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Out runs git in the data repo and returns its stdout without the trailing line break; stderr is dropped.
func (r *Repo) Out(args ...string) (string, error) {
	out, err := r.gitCmd(args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

// CleanupMark starts the first line jeeves-cleanup gives the parentless commit that replaces the removed
// history; CleanupBundleName is where the removed history is kept on this device (never pushed) until the
// next cleanup replaces it.
const (
	CleanupMark       = "jeeves-cleanup:"
	CleanupBundleName = "history-before-cleanup.bundle"
)

// CleanedRoot: the history of the given commit starts at a jeeves-cleanup root.
func (r *Repo) CleanedRoot(rev string) bool {
	roots, _ := r.Out("rev-list", "--max-parents=0", rev)
	for _, root := range strings.Fields(roots) {
		if subj, _ := r.Out("log", "-1", "--format=%s", root); strings.HasPrefix(subj, CleanupMark) {
			return true
		}
	}
	return false
}

// ShrinkRepo drops the objects no branch reaches any more (removed history, kept in the bundle), so .git
// shrinks; the sync record's bases are kept first.
func (r *Repo) ShrinkRepo() {
	r.KeepBases()
	if r.gitCmd("reflog", "expire", "--expire=now", "--all").Run() == nil {
		r.gitCmd("gc", "-q", "--prune=now").Run()
	}
}

// PlainHistoryMsg is said while origin's history still holds the plain files from before encryption.
const PlainHistoryMsg = "the data repo's history on GitHub still holds the plain files from before encryption: run /jeeves:encryption to remove it"

// PlainHistoryAt: the given encrypted commit's history still holds home/ or hosts/ files from before
// encryption (the fold after turning it on did not reach origin).
func (r *Repo) PlainHistoryAt(rev string) bool {
	first, _ := r.Out("rev-list", "--reverse", rev, "--", "recipient")
	first = FirstLine(first)
	if first == "" || r.RevParse(first+"^") == "" {
		return false
	}
	h, _ := r.Out("log", "-1", "--format=%H", first+"^", "--", "home", "hosts")
	return h != ""
}

// Branch is the current branch, "" when HEAD is detached.
func (r *Repo) Branch() string {
	b, _ := r.Out("symbolic-ref", "--short", "-q", "HEAD")
	return b
}

// Merging: a merge is in progress.
func (r *Repo) Merging() bool {
	fi, err := os.Stat(r.Dir + "/.git/MERGE_HEAD")
	return err == nil && fi.Mode().IsRegular()
}

// HostBackups are the hosts with a backup folder under hosts/, in byte order (names starting with a dot,
// such as the shared review record, are not hosts).
func HostBackups(r *Repo) []string {
	ents, _ := os.ReadDir(r.Dir + "/hosts")
	var hs []string
	for _, d := range ents {
		if strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if fi, err := os.Stat(r.Dir + "/hosts/" + d.Name()); err == nil && fi.IsDir() {
			hs = append(hs, d.Name())
		}
	}
	sort.Strings(hs)
	return hs
}

// Raw runs git in the data repo and returns its stdout as it is; stderr is dropped.
func (r *Repo) Raw(args ...string) ([]byte, error) { return r.gitCmd(args...).Output() }
