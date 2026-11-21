package proc

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	detachedProcess        = 0x00000008
	createNewProcessGroup  = 0x00000200
	createBreakawayFromJob = 0x01000000
	createSuspended        = 0x00000004
)

// StartDetached starts cmd outside the caller's console and, where the caller's job allows it, outside
// that job, so the end of the hook that started it does not end it.
func StartDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true,
		CreationFlags: detachedProcess | createNewProcessGroup | createBreakawayFromJob}
	if err := cmd.Start(); err == nil {
		return nil
	}
	c2 := exec.Command(cmd.Path, cmd.Args[1:]...)
	c2.Env, c2.Dir, c2.Stdin, c2.Stdout, c2.Stderr = cmd.Env, cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr
	c2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachedProcess | createNewProcessGroup}
	*cmd = *c2
	return cmd.Start()
}

// Contained is a started child in a job object that kills its processes when its last handle closes: the
// supervisor holds that handle, so the child and all it starts end with the supervisor, however that ends
// (a forced stop, a crash), with no watchdog.
type Contained struct {
	cmd *exec.Cmd
	job windows.Handle
}

// StartContained starts cmd suspended, puts it in a new kill-on-close job, then lets it run, so nothing it
// starts escapes the job.
func StartContained(cmd *exec.Cmd) (*Contained, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createSuspended | createNewProcessGroup}
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	fail := func(err error) (*Contained, error) {
		cmd.Process.Kill()
		cmd.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(err)
	}
	err = windows.AssignProcessToJobObject(job, ph)
	windows.CloseHandle(ph)
	if err != nil {
		return fail(err)
	}
	if err := resumeThreads(uint32(cmd.Process.Pid)); err != nil {
		return fail(err)
	}
	return &Contained{cmd, job}, nil
}

// resumeThreads resumes the threads of a process started suspended (its main thread).
func resumeThreads(pid uint32) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)
	var te windows.ThreadEntry32
	te.Size = uint32(unsafe.Sizeof(te))
	resumed := false
	for err = windows.Thread32First(snap, &te); err == nil; err = windows.Thread32Next(snap, &te) {
		if te.OwnerProcessID != pid {
			continue
		}
		th, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, te.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(th)
		windows.CloseHandle(th)
		if err != nil {
			return err
		}
		resumed = true
	}
	if !resumed {
		return windows.ERROR_NOT_FOUND
	}
	return nil
}

// Stop ends every process in the child's job (Windows has no SIGTERM to send a console program from here,
// so this is the forced stop), waiting through done up to grace.
func (c *Contained) Stop(w *Waited, grace time.Duration) {
	windows.TerminateJobObject(c.job, 1)
	select {
	case <-w.Done():
	case <-time.After(grace):
		c.cmd.Process.Kill()
		<-w.Done()
	}
}

// Close releases the job's handle: anything of the child's still running ends.
func (c *Contained) Close() { windows.CloseHandle(c.job) }

// stop ends a child past its deadline with its whole tree (a shell and the program under it): Windows has
// no SIGTERM to send a console program from here, so this is the forced stop.
func stop(cmd *exec.Cmd, done <-chan error, grace time.Duration) {
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	select {
	case <-done:
	case <-time.After(grace):
		cmd.Process.Kill()
		<-done
	}
}

// Alive reports whether a process with this Windows PID is running.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	return windows.GetExitCodeProcess(h, &code) == nil && code == 259 // STILL_ACTIVE
}

// StopPID ends a process this one did not start, with its tree (forced: see stop).
func StopPID(pid int, grace time.Duration) {
	if pid > 0 {
		exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
}
