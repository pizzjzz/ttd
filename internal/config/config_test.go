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
	t.Setenv("XDG_CONFIG_HOME", "")
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

func TestValidStorageName(t *testing.T) {
	valid := []string{"nas", "my-cloud", "disk_2", "NAS"}
	for _, name := range valid {
		if !ValidStorageName(name) {
			t.Fatalf("%q должен быть допустимым именем", name)
		}
	}
	invalid := []string{"", "local", "library", "temp", "with space", "точка", "a/b"}
	for _, name := range invalid {
		if ValidStorageName(name) {
			t.Fatalf("%q не должен быть допустимым именем", name)
		}
	}
}

func TestStoragePathAndNames(t *testing.T) {
	cfg := Default()
	cfg.MediaRoot = filepath.Join(t.TempDir(), "library")
	cfg.Storages = map[string]string{
		"nas":   "/mnt/nas/ttd",
		"cloud": "/mnt/cloud/ttd",
	}

	if path, ok := cfg.StoragePath("local"); !ok || path != cfg.MediaPath() {
		t.Fatalf("local: получено %q, %v", path, ok)
	}
	if path, ok := cfg.StoragePath("NAS"); !ok || path != "/mnt/nas/ttd" {
		t.Fatalf("NAS: получено %q, %v", path, ok)
	}
	if _, ok := cfg.StoragePath("missing"); ok {
		t.Fatal("неизвестный профиль не должен находиться")
	}
	names := cfg.StorageNames()
	if len(names) != 2 || names[0] != "cloud" || names[1] != "nas" {
		t.Fatalf("имена профилей: %v", names)
	}
	if !Default().AskDestination {
		t.Fatal("по умолчанию ttd должен спрашивать, куда сохранить")
	}
}

func TestNormalizeStorages(t *testing.T) {
	cfg := Default()
	cfg.Storages = map[string]string{
		" nas ": "/mnt/nas/ttd",
		"":      "/tmp",
		"empty": " ",
	}
	cfg.normalize()
	if len(cfg.Storages) != 1 {
		t.Fatalf("ожидался один профиль, получено %v", cfg.Storages)
	}
	if cfg.Storages["nas"] != "/mnt/nas/ttd" {
		t.Fatalf("профиль nas: %q", cfg.Storages["nas"])
	}
}

func TestUpdateMediaRootKeepsFileSettings(t *testing.T) {
	isolate(t)
	cfg := Default()
	cfg.MediaRoot = filepath.Join(t.TempDir(), "file-lib")
	cfg.Engine = "gallery-dl"
	cfg.Storages = map[string]string{"nas": "/mnt/nas/ttd"}
	if _, err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	envDir := filepath.Join(t.TempDir(), "env-lib")
	t.Setenv("TTD_DIR", envDir)
	t.Setenv("TTD_ENGINE", "yt-dlp")
	t.Setenv("TTD_ASK_DESTINATION", "false")

	newRoot := filepath.Join(t.TempDir(), "new-lib")
	if _, err := UpdateMediaRoot(newRoot); err != nil {
		t.Fatal(err)
	}
	fileCfg := LoadFile()
	if fileCfg.MediaRoot != newRoot {
		t.Fatalf("media_root в файле: %q, ожидалось %q", fileCfg.MediaRoot, newRoot)
	}
	if fileCfg.Engine != "gallery-dl" {
		t.Fatalf("движок из файла затёрт: %q", fileCfg.Engine)
	}
	if !fileCfg.AskDestination {
		t.Fatal("значение из TTD_ASK_DESTINATION не должно попадать в файл")
	}
	if fileCfg.Storages["nas"] != "/mnt/nas/ttd" {
		t.Fatalf("профиль потерян: %v", fileCfg.Storages)
	}
	if effective := Load(); effective.MediaRoot != envDir || effective.Engine != "yt-dlp" {
		t.Fatalf("Load не применил env: %+v", effective)
	}
}
