package proc

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as the processes the tests supervise: PROC_TEST_ROLE picks the role.
func TestMain(m *testing.M) {
	switch os.Getenv("PROC_TEST_ROLE") {
	case "sleeper": // sleeps, after starting a sleeping grandchild and printing its PID
		gc := exec.Command(self())
		gc.Env = append(os.Environ(), "PROC_TEST_ROLE=leaf")
		gc.Start()
		os.Stdout.WriteString(strconv.Itoa(gc.Process.Pid) + "\n")
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	case "leaf":
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	case "supervisor": // starts a contained sleeper, prints the sleeper's and grandchild's PIDs, then waits
		c := exec.Command(self())
		c.Env = append(os.Environ(), "PROC_TEST_ROLE=sleeper")
		out, _ := c.StdoutPipe()
		if _, err := StartContained(c); err != nil {
			os.Stdout.WriteString("error " + err.Error() + "\n")
			os.Exit(1)
		}
		gc, _ := bufio.NewReader(out).ReadString('\n')
		os.Stdout.WriteString(strconv.Itoa(c.Process.Pid) + " " + gc)
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func gone(pid int, within time.Duration) bool {
	for end := time.Now().Add(within); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		if !Alive(pid) {
			return true
		}
	}
	return false
}

func TestStoppingAContainedChildEndsWhatItStartedToo(t *testing.T) {
	c := exec.Command(self())
	c.Env = append(os.Environ(), "PROC_TEST_ROLE=sleeper")
	out, _ := c.StdoutPipe()
	k, err := StartContained(c)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	line, _ := bufio.NewReader(out).ReadString('\n')
	gc, _ := strconv.Atoi(strings.TrimSpace(line))
	if !Alive(c.Process.Pid) || !Alive(gc) {
		t.Fatalf("not started: child %d grandchild %d", c.Process.Pid, gc)
	}
	k.Stop(Wait(c), 5*time.Second)
	if !gone(c.Process.Pid, 5*time.Second) {
		t.Error("the child still runs")
	}
	if !gone(gc, 5*time.Second) {
		t.Error("the grandchild still runs")
	}
}

func TestOnWindowsAContainedChildEndsWithItsSupervisorKilledOutright(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the job object is Windows'; elsewhere a watchdog process does this")
	}
	s := exec.Command(self())
	s.Env = append(os.Environ(), "PROC_TEST_ROLE=supervisor")
	out, _ := s.StdoutPipe()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	f := strings.Fields(line)
	if len(f) != 2 {
		s.Process.Kill()
		t.Fatalf("supervisor said %q", line)
	}
	child, _ := strconv.Atoi(f[0])
	gc, _ := strconv.Atoi(f[1])
	s.Process.Kill()
	s.Wait()
	if !gone(child, 10*time.Second) || !gone(gc, 10*time.Second) {
		StopPID(child, 0)
		StopPID(gc, 0)
		t.Fatal("the contained child outlived its supervisor")
	}
}

func TestStopPIDEndsAProcessThisOneDidNotStart(t *testing.T) {
	c := exec.Command(self())
	c.Env = append(os.Environ(), "PROC_TEST_ROLE=leaf")
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	done := Wait(c)
	StopPID(c.Process.Pid, 5*time.Second)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		c.Process.Kill()
		t.Fatal("still running")
	}
	if Alive(-1) || Alive(0) {
		t.Error("no PID is alive below 1")
	}
}

// self is this test binary, by an absolute path (os.Args[0] may be relative, which exec refuses).
func self() string {
	p, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return p
}
