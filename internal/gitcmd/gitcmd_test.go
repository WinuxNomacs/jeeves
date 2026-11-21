package gitcmd

import "testing"

func TestRefusalStopsOptionsThatWriteRunOrReadOutside(t *testing.T) {
	for _, args := range [][]string{
		{"grep", "-O", "x"}, {"grep", "-Otouch PWNED", "x"}, {"grep", "-iO", "x"}, {"grep", "--open-files=touch PWNED", "x"},
		{"grep", "--open", "x"}, {"grep", "-f", "/etc/hostname"}, {"grep", "--no-index", "x"}, {"grep", "--file=/etc/x"},
		{"diff", "--no-index", "/dev/null", "/etc/hostname"}, {"diff", "--no-i", "/dev/null", "/etc/x"},
		{"diff", "--output=/tmp/x"}, {"log", "--out=/tmp/x"}, {"diff", "--ext-diff"}, {"diff", "-O/etc/x"},
		{"blame", "--contents", "/etc/x", "f"}, {"blame", "--cont=/etc/x", "f"}, {"blame", "-S", "/etc/x", "f"},
		{"blame", "--ignore-revs-file=/etc/x", "f"}, {"ls-files", "-X", "/etc/x"}, {"ls-files", "--exclude-from=/etc/x"},
		{"log", "--pathspec-from-file=/etc/x"}, {"push"}, {"-c", "x=y", "log"},
	} {
		if refusal(args) == "" {
			t.Errorf("not refused: %q", args)
		}
	}
	for _, args := range [][]string{
		{"log", "--oneline", "-5"}, {"log", "--format=%h %s", "--first-parent"}, {"diff", "--stat", "HEAD~1"},
		{"diff", "--no-color", "--cc", "--no-renames", "--no-ext-diff"}, {"show", "HEAD:home/CLAUDE.md"},
		{"grep", "-in", "rule"}, {"grep", "-e", "x", "--", "-O"}, {"ls-files", "--exclude-standard", "-o"},
		{"ls-files", "--exclude=*.md"}, {"status", "--porcelain"}, {"cat-file", "-p", "HEAD"},
		{"log", "-p", "--follow", "--", "home/CLAUDE.md"}, {"branch", "--show-current"},
	} {
		if why := refusal(args); why != "" {
			t.Errorf("refused %q: %s", args, why)
		}
	}
}
