//go:build darwin

package tui

import "syscall"

// macOS-имена ioctl для терминала (BSD-вариант).
const (
	termiosGet = syscall.TIOCGETA
	termiosSet = syscall.TIOCSETA
)
