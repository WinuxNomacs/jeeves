package auto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/WinuxNomacs/jeeves/internal/apply"
	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/memories"
	"github.com/WinuxNomacs/jeeves/internal/proc"
	"github.com/WinuxNomacs/jeeves/internal/risk"
	"github.com/WinuxNomacs/jeeves/internal/scan"
	"github.com/WinuxNomacs/jeeves/internal/snapshot"
	"github.com/WinuxNomacs/jeeves/internal/worker"
)

var (
	// Text that looks like a secret: a summary or queued item holding it is withheld.
	summarySecret = regexp.MustCompile(`sk-ant-|gh[pousr]_|github_pat_|A(KIA|SIA)[0-9A-Z]{16}|xox[abprs]-|PRIVATE KEY|[A-Za-z0-9_./+=-]{40,}`)
	queueSecret   = regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{30,}|github_pat_|A(KIA|SIA)[0-9A-Z]{16}|xox[abprs]-|PRIVATE KEY`)
	usageLimit    = regexp.MustCompile(`(?i)usage limit|rate limit|limit reached|quota|credit balance`)
	settingsModel = regexp.MustCompile(`^[A-Za-z0-9._:\[\]-]{1,100}$`)
	envName       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// errExit ends a run early, after its cleanup.
type errExit struct{}

// a run's moving parts, for its cleanup however it ends (a signal's cleanup runs on another goroutine, so
// the parts it stops change under mu).
type runner struct {
	*state
	mu      sync.Mutex
	r       *lib.Repo
	run     string // the run folder (native path)
	child   *exec.Cmd
	box     *proc.Contained
	done    *proc.Waited
	watch   *exec.Cmd
	dataLk  *lib.Lock
	outcome string
}

func (u *runner) cleanup() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.child != nil && u.box != nil {
		u.box.Stop(u.done, 10*time.Second)
		u.box.Close()
		u.child, u.box = nil, nil
	}
	u.stopWatch()
	if u.run != "" {
		os.RemoveAll(u.run)
	}
	os.Remove(u.dir + "/run-dir")
	os.Remove(u.dir + "/child")
	if u.dataLk != nil {
		u.dataLk.Release()
		u.dataLk = nil
	}
	u.releaseRunLock()
}

func (u *runner) stopWatch() {
	if u.watch != nil && u.watch.Process != nil {
		u.watch.Process.Kill()
		u.watch.Wait()
	}
	u.watch = nil
}

// with runs f under mu.
func (u *runner) with(f func()) { u.mu.Lock(); defer u.mu.Unlock(); f() }

func (u *runner) takeData() bool {
	l := lib.TakeLock(u.r.CryptDir, 30)
	u.with(func() { u.dataLk = l })
	return l != nil
}

func (u *runner) releaseData() {
	u.with(func() {
		if u.dataLk != nil {
			u.dataLk.Release()
			u.dataLk = nil
		}
	})
}

// git runs git in the data repo, output dropped; true on success.
func (u *runner) git(args ...string) bool { _, err := u.r.Git(args...); return err == nil }

func (u *runner) out(args ...string) string { o, _ := u.r.Out(args...); return o }

// cleanMain: the data repo is on main, no merge under way, nothing changed under home/.
func (u *runner) cleanMain() bool {
	st, _ := u.r.Git("status", "--porcelain", "--", "home")
	return u.r.Branch() == "main" && !u.r.Merging() && st == ""
}

// queued runs one of the commands the machine's worker runs (snapshot, apply, memories record) as any
// caller does, output captured.
func (u *runner) queued(cmd string, args []string, direct func(lib.Env, []string, io.Writer, io.Writer) int) (string, int) {
	var b bytes.Buffer
	u.refreshRunLock()
	rc := worker.Run(u.e, cmd, args, &b, &b, direct)
	return b.String(), rc
}

// applyEnv makes this process's environment env (the session's variables gone), keeping the lock PID its
// launcher set.
func applyEnv(env []string) {
	keep := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		keep[k] = v
	}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := keep[k]; !ok && k != "JEEVES_LOCK_PID" {
			os.Unsetenv(k)
		}
	}
}

func (s *state) run(sid, tp, folder string) (rc int) {
	s.session = firstRunes(sid, 8)
	umask()
	if !validSession(sid) || !s.validTranscript(tp) || !validFolder(folder) {
		s.logRun("skipped", "(input refused)")
		return 0
	}
	if !s.enabled() || !s.takeRunLock("auto") { // another run, or the user's /jeeves:update, is at work
		return 0
	}
	u := &runner{state: s}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, os.Interrupt, syscall.SIGHUP)
	stopped := make(chan struct{})
	go func() {
		select {
		case <-sigs:
			u.cleanup()
			os.Exit(143)
		case <-stopped:
		}
	}()
	defer func() {
		close(stopped)
		signal.Stop(sigs)
		if p := recover(); p != nil {
			if _, ok := p.(errExit); !ok {
				// A bug: recorded where the user and the next run see it (this process's output goes
				// nowhere), backed off like any failure, then the panic goes on.
				msg := firstRunes(strings.ReplaceAll(fmt.Sprint(p), "\n", " "), 120)
				u.logRun("failed", "(internal error: "+msg+")")
				u.notice("problem-run", "jeeves: a background update stopped on an internal error (see jeeves-auto status); it tries again later.")
				u.backoff(3600)
				u.cleanup()
				panic(p)
			}
		}
		u.cleanup()
	}()
	applyEnv(scrubbed(os.Environ(), "supervisor"))
	u.body(sid, tp, folder)
	return 0
}

// stop ends the run here (its cleanup runs).
func stop() { panic(errExit{}) }

func (u *runner) body(sid, tp, folder string) {
	// A run killed earlier left its child or folder: put right now.
	if out := u.reapLeftovers(true); len(out) > 0 {
		u.logRun("cleaned", "("+strings.Join(out, " ")+")")
	}
	if lib.LookPath("claude") == "" {
		u.notice("problem-claude", "jeeves: background updates could not start: the claude command is not on the PATH.")
		u.logRun("skipped", "(no claude)")
		u.backoff(3600)
		stop()
	}
	a, _ := lib.ParseDataRepoArg(nil)
	r, err := lib.FindDataRepo(u.e, a, io.Discard)
	if err != nil {
		u.notice("problem-repo", "jeeves: a background update was skipped: "+err.Error()+".")
		u.logRun("skipped", "(no data repo)")
		u.backoff(3600)
		stop()
	}
	r.Stderr = io.Discard
	u.r = r
	if u.takeData() {
		u.recoverSteps(r)
		u.releaseData()
	}

	// Sync first: what other devices pushed, and what changed on disk.
	u.queued("snapshot", nil, snapshot.Run)
	// Whether what main would send passes the secrets scan before this run commits anything: a later
	// failure is then this run's own commit.
	prescan := scan.Run(u.e, nil, io.Discard, io.Discard) == 0
	if !u.cleanMain() {
		u.notice("problem-repo", "jeeves: background updates are waiting: the data repo is not on a clean main (run /jeeves:update).")
		u.logRun("skipped", "(data repo not on a clean main)")
		u.backoff(3600)
		stop()
	}

	offset, _ := u.readSession(sid)
	base := u.runBase()
	if base == "" {
		u.logRun("failed", "(no temp folder)")
		stop()
	}
	run, err := makeRunFolder(base)
	if err != nil {
		u.logRun("failed", "(no temp folder)")
		stop()
	}
	u.with(func() { u.run = run })
	runSh := lib.ShellPath(run) // as the records, the brief and the bash version write it
	os.WriteFile(u.dir+"/run-dir", []byte(runSh+"\n"), 0o600)
	for _, d := range []string{"edit", "orig", "out", "memories"} {
		if os.MkdirAll(run+"/"+d, 0o700) != nil {
			stop()
		}
	}
	nb := make([]byte, 16)
	if _, err := rand.Read(nb); err != nil {
		u.logRun("failed", "(no random nonce)")
		stop()
	}
	nonce := hex.EncodeToString(nb)
	ef, err := os.Create(run + "/extract.txt")
	if err != nil {
		u.logRun("skipped", "(transcript unreadable)")
		stop()
	}
	newOffset, _, err := extract(lib.NativePath(tp), offset, nonce, u.maxChars, false, ef)
	ef.Close()
	if err != nil {
		u.logRun("skipped", "(transcript unreadable)")
		stop()
	}
	if fi, err := os.Stat(run + "/extract.txt"); err != nil || fi.Size() == 0 {
		u.writeSession(sid, newOffset, now())
		u.logRun("nothing", "(no new messages)")
		stop()
	}

	rules := r.HomeCopy + "/CLAUDE.md"
	if lib.CopyFile(rules, run+"/orig/CLAUDE.md") != nil || lib.CopyFile(rules, run+"/edit/CLAUDE.md") != nil {
		u.logRun("failed", "(could not copy the rules)")
		stop()
	}
	top, proj := "", ""
	if o, err := exec.Command("git", "-C", lib.NativePath(folder), "rev-parse", "--show-toplevel").Output(); err == nil {
		top = strings.TrimRight(string(o), "\n")
	}
	if top != "" {
		for _, f := range []string{top + "/CLAUDE.md", top + "/.claude/CLAUDE.md"} {
			if fi, err := os.Stat(lib.NativePath(f)); err == nil && fi.Mode().IsRegular() &&
				lib.CopyFile(lib.NativePath(f), run+"/project-CLAUDE.md") == nil {
				proj = f
				break
			}
		}
	}
	u.copyMemories(run)
	projLine := "(none)"
	if proj != "" {
		projLine = runSh + "/project-CLAUDE.md (a copy of " + proj + ")"
	}
	topLine := top
	if topLine == "" {
		topLine = "(none)"
	}
	brief := fmt.Sprintf(`# Background update brief

- nonce: %s
- device: %s
- session extract: %[3]s/extract.txt
- global rules to edit: %[3]s/edit/CLAUDE.md
- project: %[4]s
- project rules (read only): %[5]s
- memories: %[3]s/memories/ (index.tsv: number, hash, source, project)
- write: %[3]s/out/summary.txt, %[3]s/out/queue.md, %[3]s/out/memories.tsv
`, nonce, u.e.Host, runSh, topLine, projLine)
	os.WriteFile(run+"/brief.md", []byte(brief), 0o600)

	use, ms, turns, ok := u.runChild(run)
	if !ok {
		return
	}
	u.outcome = "reviewed"
	u.refreshRunLock()
	summary := readSummary(run + "/out/summary.txt")
	hash := u.commitEdit(run, summary)

	// Memories reviewed: only hashes this run was given are recorded.
	u.recordMemories(run)
	u.queueItems(run, sid, top)

	pushed := u.pushScanned()
	// The scan stopped it and this run's own commit is the tip (never pushed): that commit is taken back
	// out and set aside, and the rest is tried again.
	if pushed == "scan" && hash != "" && prescan && u.takeData() {
		if u.out("rev-parse", "--short", "HEAD") == hash && u.git("reset", "-q", "--keep", "HEAD~1") {
			hash = ""
			u.setAside(run, "a line looked like a secret")
			u.releaseData()
			pushed = u.pushScanned()
		} else {
			u.releaseData()
		}
	}
	switch pushed {
	case "pushed":
		u.outcome += ",pushed"
	case "scan":
		u.outcome += ",not-pushed(scan)"
		u.notice("problem-push", "jeeves: a background update found what may be a secret in the outgoing commits and pushed nothing: run /jeeves:update.")
	case "risk":
		u.outcome += ",not-pushed(risk)"
		u.notice("problem-push", "jeeves: flagged lines in the outgoing commits wait for the user's OK, so a background update pushed nothing: run /jeeves:update.")
	case "local":
		u.outcome += ",not-pushed"
		u.notice("problem-push", "jeeves: a background update could not push (offline, or GitHub refused); its commits are kept, and the next run or /jeeves:update pushes them.")
	}
	out, _ := u.queued("apply", nil, apply.Run)
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "jeeves apply: held ") {
			u.notice("problem-held", "jeeves: skills, agents or commands from another device wait for the user's OK: run /jeeves:update.")
			break
		}
	}
	if hash != "" {
		first := strings.Split(strings.TrimRight(summary, "\n"), "\n")
		if len(first) > 3 {
			first = first[:3]
		}
		u.notice("change-"+hash, "jeeves: a background update changed the global rules: "+strings.Join(first, " ")+". Undo: /jeeves:update revert "+hash)
	}
	u.writeSession(sid, newOffset, now())
	os.Remove(u.dir + "/backoff")
	u.logRun(u.outcome, fmt.Sprintf("%s ms=%s turns=%s", use, ms, turns))
}

// runBase is where run folders go. Claude Code reads every CLAUDE.md on the way up from the child's folder,
// so no folder on that path may be one another user can write: XDG_RUNTIME_DIR when it is this user's and
// private, else a per-user TMPDIR (macOS, Windows), else ~/.cache/jeeves (never under ~/.claude, where
// Claude Code refuses edits even with an allow rule). Never the shared /tmp.
func (s *state) runBase() string {
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		if fi, err := os.Stat(lib.NativePath(x)); err == nil && fi.IsDir() && ownedBy(fi) && private(fi) {
			return lib.NativePath(x)
		}
	}
	d := strings.TrimSuffix(os.Getenv("TMPDIR"), "/")
	switch d {
	case "", "/tmp", "/var/tmp", "/private/tmp":
	default:
		if fi, err := os.Stat(lib.NativePath(d)); err == nil && fi.IsDir() && ownedBy(fi) {
			return lib.NativePath(d)
		}
	}
	c := os.Getenv("XDG_CACHE_HOME")
	if c == "" {
		c = s.e.Home + "/.cache"
	}
	c = lib.NativePath(c) + "/jeeves"
	if os.MkdirAll(c, 0o700) != nil || os.Chmod(c, 0o700) != nil {
		return ""
	}
	return c
}

// makeRunFolder makes base/jeeves-auto.XXXXXX (six random letters and digits, as mktemp), private, and
// returns its physical path.
func makeRunFolder(base string) (string, error) {
	const set = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for try := 0; try < 100; try++ {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for i := range b {
			b[i] = set[int(b[i])%len(set)]
		}
		p := filepath.Join(base, "jeeves-auto."+string(b))
		if err := os.Mkdir(p, 0o700); err == nil {
			r, err := filepath.EvalSymlinks(p)
			if err != nil {
				os.Remove(p)
				return "", err
			}
			return r, nil
		} else if !os.IsExist(err) {
			return "", err
		}
	}
	return "", os.ErrExist
}

// copyMemories copies up to 25 of the memories not yet reviewed into the run folder, with their index.
func (u *runner) copyMemories(run string) {
	var b bytes.Buffer
	memories.Run(u.e, []string{"list"}, &b, io.Discard)
	var idx strings.Builder
	i := 0
	for _, l := range strings.Split(b.String(), "\n") {
		if i == 25 {
			break
		}
		f := strings.SplitN(l, "\t", 4)
		if len(f) < 4 {
			continue
		}
		i++
		n := fmt.Sprintf("%02d", i)
		if lib.CopyFile(lib.NativePath(f[3]), run+"/memories/"+n+".md") == nil {
			fmt.Fprintf(&idx, "%s\t%s\t%s\t%s\n", n, f[0], f[1], f[2])
		}
	}
	if idx.Len() > 0 {
		os.WriteFile(run+"/memories/index.tsv", []byte(idx.String()), 0o600)
	}
}

// userSettings reads the user's settings.json for the child: its model (checked) and its env block (minus
// CLAUDE_CODE_PLUGIN_DIRS: the plugin is loaded by path). Nothing else of it: its permission rules, hooks and
// other plugins would widen what the child may do.
func (s *state) userSettings() (model string, env []string) {
	b, err := os.ReadFile(s.e.ClaudeHome + "/settings.json")
	if err != nil {
		return "", nil
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var m map[string]any
	if d.Decode(&m) != nil {
		return "", nil
	}
	if v, ok := m["model"].(string); ok && settingsModel.MatchString(v) {
		model = v
	}
	if e, ok := m["env"].(map[string]any); ok {
		for k, v := range e {
			if !envName.MatchString(k) || k == "CLAUDE_CODE_PLUGIN_DIRS" {
				continue
			}
			val := ""
			if v != nil {
				var ok bool
				if val, ok = scalar(v); !ok {
					continue
				}
			}
			if strings.ContainsRune(val, 0) {
				continue
			}
			env = append(env, k+"="+val)
		}
	}
	return
}

// rulePath is a path as Claude Code's permission rules match it: on Windows it normalizes C:\Users\... to
// /c/Users/... (docs, "permissions"); a rule then puts "/" before it. Unchanged elsewhere.
func rulePath(native string) string {
	p := filepath.ToSlash(native)
	if len(p) >= 2 && p[1] == ':' {
		p = "/" + strings.ToLower(p[:1]) + p[2:]
	}
	return p
}

// runChild runs the child: Claude Code headless, confined to the run folder, with no shell, web or MCP
// tools, to its deadline. The user's settings are not loaded (their permission rules and hooks would widen
// it): this plugin is loaded by path, and only the settings' model and env carried over. Deny rules win over
// a skill's allowed-tools, so the update skill's own grants give it no shell either. ok is false when the
// run ends here (the child failed or hung: logged, backed off).
func (u *runner) runChild(run string) (use, ms, turns string, ok bool) {
	runN, runR := filepath.ToSlash(run), rulePath(run)
	model, senv := u.userSettings()
	args := []string{"-p", "/jeeves:update auto " + runN, "--setting-sources", "project",
		"--plugin-dir", filepath.ToSlash(lib.NativePath(u.e.Root()))}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--permission-mode", "dontAsk", "--strict-mcp-config",
		"--no-session-persistence", "--max-turns", "40", "--output-format", "json",
		"--allowedTools", "Read(/"+runR+"/**)", "Edit(/"+runR+"/edit/**)", "Write(/"+runR+"/out/**)", "Edit(/"+runR+"/out/**)",
		"--disallowedTools", "Bash", "WebFetch", "WebSearch", "Agent", "NotebookEdit")
	c := lib.Command("claude", args...)
	c.Env = append(append(scrubbed(os.Environ(), "child"), senv...), "JEEVES_AUTO_CHILD=1")
	c.Dir = run
	res, err := os.Create(run + "/result.json")
	if err != nil {
		u.logRun("failed", "(no temp folder)")
		stop()
	}
	defer res.Close()
	c.Stdout = res
	box, err := proc.StartContained(c)
	if err != nil {
		u.backoff(3600)
		u.notice("problem-child", "jeeves: a background update failed (Claude Code did not start) and is paused for an hour; jeeves-auto status shows the last runs.")
		u.logRun("failed", "(child did not start)")
		stop()
	}
	u.with(func() { u.child, u.box, u.done = c, box, proc.Wait(c) })
	end := now() + u.deadline
	os.WriteFile(u.dir+"/child", []byte(fmt.Sprintf("%d %d %s\n", c.Process.Pid, now(), lib.ShellPath(run))), 0o600)
	// A watchdog beside the child (Unix): if this supervisor dies without cleaning up, or a minute past the
	// deadline, it stops the child. Without one a killed supervisor would leave the child running
	// unsupervised, so a child it cannot watch does not go on.
	if needsWatchdog {
		exe, err := os.Executable()
		w := exec.Command(exe, "auto", "--watch", strconv.Itoa(c.Process.Pid), strconv.Itoa(os.Getpid()),
			strconv.FormatInt(end, 10), run, u.dir)
		if err == nil {
			err = proc.StartDetached(w)
		}
		if err != nil {
			u.backoff(3600)
			u.logRun("failed", "(no watchdog)")
			stop()
		}
		u.with(func() { u.watch = w })
	}
	// The deadline by the clock.
	var werr error
wait:
	for {
		select {
		case <-u.done.Done():
			werr = u.done.Err()
			break wait
		case <-time.After(500 * time.Millisecond):
			if now() >= end {
				u.with(func() {
					u.box.Stop(u.done, 10*time.Second)
					u.box.Close()
					u.child, u.box = nil, nil
				})
				os.Remove(u.dir + "/child")
				u.backoff(3600)
				u.logRun("failed", fmt.Sprintf("(stopped at the deadline, %ds)", u.deadline))
				stop()
			}
		}
	}
	u.with(func() {
		u.box.Close()
		u.child, u.box = nil, nil
		u.stopWatch()
	})
	os.Remove(u.dir + "/child")
	exit := lib.ExitCode(werr)
	b, _ := os.ReadFile(run + "/result.json")
	isErr := jsonField(b, "is_error")
	use, ms, turns = tokenFigures(b), jsonField(b, "duration_ms"), jsonField(b, "num_turns")
	if exit != 0 || isErr != "false" {
		if usageLimit.MatchString(jsonField(b, "result")) {
			u.backoff(86400)
			u.notice("problem-usage", "jeeves: background updates are paused for a day: the plan's usage limit was reached.")
			u.logRun("failed", "(usage limit; paused a day) "+use)
		} else {
			u.backoff(3600)
			u.notice("problem-child", fmt.Sprintf("jeeves: a background update failed (Claude Code exit %d; signed out, or the plugin not found?) and is paused for an hour; jeeves-auto status shows the last runs.", exit))
			u.logRun("failed", fmt.Sprintf("(child exit %d) %s ms=%s turns=%s", exit, use, ms, turns))
		}
		return "", "", "", false
	}
	return use, ms, turns, true
}

// tokenFigures is the child's token use from its result's "usage" object, for the log: the total, then
// input, cache writes, cache reads and output. Tokens, not the result's dollar figure: on a Claude plan a run
// draws on the plan's usage like any session, and nothing is charged per run.
func tokenFigures(b []byte) string {
	u, _ := decodeObject(bytes.TrimSpace(b))["usage"].(map[string]any)
	var f [4]int64
	for i, k := range []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens"} {
		if n, ok := u[k].(json.Number); ok {
			f[i], _ = n.Int64()
		}
	}
	return fmt.Sprintf("tokens=%d input=%d cache_write=%d cache_read=%d output=%d", f[0]+f[1]+f[2]+f[3], f[0], f[1], f[2], f[3])
}

// readSummary is the run's summary as it may go into a notice and a commit message: carriage returns and
// control characters out, blank lines dropped, 20 lines of up to 200 characters; withheld (replaced) if
// anything in it is flagged, looks like a secret, or holds invisible characters.
func readSummary(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	t := strings.Map(func(r rune) rune {
		if r == '\r' || r < 0x20 && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, string(b))
	var lines []string
	for _, l := range strings.Split(t, "\n") {
		if blank(l) {
			continue
		}
		if len(lines) == 20 {
			break
		}
		lines = append(lines, firstRunes(l, 200))
	}
	if len(lines) == 0 {
		return ""
	}
	summary := strings.Join(lines, "\n")
	var fl []lib.Flag
	for _, l := range lines {
		fl = append(fl, lib.Flag{Where: "summary", Text: l})
	}
	if len(lib.RiskFlags(fl)) > 0 || summarySecret.MatchString(summary) || hidden.MatchString(summary) || !utf8.ValidString(summary) {
		return "revised the global rules (the run's summary was withheld: it held flagged text)"
	}
	return summary
}

// setAside keeps the proposed change for the user, never pushed.
func (u *runner) setAside(run, why string) {
	id := time.Now().Format("20060102-150405")
	if os.MkdirAll(u.dir+"/set-aside", 0o777) == nil {
		d, _ := exec.Command("git", "diff", "--no-index", "--no-color", "--no-ext-diff", "--", run+"/orig/CLAUDE.md", run+"/edit/CLAUDE.md").Output()
		os.WriteFile(u.dir+"/set-aside/"+id+".patch", append([]byte("# set aside: "+why+"\n"), d...), 0o600)
	}
	u.notice("set-aside-"+id, "jeeves: a background update was set aside, not pushed: "+why+". The proposed change is in "+lib.ShellPath(u.dir)+"/set-aside/"+id+".patch; the next /jeeves:update takes it up.")
	u.outcome = "set-aside(" + why + ")"
}

// commitEdit checks the child's edit and, when it passes, merges it into main's home/CLAUDE.md and commits
// it; returns the commit's short hash, or "" (nothing changed, or set aside).
func (u *runner) commitEdit(run, summary string) string {
	orig, edit := run+"/orig/CLAUDE.md", run+"/edit/CLAUDE.md"
	if same(orig, edit) {
		return ""
	}
	if why := checkEdit(orig, edit); why != "" {
		u.setAside(run, why)
		return ""
	}
	if !u.takeData() {
		u.setAside(run, "the data repo stayed busy")
		return ""
	}
	defer u.releaseData()
	if !u.cleanMain() {
		u.setAside(run, "the data repo was not on a clean main")
		return ""
	}
	// main may have moved since the copy (a pull, a disk edit): a three-way merge, refused on a conflict.
	rules, merged := u.r.HomeCopy+"/CLAUDE.md", run+"/merged"
	if same(rules, orig) {
		if lib.CopyFile(edit, merged) != nil {
			u.setAside(run, "the commit failed")
			return ""
		}
	} else if lib.CopyFile(rules, merged) != nil || exec.Command("git", "merge-file", "-q", merged, orig, edit).Run() != nil {
		u.setAside(run, "the global rules changed meanwhile in the same place")
		return ""
	}
	if u.r.Encrypted() {
		u.e.LoadKey(u.r)
	}
	defer os.Unsetenv("JEEVES_AGE_KEY")
	if summary == "" {
		summary = "revised the global rules"
	}
	h, _ := lib.HashOf(merged)
	mark := u.dir + "/in-progress"
	os.WriteFile(mark, []byte("commit "+h+"\n"), 0o600)
	defer os.Remove(mark)
	first := strings.SplitN(summary, "\n", 2)[0]
	if lib.CopyFile(merged, rules) == nil && u.git("add", "--", "home/CLAUDE.md") &&
		u.git("commit", "-q", "-m", "learn: "+first+" (background update on "+u.e.Host+")", "-m", summary, "--", "home/CLAUDE.md") {
		u.outcome = "learned"
		return u.out("rev-parse", "--short", "HEAD")
	}
	u.git("reset", "-q", "--", "home/CLAUDE.md")
	u.git("checkout", "-q", "--", "home/CLAUDE.md")
	u.setAside(run, "the commit failed")
	return ""
}

func same(a, b string) bool {
	x, err1 := os.ReadFile(a)
	y, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// recordMemories records the memories the child reviewed, only hashes this run gave it.
func (u *runner) recordMemories(run string) {
	out, err1 := os.ReadFile(run + "/out/memories.tsv")
	idx, err2 := os.ReadFile(run + "/memories/index.tsv")
	if err1 != nil || err2 != nil || len(out) == 0 || len(idx) == 0 {
		return
	}
	given := map[string]bool{}
	for _, l := range strings.Split(string(idx), "\n") {
		if f := strings.Split(l, "\t"); len(f) >= 2 {
			given[f[1]] = true
		}
	}
	args := []string{"record"}
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		l = strings.Trim(l, "\t")
		h, result, _ := strings.Cut(l, "\t")
		h = strings.NewReplacer(" ", "", "\r", "").Replace(h)
		if !given[h] {
			continue
		}
		result = strings.Trim(result, "\t")
		if result == "" {
			result = "reviewed"
		}
		result = strings.Map(func(r rune) rune {
			switch {
			case r == '\r':
				return -1
			case r < 0x20 || r == 0x7f:
				return ' '
			}
			return r
		}, result)
		args = append(args, h, firstRunes(result, 180)+" (background)")
	}
	if len(args) > 1 {
		u.queued("memories", args, memories.Run)
	}
}

// queueItems keeps the child's items for the user (project rules and decisions only the user makes).
func (u *runner) queueItems(run, sid, top string) {
	b, err := os.ReadFile(run + "/out/queue.md")
	if err != nil || len(b) == 0 {
		return
	}
	if queueSecret.Match(b) {
		b = []byte("- (a queued item was withheld: it held text that looked like a secret)\n")
	}
	t := strings.Map(func(r rune) rune {
		if r == '\r' || r < 0x20 && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, string(b))
	if len(t) > 20000 {
		t = t[:20000]
	}
	if top == "" {
		top = "(none)"
	}
	if os.MkdirAll(u.dir+"/queue", 0o777) == nil {
		name := time.Now().Format("20060102-150405") + "-" + u.session + ".md"
		os.WriteFile(u.dir+"/queue/"+name, []byte(fmt.Sprintf("session: %s\nproject: %s\nqueued: %s\n\n%s\n",
			sid, top, time.Now().Format("2006-01-02"), t)), 0o600)
	}
}

// pushScanned scans and risk-checks what main would send, then pushes exactly the commit checked; one retry
// after merging what arrived. Returns pushed, nothing, local, scan or risk.
func (u *runner) pushScanned() string {
	for try := 1; try <= 3; try++ {
		sha := u.out("rev-parse", "-q", "--verify", "main")
		if sha == "" {
			return "local"
		}
		if u.out("rev-parse", "-q", "--verify", "refs/remotes/origin/main") == sha {
			return "nothing"
		}
		if scan.Run(u.e, nil, io.Discard, io.Discard) != 0 {
			os.Unsetenv("JEEVES_AGE_KEY")
			return "scan"
		}
		if risk.Run(u.e, nil, io.Discard, io.Discard) != 0 {
			os.Unsetenv("JEEVES_AGE_KEY")
			return "risk"
		}
		os.Unsetenv("JEEVES_AGE_KEY")
		if u.out("rev-parse", "main") != sha {
			continue // a snapshot committed meanwhile: check again
		}
		if u.r.RemoteGit(io.Discard, io.Discard, "push", "-q", "origin", sha+":refs/heads/main") == 0 {
			u.git("update-ref", "refs/remotes/origin/main", sha)
			return "pushed"
		}
		if try != 1 {
			break
		}
		// Refused: what another device pushed is merged in (a conflict leaves it all for the user), then again.
		if u.r.RemoteGit(io.Discard, io.Discard, "fetch", "-q", "origin", "main") != 0 || !u.takeData() {
			break
		}
		mark := u.dir + "/in-progress"
		os.WriteFile(mark, []byte("merge\n"), 0o600)
		st, _ := u.r.Git("status", "--porcelain")
		if u.r.Branch() == "main" && st == "" && u.r.TreeProblem("origin/main") == "" &&
			u.git("merge", "-q", "--no-ff", "-m", "Merge origin/main (background update on "+u.e.Host+")", "origin/main") {
			os.Remove(mark)
			u.releaseData()
		} else {
			u.git("merge", "--abort")
			os.Remove(mark)
			u.releaseData()
			break
		}
	}
	return "local"
}
