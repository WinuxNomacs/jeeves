package lib

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The test binary doubles as a lock taker: LIB_TEST_ROLE=locker takes the lock in LIB_TEST_DIR, logs "in"
// and "out" around a short hold, and releases it.
func TestMain(m *testing.M) {
	if os.Getenv("LIB_TEST_ROLE") == "locker" {
		d := os.Getenv("LIB_TEST_DIR")
		l := TakeLock(d, 20)
		if l == nil {
			os.Exit(1)
		}
		appendLine(d+"/log", "in")
		time.Sleep(200 * time.Millisecond)
		appendLine(d+"/log", "out")
		l.Release()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func appendLine(p, s string) {
	f, _ := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	f.WriteString(s + "\n")
	f.Close()
}

func TestAStaleLockIsTakenOverByOneWaiterOnly(t *testing.T) {
	d := t.TempDir()
	exe, _ := os.Executable()
	for round := 0; round < 3; round++ {
		os.WriteFile(d+"/lock", []byte("999999999\n"), 0o600)
		var wg sync.WaitGroup
		for i := 0; i < 6; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// As the launcher runs a command: the lock PID is the shell's $$, which exec keeps (on Windows
				// Git Bash's kill -0 knows no Windows PID).
				c := exec.Command("bash", "-c", `export JEEVES_LOCK_PID=$$; exec "$0"`, exe)
				c.Env = append(os.Environ(), "LIB_TEST_ROLE=locker", "LIB_TEST_DIR="+d)
				if err := c.Run(); err != nil {
					t.Errorf("a waiter did not get the lock: %v", err)
				}
			}()
		}
		wg.Wait()
	}
	b, _ := os.ReadFile(d + "/log")
	lines := strings.Fields(string(b))
	if len(lines) != 36 {
		t.Fatalf("every run got the lock: %d lines", len(lines))
	}
	for i := 0; i < len(lines); i += 2 {
		if lines[i] != "in" || lines[i+1] != "out" {
			t.Fatalf("two at once: %v", lines)
		}
	}
}

func TestALockTakenDuringTheStaleCheckIsKept(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(d+"/lock", []byte("999999999\n"), 0o600)
	// The holder read releases and exits during the check, and another waiter takes a fresh lock.
	holderAlive = func(string) bool { os.WriteFile(d+"/lock", []byte("424242\n"), 0o600); return false }
	defer func() { holderAlive = alive }()
	if TakeLock(d, 0) != nil {
		t.Error("the lock was taken from its fresh holder")
	}
	if b, _ := os.ReadFile(d + "/lock"); string(b) != "424242\n" {
		t.Errorf("the fresh holder's lock was removed: %q", b)
	}
}

// liveBashPID is a running process's PID as bash sees it: a bash that becomes a sleep, for the test's length.
func liveBashPID(t *testing.T) string {
	t.Helper()
	c := exec.Command("bash", "-c", `echo $$; exec sleep 120`)
	out, _ := c.StdoutPipe()
	if err := c.Start(); err != nil {
		t.Skip("no bash:", err)
	}
	pid, _ := bufio.NewReader(out).ReadString('\n')
	t.Cleanup(func() { c.Process.Kill(); c.Wait() })
	return strings.TrimSpace(pid)
}

func TestALiveHoldersLockIsWaitedFor(t *testing.T) {
	d := t.TempDir()
	t.Setenv("JEEVES_LOCK_PID", liveBashPID(t))
	os.WriteFile(filepath.Join(d, "lock"), []byte(liveBashPID(t)+"\n"), 0o600)
	start := time.Now()
	if TakeLock(d, 1) != nil {
		t.Error("a live holder's lock was taken")
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Error("gave up before its time")
	}
	go func() { time.Sleep(300 * time.Millisecond); os.Remove(filepath.Join(d, "lock")) }()
	l := TakeLock(d, 5)
	if l == nil {
		t.Fatal("not taken once released")
	}
	if b, _ := os.ReadFile(filepath.Join(d, "lock")); strings.TrimSpace(string(b)) != lockPID() {
		t.Errorf("holds %q", b)
	}
	l.Release()
	if _, err := os.Stat(filepath.Join(d, "lock")); !os.IsNotExist(err) {
		t.Error("not released")
	}
}

func TestALockHeldPastTheLimitIsTakenOverEvenWithItsPIDAlive(t *testing.T) {
	d := t.TempDir()
	t.Setenv("JEEVES_LOCK_PID", liveBashPID(t))
	p := filepath.Join(d, "lock")
	os.WriteFile(p, []byte(liveBashPID(t)+"\n"), 0o600)
	old := time.Now().Add(-3 * time.Hour)
	os.Chtimes(p, old, old)
	l := TakeLock(d, 2)
	if l == nil {
		t.Fatal("a lock left three hours ago (its PID now someone else's) was not taken over")
	}
	l.Release()
}

func TestADeadlineStopsAHungCommand(t *testing.T) {
	start := time.Now()
	if rc := RunDeadline(1, exec.Command("sleep", "30")); rc != 124 {
		t.Errorf("status %d, want 124", rc)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("not within a few seconds")
	}
	if rc := RunDeadline(5, exec.Command("sh", "-c", "exit 3")); rc != 3 {
		t.Errorf("the command's own status otherwise: %d", rc)
	}
	if rc := RunDeadline(5, exec.Command(filepath.Join(t.TempDir(), "missing"))); rc != 127 {
		t.Errorf("a command that cannot start: %d", rc)
	}
}

func TestNamesOtherSystemsCannotHoldAreRefused(t *testing.T) {
	for p, bad := range map[string]bool{
		"skills/ab.md": false, "skills/a\tb.md": true, "skills/a\nb.md": true, "skills/a:b.md": true,
		"skills/a?.md": true, "skills/a./b.md": true, "skills/a /b.md": true, "skills/ab.": true,
		"skills/é ü.md": false,
	} {
		if got := BadName(p) != ""; got != bad {
			t.Errorf("%q refused = %v, want %v", p, got, bad)
		}
	}
}
