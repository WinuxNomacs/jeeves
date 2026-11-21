package lib

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/term"
)

// Passphrases come from a dialog of the OS, never a terminal Claude reads and never the chat; with
// terminal set, from the user's own terminal with no echo. They stay in this process's memory only.

// ErrCancelled: the user cancelled the dialog (or gave nothing).
var ErrCancelled = errors.New("cancelled")

// ErrNoDialog: no dialog can be shown here (the user runs the command with --terminal in their own
// terminal).
var ErrNoDialog = errors.New("no dialog")

// AskPassphrase asks for a passphrase with msg. JEEVES_ASKPASS, the tests' stand-in, names a program that
// prints the answer.
func AskPassphrase(msg string, terminal bool) (string, error) {
	if ask := os.Getenv("JEEVES_ASKPASS"); ask != "" {
		out, err := Command(ask, msg).Output()
		if ExitCode(err) == 3 { // the stand-in for "no dialog here"
			return "", ErrNoDialog
		}
		return answer(string(out), err)
	}
	if terminal {
		return fromTerminal(msg)
	}
	switch OSName() {
	case "macos":
		out, err := Command("osascript", "-e", "on run argv",
			"-e", `display dialog (item 1 of argv) default answer "" with hidden answer with title "Jeeves" with icon caution`,
			"-e", "text returned of result", "-e", "end run", msg).Output()
		return answer(string(out), err)
	case "windows":
		return windowsDialog(msg)
	case "wsl":
		if LookPath("powershell.exe") != "" {
			return windowsDialog(msg)
		}
	}
	if os.Getenv("DISPLAY")+os.Getenv("WAYLAND_DISPLAY") != "" {
		if LookPath("zenity") != "" {
			out, err := Command("zenity", "--password", "--title=Jeeves: "+msg).Output()
			return answer(string(out), err)
		}
		if LookPath("kdialog") != "" {
			out, err := Command("kdialog", "--title", "Jeeves", "--password", msg).Output()
			return answer(string(out), err)
		}
	}
	return "", ErrNoDialog
}

// answer is a dialog's output: its first line, or ErrCancelled when it failed or gave nothing.
func answer(out string, err error) (string, error) {
	p := strings.TrimRight(strings.SplitN(out, "\n", 2)[0], "\r")
	if err != nil || p == "" {
		return "", ErrCancelled
	}
	return p, nil
}

// windowsDialog is Windows' own credential prompt (a window, also from WSL); the message goes in on stdin.
func windowsDialog(msg string) (string, error) {
	out, err := powershellIn(`[Console]::OutputEncoding = [Text.Encoding]::UTF8
$m = [Console]::In.ReadLine()
$c = $Host.UI.PromptForCredential("Jeeves", $m, "Jeeves", "")
if ($c -eq $null) { exit 1 }
[Console]::Out.Write($c.GetNetworkCredential().Password)`, msg+"\n", true)
	return answer(out, err)
}

// fromTerminal reads from the controlling terminal with no echo.
func fromTerminal(msg string) (string, error) {
	name := "/dev/tty"
	if OSName() == "windows" {
		name = "CONIN$"
	}
	in, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil || !term.IsTerminal(int(in.Fd())) {
		return "", ErrNoDialog
	}
	defer in.Close()
	out := in
	if OSName() == "windows" {
		if o, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
			defer o.Close()
			out = o
		}
	}
	out.WriteString(msg + ": ")
	b, err := term.ReadPassword(int(in.Fd()))
	out.WriteString("\n")
	if err != nil || len(b) == 0 {
		return "", ErrCancelled
	}
	return string(b), nil
}
