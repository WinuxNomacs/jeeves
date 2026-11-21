package auto

import "time"

// commandLine: not read on Windows (a run's child is in a job object that ended with its supervisor).
func commandLine(int) (string, bool) { return "", false }

func stopGroup(int, time.Duration) {}

// needsWatchdog: on Windows the job object ends the child with its supervisor.
const needsWatchdog = false

// ownedBy: Windows folders under the user's profile are the user's.
func ownedBy(interface{ Sys() any }) bool { return true }

// umask: Windows has none (files under the user's profile are the user's).
func umask() {}
