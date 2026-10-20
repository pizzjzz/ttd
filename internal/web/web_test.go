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
