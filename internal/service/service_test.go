package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ttd/internal/config"
	"ttd/internal/library"
	"ttd/internal/model"
	"ttd/internal/util"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.MediaRoot = t.TempDir()
	cfg.Engine = "mock"
	cfg.UseArchive = false
	return cfg
}

func TestRunWithMockCreatesDateFolders(t *testing.T) {
	cfg := testConfig(t)
	svc := New(cfg, nil)

	results := svc.Run(context.Background(), []string{
		"https://www.tiktok.com/@user/video/111",
		"https://www.tiktok.com/@user/photo/222",
	}, model.DownloadOptions{Engine: "mock"})

	if len(results) != 2 {
		t.Fatalf("ожидалось 2 результата, получено %d", len(results))
	}
	total := 0
	for _, result := range results {
		if result.Status != model.StatusOK {
			t.Fatalf("статус %q, ошибка %q", result.Status, result.Error)
		}
		total += len(result.Files)
	}
	if total != 4 {
		t.Fatalf("ожидалось 4 файла (1 видео + 3 фото), получено %d", total)
	}

	folders := library.ListFolders(cfg.MediaPath())
	if len(folders) != 1 {
		t.Fatalf("ожидалась 1 папка, получено %d: %+v", len(folders), folders)
	}
	// Без метаданных дата публикации совпадает с датой скачивания.
	want := util.Today() + library.Separator + util.Today()
	if folders[0].Name != want {
		t.Fatalf("имя папки %q, ожидалось %q", folders[0].Name, want)
	}

	// Промежуточный каталог должен быть убран.
	if _, err := os.Stat(filepath.Join(cfg.MediaPath(), StagingDirName)); err == nil {
		t.Fatal("каталог .staging должен быть удалён")
	}
}

func TestPlaceSplitsByPublishDate(t *testing.T) {
	cfg := testConfig(t)
	svc := New(cfg, nil)

	staging := filepath.Join(cfg.MediaPath(), StagingDirName, "job-1")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMedia := func(name, uploadDate string) {
		path := filepath.Join(staging, name)
		if err := os.WriteFile(path, []byte("media"), 0o644); err != nil {
			t.Fatal(err)
		}
		sidecar := path + ".info.json"
		if err := os.WriteFile(sidecar, []byte(`{"upload_date":"`+uploadDate+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMedia("recent.mp4", "20261017")
	writeMedia("old.mp4", "20251001")

	items, err := svc.place(model.DownloadOptions{MediaRoot: cfg.MediaPath(), Date: "19.10.2026"}, staging)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("ожидалось 2 разложенных файла, получено %d", len(items))
	}

	folders := library.ListFolders(cfg.MediaPath())
	if len(folders) != 2 {
		t.Fatalf("ожидалось 2 папки (разные даты публикации), получено %d: %+v", len(folders), folders)
	}
	found := map[string]bool{}
	for _, folder := range folders {
		found[folder.Name] = true
	}
	for _, want := range []string{
		library.FolderName("19.10.2026", "17.10.2026"),
		library.FolderName("19.10.2026", "01.10.2025"),
	} {
		if !found[want] {
			t.Fatalf("нет папки %q, есть: %v", want, found)
		}
	}
}

func TestPlaceKeepsFilenameOnCollision(t *testing.T) {
	cfg := testConfig(t)
	svc := New(cfg, nil)
	opts := model.DownloadOptions{MediaRoot: cfg.MediaPath(), Date: "19.10.2026"}

	staging := filepath.Join(cfg.MediaPath(), StagingDirName, "job-a")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "clip.mp4"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.place(opts, staging); err != nil {
		t.Fatal(err)
	}

	staging2 := filepath.Join(cfg.MediaPath(), StagingDirName, "job-b")
	if err := os.MkdirAll(staging2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging2, "clip.mp4"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := svc.place(opts, staging2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "clip_1.mp4" {
		t.Fatalf("ожидалось clip_1.mp4, получено %+v", items)
	}
}

func TestFormatDone(t *testing.T) {
	results := []model.JobResult{
		{Status: model.StatusOK, Files: []model.MediaItem{{Size: 1024}}},
		{Status: model.StatusSkipped},
		{Status: model.StatusError},
	}
	text := FormatDone(results)
	for _, want := range []string{"1 ок", "1 пропущено", "1 с ошибкой", "объём: 1.0 KiB"} {
		if !contains(text, want) {
			t.Fatalf("в строке %q нет %q", text, want)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func TestPlaceFlat(t *testing.T) {
	cfg := testConfig(t)
	svc := New(cfg, nil)
	staging := filepath.Join(cfg.MediaPath(), StagingDirName, "job-flat")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "clip.mp4"), []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := svc.place(model.DownloadOptions{
		MediaRoot: cfg.MediaPath(), Date: "19.10.2026", Flat: true,
	}, staging)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("ожидался 1 файл, получено %d", len(items))
	}
	if items[0].Path != filepath.Join(cfg.MediaPath(), "clip.mp4") {
		t.Fatalf("файл разложен не туда: %s", items[0].Path)
	}
	if folders := library.ListFolders(cfg.MediaPath()); len(folders) != 0 {
		t.Fatalf("в разовой папке не должно быть папок-дат: %+v", folders)
	}
}

func TestRouteResultsMoveLibrary(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	folder := "19.10.2026 | 17.10.2026"
	if err := os.MkdirAll(filepath.Join(source, folder), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, folder, "clip.mp4")
	if err := os.WriteFile(file, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := []model.JobResult{{Status: model.StatusOK, Files: []model.MediaItem{{Path: file, Name: "clip.mp4"}}}}

	count, err := RouteResults(results, source, target, false, true)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	moved := filepath.Join(target, folder, "clip.mp4")
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("файл не перенесён: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("исходный файл остался: %v", err)
	}
	if results[0].Files[0].Path != moved {
		t.Fatalf("путь результата не обновлён: %s", results[0].Files[0].Path)
	}
	if _, err := os.Stat(filepath.Join(source, folder)); !os.IsNotExist(err) {
		t.Fatalf("опустевшая папка-дата должна быть удалена: %v", err)
	}
}

func TestRouteResultsKeepsParentOutsideLibrary(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	outside := t.TempDir()
	file := filepath.Join(outside, "clip.mp4")
	if err := os.WriteFile(file, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := []model.JobResult{{Status: model.StatusOK, Files: []model.MediaItem{{Path: file, Name: "clip.mp4"}}}}

	count, err := RouteResults(results, source, target, false, true)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("каталог за пределами библиотеки удалён: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "clip.mp4")); err != nil {
		t.Fatalf("файл не перенесён: %v", err)
	}
}

func TestRouteResultsCopyFlat(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	file := filepath.Join(source, "clip.mp4")
	if err := os.WriteFile(file, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := []model.JobResult{{Status: model.StatusOK, Files: []model.MediaItem{{Path: file, Name: "clip.mp4"}}}}

	count, err := RouteResults(results, source, target, true, false)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if _, err := os.Stat(filepath.Join(target, "clip.mp4")); err != nil {
		t.Fatalf("файл не скопирован: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("исходный файл должен остаться: %v", err)
	}
	if _, err := RouteResults(results, source, source, true, false); err == nil {
		t.Fatal("перенос в тот же каталог должен быть отклонён")
	}
}
