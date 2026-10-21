//go:build !windows

package util

import (
	"os"
	"syscall"
)

// terminateProcess мягко завершает процесс на Unix.
func terminateProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}
