//go:build !windows

package lib

import "syscall"

func processAlive(n int, _ string) bool { return syscall.Kill(n, 0) == nil }
