//go:build linux

package tui

import "syscall"

// Linux-имена ioctl для терминала.
const (
	termiosGet = syscall.TCGETS
	termiosSet = syscall.TCSETS
)
