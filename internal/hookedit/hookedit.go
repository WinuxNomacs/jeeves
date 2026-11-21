// Package hookedit is `jeeves hook-edit`, the Go port of bin/jeeves-hook-edit: the PostToolUse hook
// (Write|Edit) that snapshots right after Claude edits ~/.claude/CLAUDE.md, so the edit is recorded at once.
// It reads the hook's JSON from stdin; any other file is ignored. Always exits 0, and reports to the user
// through a systemMessage only when the snapshot printed something.
package hookedit

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/snapshot"
	"github.com/WinuxNomacs/jeeves/internal/worker"
)

// Stdin is where the hook's JSON is read from (the tests' seam).
var Stdin io.Reader = os.Stdin

// filePath is tool_input.file_path, read as the bash version does: the first "file_path" string value, a
// backslash taking the next character as it is (enough for paths).
func filePath(in string) string {
	in = strings.ReplaceAll(in, "\n", " ")
	i := strings.Index(in, `"file_path"`)
	if i < 0 {
		return ""
	}
	s := strings.TrimLeft(in[i+len(`"file_path"`):], " \t")
	if !strings.HasPrefix(s, ":") {
		return ""
	}
	s = strings.TrimLeft(s[1:], " \t")
	if !strings.HasPrefix(s, `"`) {
		return ""
	}
	s = s[1:]
	var out strings.Builder
	for len(s) > 0 {
		switch c := s[0]; c {
		case '"':
			return out.String()
		case '\\':
			if len(s) > 1 {
				out.WriteByte(s[1])
				s = s[2:]
			} else {
				s = s[1:]
			}
		default:
			out.WriteByte(c)
			s = s[1:]
		}
	}
	return ""
}

// realDir is a folder's physical path, "" when it cannot be resolved.
func realDir(p string) string {
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	if fi, err := os.Stat(r); err != nil || !fi.IsDir() {
		return ""
	}
	return filepath.Clean(r)
}

// jsonString escapes as the bash version does: backslash, double quote and tab, and line breaks as \n.
func jsonString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

// Run is the hook. Always 0.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	// A background update's own session touches nothing.
	if os.Getenv("JEEVES_AUTO_CHILD") == "1" {
		return 0
	}
	in, _ := io.ReadAll(Stdin)
	p := filePath(string(in))
	if p == "" {
		return 0
	}
	// Native Windows sends C:\Users\...: forward slashes; a POSIX form (/c/Users/...) is made native.
	p = lib.NativePath(strings.ReplaceAll(p, `\`, "/"))
	if filepath.Base(p) != "CLAUDE.md" {
		return 0
	}
	home := realDir(e.ClaudeHome)
	if home == "" || realDir(filepath.Dir(p)) != home {
		return 0
	}
	var out bytes.Buffer
	worker.Run(e, "snapshot", []string{"--hook", "--no-pull"}, &out, &out, snapshot.Run)
	text := strings.TrimRight(out.String(), "\n")
	if text == "" {
		return 0
	}
	fmt.Fprintf(stdout, "{\"systemMessage\": \"%s\"}\n", jsonString(text))
	return 0
}
