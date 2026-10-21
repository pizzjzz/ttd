package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ttd/internal/config"
	"ttd/internal/library"
	"ttd/internal/model"
)

func testApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Default()
	cfg.MediaRoot = t.TempDir()
	cfg.Engine = "mock"
	return New(cfg)
}

func TestMetaEndpoint(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(app.Routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/api/meta")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("статус %d", response.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["media_root"] != app.cfg.MediaPath() {
		t.Fatalf("media_root = %v", payload["media_root"])
	}
}

func TestIndexServesHTML(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(app.Routes())
	defer server.Close()

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := make([]byte, 512)
	n, _ := response.Body.Read(body)
	if !strings.Contains(string(body[:n]), "<html") {
		t.Fatalf("не HTML: %q", string(body[:n]))
	}
}

func TestFileEndpointRangeAndTraversal(t *testing.T) {
	app := testApp(t)
	folder := library.FolderName("19.10.2026", "17.10.2026")
	dir := filepath.Join(app.cfg.MediaPath(), folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("0123456789")
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(app.Routes())
	defer server.Close()

	url := server.URL + "/api/files?folder=" + urlEnc(folder) + "&name=clip.mp4"
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("статус %d", response.StatusCode)
	}

	// Range-запрос должен поддерживаться (важно для проигрывания видео).
	request, _ := http.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("Range", "bytes=0-3")
	partial, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer partial.Body.Close()
	if partial.StatusCode != http.StatusPartialContent {
		t.Fatalf("ожидался 206, получено %d", partial.StatusCode)
	}

	// Выход за пределы библиотеки запрещён.
	traversal := server.URL + "/api/files?folder=" + urlEnc("../..") + "&name=etc/passwd"
	bad, err := http.Get(traversal)
	if err != nil {
		t.Fatal(err)
	}
	defer bad.Body.Close()
	if bad.StatusCode == http.StatusOK {
		t.Fatal("обход каталога не должен работать")
	}
}

func TestDownloadJobWithMockEngine(t *testing.T) {
	app := testApp(t)
	server := httptest.NewServer(app.Routes())
	defer server.Close()

	payload := `{"urls":["https://www.tiktok.com/@user/video/111"],"engine":"mock"}`
	response, err := http.Post(server.URL+"/api/download", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	var started map[string]any
	if err := json.NewDecoder(response.Body).Decode(&started); err != nil {
		t.Fatal(err)
	}
	id, _ := started["id"].(string)
	if id == "" {
		t.Fatalf("нет идентификатора задачи: %v", started)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		jobResponse, err := http.Get(server.URL + "/api/job/" + id + "?since=0")
		if err != nil {
			t.Fatal(err)
		}
		var status struct {
			Status  string            `json:"status"`
			Results []model.JobResult `json:"results"`
		}
		err = json.NewDecoder(jobResponse.Body).Decode(&status)
		jobResponse.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if status.Status == "done" {
			if len(status.Results) != 1 || status.Results[0].Status != model.StatusOK {
				t.Fatalf("неожиданный результат: %+v", status.Results)
			}
			folders := library.ListFolders(app.cfg.MediaPath())
			if len(folders) != 1 {
				t.Fatalf("ожидалась 1 папка, получено %d", len(folders))
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("задача не завершилась за отведённое время")
}

func urlEnc(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, " ", "%20"), "|", "%7C")
}

func TestRouteEndpointMovesToStorage(t *testing.T) {
	app := testApp(t)
	storage := t.TempDir()
	app.cfg.Storages = map[string]string{"nas": storage}

	folder := library.FolderName("19.10.2026", "17.10.2026")
	dir := filepath.Join(app.cfg.MediaPath(), folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(source, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}

	job := app.newJob([]string{"https://www.tiktok.com/@user/video/1"})
	job.MediaRoot = app.cfg.MediaPath()
	job.status = "done"
	job.results = []model.JobResult{{
		Status: model.StatusOK,
		Files:  []model.MediaItem{{Path: source, Name: "clip.mp4"}},
	}}

	body := `{"id":"` + job.ID + `","storage":"nas","move":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/route", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(storage, folder, "clip.mp4")); err != nil {
		t.Fatalf("файл не перенесён в профиль: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("исходный файл остался: %v", err)
	}
}

func TestRouteEndpointPermanent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := testApp(t)

	source := filepath.Join(app.cfg.MediaPath(), library.FolderName("19.10.2026", "19.10.2026"))
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "clip.mp4")
	if err := os.WriteFile(file, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	job := app.newJob([]string{"https://www.tiktok.com/@user/video/1"})
	job.MediaRoot = app.cfg.MediaPath()
	job.status = "done"
	job.results = []model.JobResult{{
		Status: model.StatusOK,
		Files:  []model.MediaItem{{Path: file, Name: "clip.mp4"}},
	}}

	target := filepath.Join(t.TempDir(), "new-library")
	body := `{"id":"` + job.ID + `","target":"` + target + `","permanent":true,"move":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/route", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d: %s", recorder.Code, recorder.Body.String())
	}
	if app.cfg.MediaRoot != target {
		t.Fatalf("media_root = %q, ожидался %q", app.cfg.MediaRoot, target)
	}
	if saved := config.LoadFile(); saved.MediaRoot != target {
		t.Fatalf("media_root в файле = %q, ожидался %q", saved.MediaRoot, target)
	}
	if _, err := os.Stat(filepath.Join(target, library.FolderName("19.10.2026", "19.10.2026"), "clip.mp4")); err != nil {
		t.Fatalf("файл не перенесён: %v", err)
	}
}

func TestRouteEndpointPermanentFailureKeepsConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := testApp(t)
	original := app.cfg.MediaRoot

	folder := library.FolderName("19.10.2026", "19.10.2026")
	if err := os.MkdirAll(filepath.Join(original, folder), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(original, folder, "missing.mp4")

	job := app.newJob([]string{"https://www.tiktok.com/@user/video/1"})
	job.MediaRoot = original
	job.status = "done"
	job.results = []model.JobResult{{
		Status: model.StatusOK,
		Files:  []model.MediaItem{{Path: missing, Name: "missing.mp4"}},
	}}

	target := filepath.Join(t.TempDir(), "new-library")
	body := `{"id":"` + job.ID + `","target":"` + target + `","permanent":true,"move":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/route", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	app.Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("статус %d: %s", recorder.Code, recorder.Body.String())
	}
	if app.cfg.MediaRoot != original {
		t.Fatalf("media_root изменён при ошибке: %q", app.cfg.MediaRoot)
	}
	if saved := config.LoadFile(); saved.MediaRoot == target {
		t.Fatal("ошибочный перенос не должен сохраняться в настройках")
	}
}
