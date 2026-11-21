// Package auto is jeeves-auto, background updates: every so often, a separate headless Claude Code session
// reviews what the user's session taught and writes it to the global rules, as /jeeves:update does when the
// user runs it. Off by default, per device; /jeeves:autoupdate turns it on or off.
//
//	jeeves-auto status            the setting, the last runs, what waits for the user
//	jeeves-auto on | off          turn background updates on or off on this device (either answers the offer)
//	jeeves-auto --hook-stop       Stop hook: reads the hook's JSON; when a run is due, starts one detached
//	jeeves-auto --session-start   SessionStart hook: the one-line offer, and notices from background runs
//	jeeves-auto run <session id> <transcript> <folder>
//	                              one background run (started by the Stop hook; runs in the foreground)
//	jeeves-auto hold | release    /jeeves:update, run by the user, holds the run lock while it works
//	jeeves-auto queue             items background runs left for the user's next /jeeves:update
//	jeeves-auto queue clear <id>...
//
// How a run keeps the user's data safe (the plan in upcoming-changes.md, "Background auto-update"):
//   - The model sees only a private run folder: an extract of the session since the last run (no tool
//     results; each message between markers with a random per-run nonce, so quoted text cannot pose as the
//     user), a copy of the global CLAUDE.md to edit, the project's CLAUDE.md to read, and up to 25 memory
//     files. It runs with --permission-mode dontAsk, may edit that one copy and write its notes, and has no
//     shell, web or MCP tools.
//   - This program, not the model, changes the data repo: it checks the edited copy (only additions and
//     revisions: every Security line, every heading and every rule kept, few lines changed, nothing the risk
//     check flags), merges it into main under the data repo's lock, runs the secrets scan and the risk check,
//     and pushes the commit it checked. A result that fails a check is set aside with a notice, never pushed.
//   - One run at a time per device (a run lock with PID and start time); a second due run skips. A deadline
//     stops a hung child, with everything it started. Logs hold figures only, never session text.
//   - The child gets none of the session's variables (messaging token, socket, plugin variables, the age key).
package auto

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// state is one invocation's view of this device's background-update files.
type state struct {
	e        lib.Env
	dir      string // ~/.claude/jeeves/auto
	conf     string // ~/.claude/jeeves/auto.conf
	lock     string // dir/run.lock
	interval int64  // seconds between runs of one session
	minNew   int    // characters a session must have grown by
	deadline int64  // seconds a run's child may take
	maxChars int    // the extract's size limit, in characters
	stdout   io.Writer
	stderr   io.Writer
	session  string // the run's session id, cut to 8 characters, for the log
}

func newState(e lib.Env, stdout, stderr io.Writer) *state {
	s := &state{e: e, dir: e.ClaudeHome + "/jeeves/auto", conf: e.ClaudeHome + "/jeeves/auto.conf",
		stdout: stdout, stderr: stderr}
	s.lock = s.dir + "/run.lock"
	s.interval = int64(s.numConf("interval_minutes", 60)) * 60
	s.minNew = s.numConf("min_new_chars", 8000)
	s.deadline = int64(s.numConf("deadline_minutes", 15)) * 60
	if v, err := strconv.ParseInt(os.Getenv("JEEVES_AUTO_DEADLINE_SECONDS"), 10, 64); err == nil {
		s.deadline = v
	}
	s.maxChars = s.numConf("max_extract_chars", 200000)
	return s
}

func (s *state) say(f string, a ...any) { fmt.Fprintf(s.stdout, "jeeves auto: "+f+"\n", a...) }

func now() int64 { return time.Now().Unix() }

// confValue is a setting from auto.conf (this device's): the last `key = value` line, value trimmed, or def.
func (s *state) confValue(key, def string) string {
	b, err := os.ReadFile(s.conf)
	v := ""
	if err == nil {
		for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
			t := strings.TrimLeft(l, " \t\v\f")
			if !strings.HasPrefix(t, key) {
				continue
			}
			t = strings.TrimLeft(t[len(key):], " \t\v\f")
			if !strings.HasPrefix(t, "=") {
				continue
			}
			v = strings.TrimRight(strings.TrimLeft(t[1:], " \t\v\f"), " \t\v\f")
		}
	}
	if v == "" {
		return def
	}
	return v
}

func (s *state) enabled() bool { return s.confValue("enabled", "no") == "yes" }

