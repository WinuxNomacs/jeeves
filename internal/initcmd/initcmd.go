// Package initcmd is `jeeves init`, the Go port of bin/jeeves-init: a new, empty data repo.
package initcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

const gitignore = `# Per-device sync record (hashes at the last sync, overwritten copies); never shared.
.sync-state/
# Interrupted atomic copies.
*.sync-tmp
`

const gitattributes = `# Byte for byte: no line-ending conversion, also in a Windows checkout (core.autocrlf).
* -text
`

const readme = "# Jeeves data repo\n\nGlobal Claude Code preferences, versioned by the [Jeeves](https://github.com/WinuxNomacs/jeeves) plugin.\n\n" +
	"- `home/`: shared by every device: `~/.claude/` `CLAUDE.md`, `agents/`, `commands/`, `skills/`.\n" +
	"- `hosts/<host>/`: each device's backup of its `settings.json`, `keybindings.json` and memories, laid out\n" +
	"  as in `~/.claude`. Restore one onto a machine with `/jeeves:restore <host>`.\n"

// Run: jeeves init <folder> [<remote url>]. The folder must not exist yet, or be empty. Makes a git repo on
// main with .gitignore, .gitattributes and a README, commits them, and sets origin when a remote URL (of an
// empty repo) is given. It never pushes: /jeeves:update does, after the first snapshot.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	say := func(f string, a ...any) { fmt.Fprintf(stdout, "jeeves init: "+f+"\n", a...) }
	if len(args) != 1 && len(args) != 2 {
		fmt.Fprintln(stderr, "usage: jeeves-init <folder> [<remote url>]")
		return 2
	}
	dir, url := args[0], ""
	if len(args) == 2 {
		url = args[1]
	}
	if strings.HasPrefix(dir, "~/") {
		dir = e.Home + "/" + dir[2:]
	}
	if _, err := os.Stat(dir); err == nil {
		if ents, _ := os.ReadDir(dir); len(ents) > 0 {
			say("%s is not empty; clone an existing data repo there instead, or pick another folder", lib.ShellArg(dir))
			return 1
		}
	}
	if os.MkdirAll(dir, 0o777) != nil {
		fmt.Fprintf(stderr, "mkdir: cannot create directory '%s'\n", lib.ShellArg(dir))
		return 1
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return 1
	}
	git := func(a ...string) bool {
		cmd := exec.Command("git", a...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, stdout, stderr
		return cmd.Run() == nil
	}
	if !git("init", "-q") || !git("symbolic-ref", "HEAD", "refs/heads/main") {
		return 1
	}
	for name, text := range map[string]string{".gitignore": gitignore, ".gitattributes": gitattributes, "README.md": readme} {
		if os.WriteFile(filepath.Join(dir, name), []byte(text), 0o666) != nil {
			return 1
		}
	}
	if !git("add", "-A") || !git("commit", "-q", "-m", "Data repo for Jeeves") {
		say("commit failed")
		return 1
	}
	if url != "" && !git("remote", "add", "origin", url) {
		return 1
	}
	abs, _ := filepath.Abs(dir)
	msg := "created " + lib.ShellPath(filepath.ToSlash(abs))
	if url != "" {
		msg += " with origin " + lib.ShellArg(url)
	}
	say("%s", msg)
	return 0
}
