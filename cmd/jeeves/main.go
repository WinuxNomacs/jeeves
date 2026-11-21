// Command jeeves is the Go build of Jeeves's scripts, one program with a subcommand per script
// (`jeeves status` is bin/jeeves-status). The bin/ launchers run it from dist/, the build for this OS and
// processor (lib/launch.sh).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/WinuxNomacs/jeeves/internal/apply"
	"github.com/WinuxNomacs/jeeves/internal/check"
	"github.com/WinuxNomacs/jeeves/internal/cleanup"
	"github.com/WinuxNomacs/jeeves/internal/gitcmd"
	"github.com/WinuxNomacs/jeeves/internal/github"
	"github.com/WinuxNomacs/jeeves/internal/hookedit"
	"github.com/WinuxNomacs/jeeves/internal/initcmd"
	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/memories"
	"github.com/WinuxNomacs/jeeves/internal/restore"
	"github.com/WinuxNomacs/jeeves/internal/risk"
	"github.com/WinuxNomacs/jeeves/internal/scan"
	"github.com/WinuxNomacs/jeeves/internal/snapshot"
	"github.com/WinuxNomacs/jeeves/internal/status"
	"github.com/WinuxNomacs/jeeves/internal/updatecheck"
	"github.com/WinuxNomacs/jeeves/internal/worker"
)

// commands maps each subcommand to its Run.
var commands = map[string]func(lib.Env, []string, io.Writer, io.Writer) int{
	"apply":        apply.Run,
	"check":        check.Run,
	"cleanup":      cleanup.Run,
	"git":          gitcmd.Run,
	"github":       github.Run,
	"hook-edit":    hookedit.Run,
	"init":         initcmd.Run,
	"memories":     memories.Run,
	"restore":      restore.Run,
	"risk":         risk.Run,
	"scan":         scan.Run,
	"snapshot":     snapshot.Run,
	"status":       status.Run,
	"update-check": updatecheck.Run,
	"worker":       worker.Serve,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: jeeves <command> [arguments]   (commands: apply, check, cleanup, git, github, hook-edit, init, memories, restore, risk, scan, snapshot, status, update-check, worker)")
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
	var stdout, stderr io.Writer = os.Stdout, os.Stderr
	if o, x, ok := worker.ChildOutput(); ok { // this process runs a queued request
		stdout, stderr = o, x
	}
	// The commands that change the data repo or ~/.claude go through the machine's worker, one at a time.
	os.Exit(worker.Run(e, os.Args[1], os.Args[2:], stdout, stderr, run))
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