// numConf is a whole number from auto.conf, or def.
func (s *state) numConf(key string, def int) int {
	v := s.confValue(key, strconv.Itoa(def))
	for _, c := range v {
		if c < '0' || c > '9' {
			return def
		}
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// ---- Notices (printed once by the session-start hook) and the log ----

// notice writes a notice, replacing one of the same name.
func (s *state) notice(name, text string) {
	if os.MkdirAll(s.dir+"/notices", 0o777) == nil {
		os.WriteFile(s.dir+"/notices/"+name, []byte(text+"\n"), 0o666)
	}
}

// logRun appends one line per run, keeping the last 50; never session text.
func (s *state) logRun(outcome, figures string) {
	if os.MkdirAll(s.dir, 0o777) != nil {
		return
	}
	f := s.dir + "/log"
	sess := s.session
	if sess == "" {
		sess = "?"
	}
	line := fmt.Sprintf("%s %s %s %s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"), sess, outcome, figures)
	if fh, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o666); err == nil {
		fh.WriteString(line)
		fh.Close()
	}
	b, err := os.ReadFile(f)
	if err != nil {
		return
	}
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 50 {
		if os.WriteFile(f+".tmp", []byte(strings.Join(lines[len(lines)-50:], "")), 0o666) == nil {
			os.Rename(f+".tmp", f)
		}
	}
}

// ---- Per-session records, the backoff, what waits for the user ----

func (s *state) sessionFile(id string) string { return s.dir + "/sessions/" + id }

// readSession gives a session's offset reached and last run time (0 when unknown).
func (s *state) readSession(id string) (offset, lastRun int64) {
	f := strings.Fields(firstLine(s.sessionFile(id)))
	if len(f) > 0 {
		offset = digits(f[0])
	}
	if len(f) > 1 {
		lastRun = digits(f[1])
	}
	return
}

func (s *state) writeSession(id string, offset, lastRun int64) {
	if os.MkdirAll(s.dir+"/sessions", 0o777) != nil {
		return
	}
	f := s.sessionFile(id)
	if os.WriteFile(f+".tmp", []byte(fmt.Sprintf("%d %d\n", offset, lastRun)), 0o666) == nil {
		os.Rename(f+".tmp", f)
	}
}

func (s *state) backoffUntil() int64 { return digits(strings.TrimSpace(firstLine(s.dir + "/backoff"))) }
func (s *state) backoffActive() bool { return now() < s.backoffUntil() }
func (s *state) backoff(secs int64) {
	os.WriteFile(s.dir+"/backoff", []byte(strconv.FormatInt(now()+secs, 10)+"\n"), 0o666)
}

// count gives how many files in dir end with suffix.
func count(dir, suffix string) int {
	ents, _ := os.ReadDir(dir)
	n := 0
	for _, d := range ents {
		if strings.HasSuffix(d.Name(), suffix) {
			n++
		}
	}
	return n
}

func (s *state) queueCount() int    { return count(s.dir+"/queue", ".md") }
func (s *state) setAsideCount() int { return count(s.dir+"/set-aside", ".patch") }

