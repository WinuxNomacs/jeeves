// Package gitcmd is `jeeves git`, the Go port of bin/jeeves-git: git in the data repo, so the skills need
// no `cd` into it (the Bash tool's folder is not kept outside the project, and a permission rule does not
// match past such a `cd`). It works on a locked device too.
package gitcmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Run runs git with args in the data repo and returns git's exit status.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	a, args := lib.ParseDataRepoArg(args)
	os.Setenv("ALLOW_LOCKED", "1")
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "jeeves git: %s\n", err)
		return 1
	}
	// The same options as the scripts' git_repo, so jeeves git and jeeves status agree on what is dirty.
	cmd := exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false",
		"-c", "core.excludesFile="}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	signal.Ignore(os.Interrupt) // git gets the terminal's Ctrl+C itself; this waits for it to end
	return lib.ExitCode(cmd.Run())
}
