package lib

import "os/exec"

// processAlive asks Git Bash's kill -0: the PID is one Git Bash gave (a bash script's $$, or the launcher's
// for the Go build), which Windows does not know.
func processAlive(_ int, pid string) bool {
	return exec.Command("bash", "-c", `kill -0 "$1" 2>/dev/null`, "_", pid).Run() == nil
}