// isNum: one or more digits and nothing else.
func isNum(v string) bool {
	if v == "" {
		return false
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// digits is a whole number made only of digits, else 0.
func digits(v string) int64 {
	if v == "" {
		return 0
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return 0
		}
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// firstLine is a file's first line without its line break, "" when it cannot be read.
func firstLine(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	l, _, _ := strings.Cut(string(b), "\n")
	return l
}

// Run is jeeves-auto.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	s := newState(e, stdout, stderr)
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "--hook-stop":
		return s.hookStop(os.Stdin)
	case "--session-start":
		return s.sessionStart()
	case "--watch":
		return watch(args[1:])
	case "run":
		if len(args) != 4 {
			fmt.Fprintln(stderr, "usage: jeeves-auto run <session id> <transcript> <folder>")
			return 2
		}
		return s.run(args[1], args[2], args[3])
	case "on":
		if lib.LookPath("claude") == "" {
			s.say("the claude command is not on the PATH; left off")
			return 1
		}
		if !s.setEnabled("yes") {
			return 1
		}
		s.say("on: a background review at most every %d minutes per session, once it has grown by %d characters", s.interval/60, s.minNew)
	case "off":
		if !s.setEnabled("no") {
			return 1
		}
		s.say("off")
	case "status":
		s.status()
	case "hold":
		return s.hold()
	case "release":
		s.release()
	case "queue":
		s.queue(args[1:])
	default:
		fmt.Fprintln(stderr, "usage: jeeves-auto status | on | off | queue [clear <id>...] | hold | release")
		return 2
	}
	return 0
}

// setEnabled writes enabled=<v> into auto.conf, keeping its other settings.
func (s *state) setEnabled(v string) bool {
	if os.MkdirAll(s.e.ClaudeHome+"/jeeves", 0o777) != nil {
		return false
	}
	var keep []string
	if b, err := os.ReadFile(s.conf); err == nil {
		for _, l := range strings.SplitAfter(string(b), "\n") {
			if l == "" {
				continue
			}
			t := strings.TrimLeft(l, " \t\v\f")
			if strings.HasPrefix(t, "enabled") && strings.HasPrefix(strings.TrimLeft(t[len("enabled"):], " \t\v\f"), "=") {
				continue
			}
			if !strings.HasSuffix(l, "\n") {
				l += "\n"
			}
			keep = append(keep, l)
		}
	}
	keep = append(keep, "enabled="+v+"\n")
	if os.WriteFile(s.conf+".tmp", []byte(strings.Join(keep, "")), 0o666) != nil {
		return false
	}
	return os.Rename(s.conf+".tmp", s.conf) == nil
}

func (s *state) status() {
	switch {
	case !exists(s.conf):
		s.say("off (not decided on this device)")
	case s.enabled():
		s.say("on")
	default:
		s.say("off")
	}
	s.say("at most every %d minutes per session, after %d new characters; deadline %d minutes", s.interval/60, s.minNew, s.deadline/60)
	if exists(s.lock) && !s.lockStale() {
		f := strings.Fields(firstLine(s.lock))
		start := int64(0)
		if len(f) > 1 {
			start = digits(f[1])
		}
		child := ""
		if exists(s.dir + "/child") {
			if c := strings.Fields(firstLine(s.dir + "/child")); len(c) > 0 {
				child = ", child PID " + c[0]
			} else {
				child = ", child PID "
			}
		}
		s.say("at work now (%s, %ds)%s", s.lockOwner(), now()-start, child)
	} else if out := s.reapLeftovers(false); len(out) > 0 {
		s.say("%s", strings.Join(out, "\n"))
	}
	if s.backoffActive() {
		s.say("paused after a failure for %d more minutes", (s.backoffUntil()-now())/60+1)
	}
	s.say("waiting for the user: %d queued item(s), %d set-aside change(s)", s.queueCount(), s.setAsideCount())
	if b, err := os.ReadFile(s.dir + "/log"); err == nil {
		s.say("last runs:")
		lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
		if len(lines) > 5 {
			lines = lines[len(lines)-5:]
		}
		for _, l := range lines {
			fmt.Fprintf(s.stdout, "  %s\n", l)
		}
	}
}

// hold takes the run lock for the user's /jeeves:update, waiting for a background run to finish (up to
// JEEVES_AUTO_HOLD_WAIT seconds, 100: under the Bash tool's two-minute limit). Another hold of the user's
// counts one more, so its release does not free this one.
func (s *state) hold() int {
	wait := 100
	if v, err := strconv.Atoi(os.Getenv("JEEVES_AUTO_HOLD_WAIT")); err == nil {
		wait = v
	}
	for i := 0; !s.takeRunLock("manual"); i++ {
		if s.lockOwner() == "manual" {
			f := strings.Fields(firstLine(s.lock))
			c := int64(1)
			if len(f) > 3 && digits(f[3]) > 0 {
				c = digits(f[3])
			}
			os.WriteFile(s.lock, []byte(fmt.Sprintf("%s %d manual %d\n", lib.LockPID(), now(), c+1)), 0o666)
			break
		}
		if i >= wait*10 {
			s.say("a background update is still at work; try again in a few minutes")
			return 1
		}
		if i == 0 {
			s.say("a background update is at work; waiting for it to finish (up to %d seconds)", wait)
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.say("held")
	return 0
}

func (s *state) release() {
	if s.lockOwner() == "manual" {
		f := strings.Fields(firstLine(s.lock))
		c := int64(1)
		if len(f) > 3 && digits(f[3]) > 0 {
			c = digits(f[3])
		}
		if c > 1 && len(f) >= 2 {
			os.WriteFile(s.lock, []byte(fmt.Sprintf("%s %s manual %d\n", f[0], f[1], c-1)), 0o666)
		} else {
			os.Remove(s.lock)
		}
	}
	s.say("released")
}

func (s *state) queue(args []string) {
	if len(args) > 0 && args[0] == "clear" {
		for _, id := range args[1:] {
			if id == "" || strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") {
				s.say("not a queue item: %s", id)
				continue
			}
			os.Remove(s.dir + "/queue/" + strings.TrimSuffix(id, ".md") + ".md")
			if strings.HasSuffix(id, ".patch") {
				os.Remove(s.dir + "/set-aside/" + id)
			}
		}
		s.say("cleared")
		return
	}
	show := func(dir, suffix, note string) {
		for _, n := range sortedNames(dir) {
			if !strings.HasSuffix(n, suffix) {
				continue
			}
			b, err := os.ReadFile(dir + "/" + n)
			if err != nil {
				continue
			}
			fmt.Fprintf(s.stdout, "=== %s%s\n", n, note)
			s.stdout.Write(b)
			fmt.Fprintln(s.stdout)
		}
	}
	show(s.dir+"/queue", ".md", "")
	show(s.dir+"/set-aside", ".patch", " (set aside)")
}

// sortedNames lists the regular files in dir in byte order (a shell glob's order in the C locale).
func sortedNames(dir string) []string {
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, d := range ents {
		if fi, err := os.Stat(dir + "/" + d.Name()); err == nil && fi.Mode().IsRegular() {
			out = append(out, d.Name())
		}
	}
	return out // os.ReadDir sorts by name
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
