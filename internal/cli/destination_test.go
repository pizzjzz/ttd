package cli

import (
	"path/filepath"
	"testing"

	"ttd/internal/config"
)

func TestLooksLikePath(t *testing.T) {
	paths := []string{"/mnt/nas/ttd", "./ttd", "~/ttd", `C:\ttd`, `\\server\share`}
	for _, value := range paths {
		if !looksLikePath(value) {
			t.Fatalf("%q должен считаться путём", value)
		}
	}
	for _, value := range []string{"nas", "cloud", "my-disk"} {
		if looksLikePath(value) {
			t.Fatalf("%q не должен считаться путём", value)
		}
	}
}

func TestResolveDestination(t *testing.T) {
	cfg := config.Default()
	cfg.MediaRoot = filepath.Join(t.TempDir(), "library")
	cfg.Storages = map[string]string{"nas": "/mnt/nas/ttd"}

	path, named, err := resolveDestination(&cfg, "nas")
	if err != nil || !named || path != "/mnt/nas/ttd" {
		t.Fatalf("nas: path=%q named=%v err=%v", path, named, err)
	}
	path, named, err = resolveDestination(&cfg, "local")
	if err != nil || named || path != cfg.MediaPath() {
		t.Fatalf("local: path=%q named=%v err=%v", path, named, err)
	}
	path, named, err = resolveDestination(&cfg, "/mnt/custom")
	if err != nil || named || path != "/mnt/custom" {
		t.Fatalf("путь: path=%q named=%v err=%v", path, named, err)
	}
	if _, _, err := resolveDestination(&cfg, "missing"); err == nil {
		t.Fatal("неизвестный профиль должен давать ошибку")
	}
}
