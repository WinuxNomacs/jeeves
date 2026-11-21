package lib

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClassifyMatchesTheThreeWayRules(t *testing.T) {
	cases := []struct{ d, r, b, want string }{
		{"a", "a", "", "insync"},
		{"a", "a", "x", "insync"},
		{"a", "-", "", "disk"},
		{"-", "a", "", "repo"},
		{"a", "b", "", "conflict"},
		{"a", "b", "b", "disk"},
		{"a", "b", "a", "repo"},
		{"a", "b", "c", "conflict"},
		{"-", "b", "b", "disk"},
	}
	for _, c := range cases {
		if got := Classify(c.d, c.r, c.b); got != c.want {
			t.Errorf("Classify(%q,%q,%q) = %q, want %q", c.d, c.r, c.b, got, c.want)
		}
	}
}

func TestHashOfGivesGitsBlobHash(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"empty": "", "text": "rules\r\nv2\n", "bin": "\x00\xff"} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o644)
		out, err := exec.Command("git", "hash-object", "--no-filters", "--", p).Output()
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := HashOf(p); got != strings.TrimSpace(string(out)) {
			t.Errorf("%s: %s, git says %s", name, got, out)
		}
	}
}

func TestHashOfAMissingFileOrAFolderIsADash(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{filepath.Join(dir, "nope"), dir} {
		if got, err := HashOf(p); got != "-" || err != nil {
			t.Errorf("%s: %q %v", p, got, err)
		}
	}
}

func TestHashOfAnUnreadableFileIsEmptyWithAnError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permissions do not stop this user reading")
	}
	p := filepath.Join(t.TempDir(), "locked")
	os.WriteFile(p, []byte("x"), 0o000)
	if got, err := HashOf(p); got != "" || err == nil {
		t.Errorf("got %q %v", got, err)
	}
}

func TestTrackedUnderSkipsExcludedFoldersAndSymlinkedFolders(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"CLAUDE.md", "agents/a.md", "skills/s/SKILL.md", "skills/synced/x.md",
		"commands/.trash/old.md", "other/z.md", "real/r.md"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, f)), 0o755)
		os.WriteFile(filepath.Join(root, f), []byte(f), 0o644)
	}
	if runtime.GOOS != "windows" {
		os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "commands"))
		os.Symlink(filepath.Join(root, "real/r.md"), filepath.Join(root, "agents/link.md"))
	}
	got := strings.Join(TrackedUnder(root), " ")
	if got != "CLAUDE.md agents/a.md skills/s/SKILL.md" {
		t.Errorf("got %q", got)
	}
}

func TestAllTrackedIsTheSortedUnion(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for root, files := range map[string][]string{a: {"CLAUDE.md", "skills/b.md"}, b: {"CLAUDE.md", "agents/a.md", "skills/B.md"}} {
		for _, f := range files {
			os.MkdirAll(filepath.Dir(filepath.Join(root, f)), 0o755)
			os.WriteFile(filepath.Join(root, f), nil, 0o644)
		}
	}
	if got := strings.Join(AllTracked(a, b), " "); got != "CLAUDE.md agents/a.md skills/B.md skills/b.md" {
		t.Errorf("got %q", got)
	}
}

func TestBasesReadsTheSyncRecord(t *testing.T) {
	r := &Repo{StateDir: t.TempDir()}
	if len(r.Bases()) != 0 {
		t.Error("a missing record should give no bases")
	}
	os.WriteFile(r.StateDir+"/hashes", []byte("h1\tCLAUDE.md\nh2\tagents/a b.md\ngarbage\n"), 0o644)
	m := r.Bases()
	if m["CLAUDE.md"] != "h1" || m["agents/a b.md"] != "h2" || len(m) != 2 {
		t.Errorf("got %v", m)
	}
}

// scratch makes a home folder, a ~/.claude and a data repo, and points the environment at them.
func scratch(t *testing.T) (Env, string) {
	t.Helper()
	home := t.TempDir()
	repo := filepath.Join(home, "data")
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatal(string(out))
	}
	t.Setenv("JEEVES_DATA", "")
	t.Setenv("CLAUDE_PLUGIN_OPTION_DATA_REPO", "")
	t.Setenv("ALLOW_LOCKED", "")
	return Env{Home: home, ClaudeHome: filepath.Join(home, ".claude"), Host: "testhost", Root: func() string { return "/plugin" }}, repo
}

