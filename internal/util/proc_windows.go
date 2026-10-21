//go:build windows

package util

import "os"

// terminateProcess завершает процесс на Windows: Signal(SIGTERM) там не
// поддерживается, поэтому используем Kill.
func terminateProcess(process *os.Process) error {
	return process.Kill()
}
