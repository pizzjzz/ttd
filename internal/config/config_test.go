package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// isolate подготавливает предсказуемое окружение для тестов путей.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("TTD_HOME", t.TempDir())
	t.Setenv("TTD_VENV", "")
	t.Setenv("TTD_DATA_DIR", "")
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", t.TempDir())
	}
}

func TestDataDirDefaults(t *testing.T) {
	isolate(t)
	want := filepath.Join(os.Getenv("HOME"), ".local", "share", "ttd")
	if runtime.GOOS == "windows" {
		want = filepath.Join(os.Getenv("APPDATA"), "ttd")
	}
	if got := DataDir(); got != want {
		t.Fatalf("DataDir = %q, ожидалось %q", got, want)
	}
}

func TestDataDirHonorsTTDDataDir(t *testing.T) {
	isolate(t)
	custom := filepath.Join(t.TempDir(), "my data")
	t.Setenv("TTD_DATA_DIR", custom)
	if got := DataDir(); got != custom {
		t.Fatalf("DataDir = %q, ожидалось %q", got, custom)
	}
	dirs := DataDirs()
	if len(dirs) == 0 || dirs[0] != custom {
		t.Fatalf("DataDirs должен начинаться с TTD_DATA_DIR: %v", dirs)
	}
}

func TestVenvCandidatesIncludeSourceAndData(t *testing.T) {
	isolate(t)
	dataDir := t.TempDir()
	t.Setenv("TTD_DATA_DIR", dataDir)

	candidates := VenvCandidates()
	wantProject := filepath.Join(ProjectRoot(), ".venv")
	wantData := filepath.Join(dataDir, "venv")
	foundProject, foundData := false, false
	for _, candidate := range candidates {
		if candidate == wantProject {
			foundProject = true
		}
		if candidate == wantData {
			foundData = true
		}
	}
	if !foundProject {
		t.Fatalf("нет кандидата рядом с исходниками: %v", candidates)
	}
	if !foundData {
		t.Fatalf("нет кандидата в каталоге данных: %v", candidates)
	}
}

func TestVenvDirPrefersExisting(t *testing.T) {
	isolate(t)
	dataDir := t.TempDir()
	t.Setenv("TTD_DATA_DIR", dataDir)
	venv := filepath.Join(dataDir, "venv")
	if err := os.MkdirAll(venv, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	if got := cfg.VenvDir(); got != venv {
		t.Fatalf("VenvDir = %q, ожидалось %q", got, venv)
	}
}

func TestVenvDirFallsBackToDataDir(t *testing.T) {
	isolate(t)
	dataDir := t.TempDir()
	t.Setenv("TTD_DATA_DIR", dataDir)
	cfg := Default()
	want := filepath.Join(dataDir, "venv")
	if got := cfg.VenvDir(); got != want {
		t.Fatalf("VenvDir = %q, ожидалось %q", got, want)
	}
}

func TestVenvDirHonorsExplicitTTDVenv(t *testing.T) {
	isolate(t)
	explicit := filepath.Join(t.TempDir(), "custom-venv")
	if err := os.MkdirAll(explicit, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TTD_VENV", explicit)
	cfg := Default()
	if got := cfg.VenvDir(); got != explicit {
		t.Fatalf("VenvDir = %q, ожидалось %q", got, explicit)
	}
}
