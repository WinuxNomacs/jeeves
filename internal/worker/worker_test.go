package worker

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestQueuedCommandsAreThoseThatChangeTheDataRepoOrClaudeFolder(t *testing.T) {
	for _, c := range []struct {
		cmd  string
		args []string
		want bool
	}{
		{"snapshot", nil, true}, {"apply", []string{"--show"}, true}, {"restore", nil, true},
		{"cleanup", []string{"--auto"}, true}, {"memories", []string{"record", "h", "o"}, true},
		{"memories", []string{"--data-repo", "/x", "record", "h", "o"}, true},
		{"memories", []string{"list"}, false}, {"status", nil, false}, {"git", []string{"push"}, false},
		{"worker", nil, false},
	} {
		if got := Queued(c.cmd, c.args); got != c.want {
			t.Errorf("Queued(%s %v) = %v, want %v", c.cmd, c.args, got, c.want)
		}
	}
}

func TestTheAgeKeyAndTheCallersLockPIDNeverTravelWithARequest(t *testing.T) {
	env := callerEnv([]string{"JEEVES_AGE_KEY=AGE-SECRET-KEY-1X", "JEEVES_LOCK_PID=42", "JEEVES_HOST=h",
		"HOME=/home/u", "GH_TOKEN=secret", "ANTHROPIC_API_KEY=secret", "FAKE_LOG=/l", "PATH=/bin"})
	for _, k := range []string{"JEEVES_AGE_KEY", "JEEVES_LOCK_PID", "GH_TOKEN", "ANTHROPIC_API_KEY", "FAKE_LOG"} {
		if _, ok := env[k]; ok {
			t.Errorf("%s was passed", k)
		}
	}
	for _, k := range []string{"JEEVES_HOST", "HOME", "PATH"} {
		if _, ok := env[k]; !ok {
			t.Errorf("%s was not passed", k)
		}
	}
}

func TestJeevesWorkerPassNamesMoreVariablesForTheTestsFakes(t *testing.T) {
	env := callerEnv([]string{"JEEVES_WORKER_PASS=FAKE_LOG JEEVES_AGE_KEY", "FAKE_LOG=/l", "JEEVES_AGE_KEY=k"})
	if env["FAKE_LOG"] != "/l" {
		t.Errorf("FAKE_LOG not passed: %v", env)
	}
	if _, ok := env["JEEVES_AGE_KEY"]; ok {
		t.Error("JEEVES_WORKER_PASS let the age key through")
	}
}

func TestTheChildGetsTheRequestsVariablesAndTheWorkersOthers(t *testing.T) {
	r := &Request{Env: map[string]string{"HOME": "/caller", "JEEVES_HOST": "c"}}
	got := childEnv([]string{"HOME=/worker", "JEEVES_DATA=/w", "GH_TOKEN=t", "JEEVES_AGE_KEY=k", "JEEVES_LOCK_PID=1"}, r)
	want := []string{"GH_TOKEN=t", "HOME=/caller", "JEEVES_HOST=c"}
	if !slices.Equal(got, want) {
		t.Errorf("childEnv = %v, want %v (a variable the caller had unset stays unset)", got, want)
	}
}

func TestFramesGiveBackEachStreamInTheOrderWritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	fr := &framer{f: f}
	o, e := frameWriter{fr, 'o'}, frameWriter{fr, 'e'}
	o.Write([]byte("one\n"))
	e.Write([]byte("two\n"))
	o.Write([]byte("three\n"))
	f.Close()
	var both, out, errs bytes.Buffer
	data, _ := os.ReadFile(p)
	replay(bytes.NewReader(data), &both, &both)
	replay(bytes.NewReader(data), &out, &errs)
	if both.String() != "one\ntwo\nthree\n" || out.String() != "one\nthree\n" || errs.String() != "two\n" {
		t.Errorf("replay: %q / %q %q", both.String(), out.String(), errs.String())
	}
	// A frame cut short (a child killed mid-write) ends the replay without garbage.
	var cut bytes.Buffer
	replay(bytes.NewReader(data[:len(data)-3]), &cut, &cut)
	if cut.String() != "one\ntwo\n" {
		t.Errorf("cut replay: %q", cut.String())
	}
}

func TestARequestThatDoesNotReadOrIsNotOneTheWorkerRunsIsRefusedWithWhy(t *testing.T) {
	q := t.TempDir()
	write := func(id, body string) string {
		p := filepath.Join(q, id+".req")
		os.WriteFile(p, []byte(body), 0o600)
		return p
	}
	for body, why := range map[string]string{
		"not json": "does not read",
		`{"version":99,"id":"a","command":"snapshot"}`:                "format version 99",
		`{"version":1,"id":"other","command":"snapshot"}`:             "does not match",
		`{"version":1,"id":"a","command":"git","args":["push","-f"]}`: "not a command the worker runs",
	} {
		if _, err := readRequest(write("a", body)); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: err %v, want %q", body, err, why)
		}
	}
	if r, err := readRequest(write("a", `{"version":1,"id":"a","command":"snapshot","args":[]}`)); err != nil || r.Command != "snapshot" {
		t.Errorf("a good request: %v %v", r, err)
	}
}

func TestRequestIDsSortInArrivalOrder(t *testing.T) {
	a := newID()
	b := newID()
	if !(a < b) {
		t.Errorf("%s then %s", a, b)
	}
}

func TestOnlyIdenticalRequestsAreRunOnce(t *testing.T) {
	a := &Request{Command: "snapshot", Args: []string{"--hook"}, Dir: "/d", Exe: "/x", Env: map[string]string{"HOME": "/h"}}
	b := *a
	if !same(a, &b) {
		t.Error("identical requests differ")
	}
	c := *a
	c.Env = map[string]string{"HOME": "/other"}
	d := *a
	d.Args = []string{"--hook", "--no-pull"}
	if same(a, &c) || same(a, &d) {
		t.Error("different requests counted as the same")
	}
}
