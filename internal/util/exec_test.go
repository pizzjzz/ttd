package util

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunStreamCapturesOutputAndExitCode(t *testing.T) {
	var lines []string
	result := RunStream(context.Background(), "sh",
		[]string{"-c", "echo out; echo err 1>&2; exit 3"},
		nil, func(line string) { lines = append(lines, line) })

	if result.ExitCode != 3 {
		t.Fatalf("код выхода = %d, ошибка = %v", result.ExitCode, result.Err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "out") || !strings.Contains(joined, "err") {
		t.Fatalf("не пойманы строки вывода: %q", joined)
	}
}

func TestRunStreamLongRunningOutput(t *testing.T) {
	start := time.Now()
	var lines []string
	result := RunStream(context.Background(), "sh",
		[]string{"-c", "echo first; sleep 0.4; echo second; exit 0"},
		nil, func(line string) { lines = append(lines, line) })

	if result.ExitCode != 0 {
		t.Fatalf("код выхода = %d, ошибка = %v", result.ExitCode, result.Err)
	}
	if len(lines) != 2 {
		t.Fatalf("строки: %q", lines)
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Fatalf("процесс завершился слишком быстро: %v", elapsed)
	}
}

func TestRunStreamCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	result := RunStream(ctx, "sh", []string{"-c", "sleep 5"}, nil, nil)
	if !result.Cancelled {
		t.Fatalf("ожидалась отмена, получено %+v", result)
	}
}

func TestCaptureOutput(t *testing.T) {
	out, _, err := Capture(context.Background(), "sh", []string{"-c", "printf hello"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("out = %q", out)
	}
}

func TestFindExecutableMissing(t *testing.T) {
	if got := FindExecutable([]string{"ttd-definitely-missing-binary"}, nil); got != "" {
		t.Fatalf("найден несуществующий файл: %q", got)
	}
}
