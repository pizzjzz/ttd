package util

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// RunResult — итог запуска внешнего процесса.
type RunResult struct {
	ExitCode  int
	Cancelled bool
	Err       error
}

// RunStream запускает команду, построчно отдавая объединённый stdout+stderr.
// При отмене контекста процесс завершается (SIGTERM на Unix, Kill на Windows).
func RunStream(ctx context.Context, name string, args []string, env []string, onLine func(string)) RunResult {
	cmd := exec.CommandContext(ctx, name, args...)
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = env
	cmd.Stdin = nil
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return terminateProcess(cmd.Process)
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RunResult{ExitCode: -1, Err: err}
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return RunResult{ExitCode: -1, Err: err}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			if onLine != nil {
				onLine(StripANSI(scanner.Text()))
			}
		}
	}()

	waitErr := cmd.Wait()
	<-done

	result := RunResult{Cancelled: ctx.Err() != nil}
	if waitErr == nil {
		result.ExitCode = 0
		return result
	}
	result.Err = waitErr
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result
}

// Capture запускает команду и возвращает её stdout и stderr.
func Capture(ctx context.Context, name string, args []string, env []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env == nil {
		env = os.Environ()
	}
	cmd.Env = env
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return out.String(), errBuf.String(), err
}

// FindExecutable ищет исполняемый файл в заданных каталогах, затем в PATH.
// На Windows учитывает расширения .exe/.cmd/.bat.
func FindExecutable(names []string, extraDirs []string) string {
	for _, dir := range extraDirs {
		if dir == "" {
			continue
		}
		for _, name := range names {
			for _, candidate := range executableNames(name) {
				path := filepath.Join(dir, candidate)
				info, err := os.Stat(path)
				if err == nil && isExecutable(info) {
					return path
				}
			}
		}
	}
	for _, name := range names {
		for _, candidate := range executableNames(name) {
			if path, err := exec.LookPath(candidate); err == nil {
				return path
			}
		}
	}
	return ""
}

// executableNames добавляет Windows-расширения, если их нет в имени.
func executableNames(name string) []string {
	if runtime.GOOS != "windows" || filepath.Ext(name) != "" {
		return []string{name}
	}
	return []string{name + ".exe", name + ".cmd", name + ".bat", name}
}

func isExecutable(info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	// На Windows бит исполнения не выставляется — достаточно, что это файл.
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
