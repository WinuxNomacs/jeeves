package auto

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/proc"
)

// ---- The hook's inputs, checked ----

var (
	sessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	plainStem = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func hasControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0
}

// hookPath is a path from the hook's JSON with backslashes made slashes (a Windows path, C:\...).
func hookPath(p string) string { return strings.ReplaceAll(p, `\`, "/") }

func validSession(id string) bool { return sessionID.MatchString(id) }

// physical is a folder's absolute path with its links resolved, "" when it is not a folder.
func physical(p string) string {
	a, err := filepath.Abs(lib.NativePath(p))
	if err != nil {
		return ""
	}
	r, err := filepath.EvalSymlinks(a)
	if err != nil {
		return ""
	}
	if fi, err := os.Stat(r); err != nil || !fi.IsDir() {
		return ""
	}
	return filepath.ToSlash(r)
}

// validTranscript: a plain .jsonl name in a folder under ~/.claude/projects/ (physical paths compared, no
// ".."), a readable file.
func (s *state) validTranscript(t string) bool {
	if t == "" || hasControl(t) || strings.Contains(t, "/../") || strings.HasPrefix(t, "../") || strings.HasSuffix(t, "/..") {
		return false
	}
	base := t[strings.LastIndex(t, "/")+1:]
	if !strings.HasSuffix(base, ".jsonl") || !plainStem.MatchString(strings.TrimSuffix(base, ".jsonl")) {
		return false
	}
	dir := t
	if i := strings.LastIndex(t, "/"); i >= 0 {
		dir = t[:i]
	}
	d, projects := physical(dir), physical(s.e.ClaudeHome+"/projects")
	if d == "" || projects == "" || !strings.HasPrefix(d+"/", projects+"/") {
		return false
	}
	f, err := os.Open(lib.NativePath(t))
	if err != nil {
		return false
	}
	fi, err := f.Stat()
	f.Close()
	return err == nil && fi.Mode().IsRegular()
}

func validFolder(p string) bool {
	if p == "" || hasControl(p) {
		return false
	}
	fi, err := os.Stat(lib.NativePath(p))
	return err == nil && fi.IsDir()
}

// ---- Hooks ----

// hookStop is the Stop hook: when a run is due for this session (on, not backing off, the interval passed,
// no run at work, the transcript grown by min_new_chars of extract), it starts one detached and ends at once.
func (s *state) hookStop(stdin io.Reader) int {
	if os.Getenv("JEEVES_AUTO_CHILD") == "1" || !s.enabled() {
		return 0
	}
	input, _ := io.ReadAll(io.LimitReader(stdin, 1<<20))
	sid := jsonField(input, "session_id")
	tp := hookPath(jsonField(input, "transcript_path"))
	folder := hookPath(jsonField(input, "cwd"))
	if !validSession(sid) || !s.validTranscript(tp) || !validFolder(folder) || s.backoffActive() {
		return 0
	}
	offset, lastRun := s.readSession(sid)
	if now()-lastRun < s.interval || exists(s.lock) && !s.lockStale() {
		return 0
	}
	fi, err := os.Stat(lib.NativePath(tp))
	if err != nil {
		return 0
	}
	if offset > fi.Size() {
		offset = 0
	}
	if fi.Size()-offset < int64(s.minNew) {
		return 0
	}
	if _, n, err := extract(lib.NativePath(tp), offset, "-", s.minNew, true, io.Discard); err != nil || n < s.minNew {
		return 0
	}
	// Detached, through the wrapper (so the run's lock PID is one the bash scripts can check): it outlives
	// this hook and the session; its output goes nowhere.
	c := lib.Command(s.e.Root()+"/bin/jeeves-auto", "run", sid, tp, folder)
	c.Env = scrubbed(os.Environ(), "supervisor")
	c.Stdin, c.Stdout, c.Stderr = nil, nil, nil
	// Not in the session's folder (the user's project): a run lives for minutes, and on Windows a process's
	// folder cannot be removed or renamed meanwhile.
	c.Dir = os.TempDir()
	proc.StartDetached(c)
	if c.Process != nil {
		c.Process.Release()
	}
	return 0
}

// recoverIdle puts right a step a killed run left half done when no run is at work: otherwise, with
// background updates off or backing off, every snapshot would skip the dirty data repo until the next run.
// Briefly: a session start must not wait.
func (s *state) recoverIdle() {
	if !exists(s.dir+"/in-progress") || !s.takeRunLock("auto") {
		return
	}
	defer s.releaseRunLock()
	a, _ := lib.ParseDataRepoArg(nil)
	r, err := lib.FindDataRepo(s.e, a, io.Discard)
	if err != nil {
		return
	}
	r.Stderr = io.Discard
	if l := lib.TakeLock(r.CryptDir, 5); l != nil {
		s.recoverSteps(r)
		l.Release()
	}
}

// sessionStart is the SessionStart hook: leftovers cleaned up, the one-line offer while background updates
// are undecided, each notice once, and what waits for the user's next /jeeves:update.
func (s *state) sessionStart() int {
	if os.Getenv("JEEVES_AUTO_CHILD") == "1" {
		return 0
	}
	if out := s.reapLeftovers(false); len(out) > 0 {
		s.logRun("cleaned", "("+strings.Join(out, " ")+")")
	}
	s.recoverIdle()
	if !exists(s.conf) {
		fmt.Fprintln(s.stdout, "jeeves: background updates (a /jeeves:update review about once an hour of use, in a separate headless session; off by default) are not decided on this device. Tell the user once, in one line, that /jeeves:autoupdate turns them on or keeps them off; either answer ends this reminder.")
	}
	for _, n := range sortedNames(s.dir + "/notices") {
		p := s.dir + "/notices/" + n
		if b, err := os.ReadFile(p); err == nil {
			s.stdout.Write(b)
		}
		os.Remove(p)
	}
	if n := s.queueCount() + s.setAsideCount(); n > 0 {
		fmt.Fprintf(s.stdout, "jeeves: %d item(s) from background updates wait for the user's next /jeeves:update (project rules, changes set aside, or decisions only the user makes).\n", n)
	}
	return 0
}

// ---- The session's variables never reach a run ----

// childKeeps are the variables the child (Claude Code itself) keeps: what selects its config, sign-in and
// provider, none of the session's (its messaging token and socket, plugin variables, the age key).
var childKeeps = map[string]bool{"CLAUDE_CONFIG_DIR": true, "CLAUDE_CODE_OAUTH_TOKEN": true,
	"CLAUDE_CODE_USE_BEDROCK": true, "CLAUDE_CODE_USE_VERTEX": true, "CLAUDE_CODE_USE_FOUNDRY": true,
	"CLAUDE_CODE_SKIP_BEDROCK_AUTH": true, "CLAUDE_CODE_SKIP_VERTEX_AUTH": true, "CLAUDE_CODE_SKIP_FOUNDRY_AUTH": true,
	"CLAUDE_CODE_GIT_BASH_PATH": true}

// scrubbed is env without the session's CLAUDE* and JEEVES* variables: the supervisor keeps what finds the
// data repo (CLAUDE_HOME, the plugin's data_repo option, JEEVES_* but the age key) and the child's; the
// child keeps only childKeeps. JEEVES_LOCK_PID never passes on: each process's own launcher sets it.
func scrubbed(env []string, who string) []string {
	var out []string
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(k, "CLAUDE") && !strings.HasPrefix(k, "JEEVES") {
			out = append(out, kv)
			continue
		}
		keep := childKeeps[k]
		if who == "supervisor" {
			switch {
			case k == "JEEVES_AGE_KEY" || k == "JEEVES_LOCK_PID":
				keep = false
			case k == "CLAUDE_HOME" || k == "CLAUDE_PLUGIN_OPTION_DATA_REPO" || strings.HasPrefix(k, "JEEVES_"):
				keep = true
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}
