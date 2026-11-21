// Command jeeves is the Go build of Jeeves's scripts, one program with a subcommand per script
// (`jeeves status` is bin/jeeves-status). The bin/ launchers run it when dist/ holds a build for this OS
// and processor, and the bash scripts otherwise.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/WinuxNomacs/jeeves/internal/check"
	"github.com/WinuxNomacs/jeeves/internal/gitcmd"
	"github.com/WinuxNomacs/jeeves/internal/github"
	"github.com/WinuxNomacs/jeeves/internal/initcmd"
	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/memories"
	"github.com/WinuxNomacs/jeeves/internal/risk"
	"github.com/WinuxNomacs/jeeves/internal/scan"
	"github.com/WinuxNomacs/jeeves/internal/status"
	"github.com/WinuxNomacs/jeeves/internal/updatecheck"
)

// commands maps each subcommand to its Run.
var commands = map[string]func(lib.Env, []string, io.Writer, io.Writer) int{
	"check":        check.Run,
	"git":          gitcmd.Run,
	"github":       github.Run,
	"init":         initcmd.Run,
	"memories":     memories.Run,
	"risk":         risk.Run,
	"scan":         scan.Run,
	"status":       status.Run,
	"update-check": updatecheck.Run,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: jeeves <command> [arguments]   (commands: check, git, github, init, memories, risk, scan, status, update-check)")
		os.Exit(2)
	}
	if os.Getenv("JEEVES_TEST_PANIC") == "1" { // the tests' way to make the program crash
		panic("JEEVES_TEST_PANIC")
	}
	e := lib.LoadEnv(pluginRoot)
	run, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "jeeves: unknown command %s\n", os.Args[1])
		os.Exit(2)
	}
	os.Exit(run(e, os.Args[2:], os.Stdout, os.Stderr))
}

// pluginRoot is the plugin folder in the scripts' form (worked out once, on first use): JEEVES_ROOT from the launcher (bash's own
// `cd .. && pwd`, so a git setting naming bin/ reads the same whichever version wrote it), else the
// folder above this program's own.
var pluginRoot = sync.OnceValue(func() string {
	if r := os.Getenv("JEEVES_ROOT"); r != "" {
		return lib.ShellPath(r)
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return lib.ShellPath(filepath.ToSlash(filepath.Dir(filepath.Dir(exe))))
})
