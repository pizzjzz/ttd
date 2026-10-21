package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledWheelsPresent(t *testing.T) {
	versions := BundledVersions()
	if versions["gallery-dl"] == "" {
		t.Fatal("во встроенном наборе нет gallery-dl")
	}
	if versions["yt-dlp"] == "" {
		t.Fatal("во встроенном наборе нет yt-dlp")
	}
	names := BundledWheelNames()
	if len(names) < 2 {
		t.Fatalf("ожидались хотя бы два колеса, получено %d", len(names))
	}
}

func TestBundledWheelsArePurePython(t *testing.T) {
	for _, name := range BundledWheelNames() {
		if !strings.HasSuffix(name, "-py3-none-any.whl") {
			t.Fatalf("колесо %q не является чистым Python-пакетом", name)
		}
	}
}

func TestExtractEmbeddedWheels(t *testing.T) {
	dir := t.TempDir()
	count, err := extractEmbeddedWheels(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(BundledWheelNames()) {
		t.Fatalf("распаковано %d из %d колёс", count, len(BundledWheelNames()))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("в каталоге %d файлов, ожидалось %d", len(entries), count)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Fatalf("файл %s пустой", entry.Name())
		}
	}
}

func TestPipInstallArgsOffline(t *testing.T) {
	args, cleanup, err := pipInstallArgs(Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-index") || !strings.Contains(joined, "--find-links") {
		t.Fatalf("офлайн-установка должна использовать --no-index: %v", args)
	}
	if strings.Contains(joined, "--upgrade") {
		t.Fatalf("офлайн-установка не должна обновлять пакеты: %v", args)
	}
}

func TestPipInstallArgsOnline(t *testing.T) {
	args, _, err := pipInstallArgs(Options{Online: true})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--upgrade") {
		t.Fatalf("онлайн-установка должна обновлять пакеты: %v", args)
	}
	if strings.Contains(joined, "--no-index") {
		t.Fatalf("онлайн-установка не должна отключать индекс: %v", args)
	}
}

func TestPipInstallArgsWheelsDir(t *testing.T) {
	dir := t.TempDir()
	args, _, err := pipInstallArgs(Options{WheelsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--no-index") || !strings.Contains(joined, dir) {
		t.Fatalf("ожидался локальный каталог колёс: %v", args)
	}
	if _, _, err := pipInstallArgs(Options{WheelsDir: filepath.Join(dir, "нет")}); err == nil {
		t.Fatal("несуществующий каталог должен вызывать ошибку")
	}
}

func TestPythonVersionOK(t *testing.T) {
	cases := map[string]bool{
		"3.14.8": true,
		"3.10":   true,
		"3.10.0": true,
		"3.11":   true,
		"4.0":    true,
		"3.9":    false,
		"3.9.18": false,
		"2.7":    false,
		"":       false,
		"abc":    false,
	}
	for version, want := range cases {
		if got := pythonVersionOK(version); got != want {
			t.Fatalf("pythonVersionOK(%q) = %v, ожидалось %v", version, got, want)
		}
	}
}

func TestWheelNameParts(t *testing.T) {
	dist, version := wheelNameParts("gallery_dl-1.32.17-py3-none-any.whl")
	if dist != "gallery-dl" || version != "1.32.17" {
		t.Fatalf("получено %q %q", dist, version)
	}
	dist, version = wheelNameParts("yt_dlp-2026.8.19-py3-none-any.whl")
	if dist != "yt-dlp" || version != "2026.8.19" {
		t.Fatalf("получено %q %q", dist, version)
	}
}
