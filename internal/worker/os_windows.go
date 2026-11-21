package worker

import (
	"os"
	"os/exec"
	"strconv"

	"golang.org/x/sys/windows"
)

// tryLock locks one byte of p far past its end (LockFileEx, exclusive, without waiting), on a handle
// opened with full sharing, so the file stays readable, writable and deletable while the worker lives;
// Windows releases the lock when the process ends, so a dead worker never holds the queue. The handle is
// not inheritable.
func tryLock(p string, mark bool) (*os.File, bool) {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return nil, false
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, false
	}
	ol := &windows.Overlapped{OffsetHigh: 1} // byte 4 GiB: never part of the file's content
	if windows.LockFileEx(h, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol) != nil {
		windows.CloseHandle(h)
		return nil, false
	}
	f := os.NewFile(uintptr(h), p)
	if mark { // the worker's PID, for a person looking
		f.Truncate(0)
		f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	}
	return f, true
}

// startDetached starts the worker outside the caller's console and, where the job allows it, outside the
// caller's job, so the end of the hook that started it does not end it.

// childCommand runs the request's command under a Git Bash shell that stays for its whole run: the data
// repo's lock holds that shell's $$, which the bash scripts' kill -0 can check (it knows no Windows PID).
func childCommand(exe string, args []string) *exec.Cmd {
	return exec.Command("bash", append([]string{"-c", `export JEEVES_LOCK_PID=$$; "$@"`, "_", exe}, args...)...)
}

// openLog opens the worker's log for appending, shared for deletion, so the queue folder can be removed
// while a worker still runs (the file goes when it closes).
func openLog(p string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.FILE_APPEND_DATA,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), p), nil
}
