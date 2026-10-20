package util

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTodayMatchesLayout(t *testing.T) {
	today := Today()
	parsed, ok := ParseDate(today)
	if !ok {
		t.Fatalf("Today()=%q не разбирается как дата", today)
	}
	if parsed.Format(DateLayout) != today {
		t.Fatalf("дата не совпала: %q", today)
	}
	if parsed.Day() != time.Now().Day() {
		t.Fatalf("не сегодняшний день: %v", parsed)
	}
}

func TestParseDateRejectsGarbage(t *testing.T) {
	for _, value := range []string{"", "2006-10-19", "19/10/2026", "абв"} {
		if _, ok := ParseDate(value); ok {
			t.Fatalf("ожидалась ошибка для %q", value)
		}
	}
}

func TestFormatDate(t *testing.T) {
	when := time.Date(2026, 10, 19, 15, 4, 5, 0, time.Local)
	if got := FormatDate(when); got != "19.10.2026" {
		t.Fatalf("FormatDate = %q", got)
	}
}

func TestIsDateName(t *testing.T) {
	if !IsDateName("19.10.2026") {
		t.Fatal("19.10.2026 должна быть датой")
	}
	for _, value := range []string{"19.10.2026 | 17.10.2026", "2026-10-19", ".staging", "abc"} {
		if IsDateName(value) {
			t.Fatalf("%q не должна считаться простой датой", value)
		}
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		0:       "0 B",
		512:     "512 B",
		1024:    "1.0 KiB",
		1536:    "1.5 KiB",
		1048576: "1.0 MiB",
	}
	for input, want := range cases {
		if got := HumanSize(input); got != want {
			t.Errorf("HumanSize(%d) = %q, ожидалось %q", input, got, want)
		}
	}
}

func TestHumanETA(t *testing.T) {
	if got := HumanETA(65); got != "01:05" {
		t.Errorf("HumanETA(65) = %q", got)
	}
	if got := HumanETA(3661); got != "1:01:01" {
		t.Errorf("HumanETA(3661) = %q", got)
	}
	if got := HumanETA(-1); got != "" {
		t.Errorf("HumanETA(-1) = %q", got)
	}
}

func TestIsTempFile(t *testing.T) {
	temp := []string{".hidden", "video.mp4.part", "video.mp4.ytdl", "clip.f137.mp4", "x~"}
	for _, name := range temp {
		if !IsTempFile(name) {
			t.Errorf("%q должен считаться временным", name)
		}
	}
	if IsTempFile("user_123.mp4") {
		t.Error("обычный файл не должен быть временным")
	}
}

func TestSafeJoinBlocksTraversal(t *testing.T) {
	root := t.TempDir()
	if _, ok := SafeJoin(root, "..", "etc", "passwd"); ok {
		t.Fatal("выход за пределы корня должен блокироваться")
	}
	joined, ok := SafeJoin(root, "19.10.2026 | 17.10.2026", "a.mp4")
	if !ok {
		t.Fatal("обычный путь должен разрешаться")
	}
	if filepath.Dir(joined) != filepath.Join(root, "19.10.2026 | 17.10.2026") {
		t.Fatalf("неверный путь: %s", joined)
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	name := "clip.mp4"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := UniquePath(dir, name)
	if filepath.Base(got) != "clip_1.mp4" {
		t.Fatalf("UniquePath = %s", got)
	}
}

func TestStripANSI(t *testing.T) {
	if got := StripANSI("\x1b[31mкрасный\x1b[0m"); got != "красный" {
		t.Fatalf("StripANSI = %q", got)
	}
}

func TestSnapshotAndNewFiles(t *testing.T) {
	dir := t.TempDir()
	snapshot := TakeSnapshot(dir)
	if err := os.WriteFile(filepath.Join(dir, "new.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := NewFiles(snapshot, dir)
	if len(names) != 1 || names[0] != "new.mp4" {
		t.Fatalf("NewFiles = %v", names)
	}
}
