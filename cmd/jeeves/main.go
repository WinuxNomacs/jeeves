// Command jeeves is the Go build of Jeeves's scripts, one program with a subcommand per script
// (`jeeves status` is bin/jeeves-status). The bin/ launchers run it when dist/ holds a build for this OS
// and processor, and the bash scripts otherwise.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/status"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: jeeves <command> [arguments]   (commands: status)")
		os.Exit(2)
	}
	e := lib.LoadEnv(pluginRoot())
	switch os.Args[1] {
	case "status":
		os.Exit(status.Run(e, os.Args[2:], os.Stdout, os.Stderr))
	default:
		fmt.Fprintf(os.Stderr, "jeeves: unknown command %s\n", os.Args[1])
		os.Exit(2)
	}
}

// pluginRoot is the plugin folder in the scripts' form: JEEVES_ROOT from the launcher (bash's own
// `cd .. && pwd`, so a git setting naming bin/ reads the same whichever version wrote it), else the
// folder above this program's own.
func pluginRoot() string {
	if r := os.Getenv("JEEVES_ROOT"); r != "" {
		return lib.ShellPath(r)
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return lib.ShellPath(filepath.ToSlash(filepath.Dir(filepath.Dir(exe))))
}