func TestFindDataRepoWithNothingSetPointsToSetup(t *testing.T) {
	e, _ := scratch(t)
	t.Chdir(e.Home)
	_, err := FindDataRepo(e, DataRepoArg{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not set up on this machine yet (no data repo at "+ShellPath(e.Home+"/jeeves-data")) {
		t.Errorf("got %v", err)
	}
}

func TestFindDataRepoRefusesAFolderThatIsNotARepo(t *testing.T) {
	e, _ := scratch(t)
	_, err := FindDataRepo(e, DataRepoArg{JeevesData: e.Home, Given: true}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "is not a git repository") {
		t.Errorf("got %v", err)
	}
}

func TestFindDataRepoRefusesAnOddHostName(t *testing.T) {
	e, repo := scratch(t)
	for _, h := range []string{"", ".memory-review", "a/b", `a\b`} {
		e.Host = h
		if _, err := FindDataRepo(e, DataRepoArg{JeevesData: repo}, &bytes.Buffer{}); err == nil {
			t.Errorf("host %q accepted", h)
		}
	}
}

func TestFindDataRepoRecordsAGivenRepoAndReadsTheRecordLater(t *testing.T) {
	e, repo := scratch(t)
	r, err := FindDataRepo(e, DataRepoArg{JeevesData: repo, Given: true}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(e.pointerFile()); string(b) != r.ShellDir+"\n" {
		t.Errorf("record holds %q", b)
	}
	t.Chdir(e.Home)
	r2, err := FindDataRepo(e, DataRepoArg{}, &bytes.Buffer{})
	if err != nil || r2.Dir != r.Dir {
		t.Errorf("from the record: %v %v", r2, err)
	}
}

func TestARecordThatCannotBeWrittenIsReported(t *testing.T) {
	e, repo := scratch(t)
	os.MkdirAll(e.ClaudeHome, 0o755)
	os.WriteFile(e.ClaudeHome+"/jeeves", []byte("a file where the folder goes"), 0o644)
	var stderr bytes.Buffer
	if _, err := FindDataRepo(e, DataRepoArg{JeevesData: repo, Given: true}, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "could not record the data repo") {
		t.Errorf("stderr: %q", stderr.String())
	}
}

func TestALockedEncryptedRepoIsRefusedUnlessAllowed(t *testing.T) {
	e, repo := scratch(t)
	os.WriteFile(repo+"/recipient", []byte("age1abc\n"), 0o644)
	_, err := FindDataRepo(e, DataRepoArg{JeevesData: repo}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "this device is locked") {
		t.Errorf("got %v", err)
	}
	t.Setenv("ALLOW_LOCKED", "1")
	if _, err := FindDataRepo(e, DataRepoArg{JeevesData: repo}, &bytes.Buffer{}); err != nil {
		t.Errorf("with ALLOW_LOCKED: %v", err)
	}
	out, _ := exec.Command("git", "-C", repo, "config", "filter.jeeves.clean").Output()
	if string(out) != `"/plugin/bin/jeeves-crypt" clean %f`+"\n" {
		t.Errorf("filter not set up: %q", out)
	}
}

func TestAnUnlockedDeviceNeedsTheMarkerToNameTheRecipient(t *testing.T) {
	e, repo := scratch(t)
	os.WriteFile(repo+"/recipient", []byte("age1abc\n"), 0o644)
	os.MkdirAll(repo+"/.git/jeeves", 0o755)
	os.WriteFile(repo+"/.git/jeeves/unlocked", []byte("age1other\n"), 0o644)
	if _, err := FindDataRepo(e, DataRepoArg{JeevesData: repo}, &bytes.Buffer{}); err == nil {
		t.Error("a marker for another recipient unlocked the device")
	}
	os.WriteFile(repo+"/.git/jeeves/unlocked", []byte("age1abc\n\n"), 0o644)
	if _, err := FindDataRepo(e, DataRepoArg{JeevesData: repo}, &bytes.Buffer{}); err != nil {
		t.Errorf("the right marker: %v", err)
	}
}

func TestParseDataRepoArg(t *testing.T) {
	t.Setenv("JEEVES_DATA", "")
	a, rest := ParseDataRepoArg([]string{"--data-repo", "/x", "more"})
	if a.JeevesData != "/x" || !a.Given || len(rest) != 1 {
		t.Errorf("got %+v %v", a, rest)
	}
	a, rest = ParseDataRepoArg([]string{"--data-repo", "${user_config.data_repo}"})
	if a.Given || len(rest) != 0 {
		t.Errorf("placeholder: %+v %v", a, rest)
	}
	a, _ = ParseDataRepoArg([]string{"--data-repo", ""})
	if !a.Given || a.JeevesData != "" {
		t.Errorf("empty: %+v", a)
	}
}
