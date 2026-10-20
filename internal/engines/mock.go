package engines

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ttd/internal/model"
)

// Mock — офлайн-движок для демонстрации и тестов.
// Он ничего не скачивает, а создаёт заглушки в целевом каталоге.
type Mock struct{}

// NewMock создаёт mock-движок.
func NewMock() Engine { return Mock{} }

func (Mock) Name() string    { return "mock" }
func (Mock) Available() bool { return true }
func (Mock) Version() string { return "mock-1" }

func (m Mock) Download(ctx context.Context, url string, opts model.DownloadOptions, dest string, onEvent func(model.Event)) Outcome {
	emit := EventFunc(onEvent)
	if opts.DryRun {
		emit(model.Event{"type": "log", "engine": "mock", "line": "[dry-run] mock " + url})
		return Outcome{OK: true}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Outcome{Error: err.Error()}
	}

	select {
	case <-ctx.Done():
		return Outcome{Cancelled: true, Error: "отменено"}
	default:
	}

	emit(model.Event{"type": "log", "engine": "mock", "line": "mock: имитация загрузки " + url})

	kind := Classify(url)
	exts := []string{".mp4"}
	switch {
	case strings.Contains(strings.ToLower(url), "photo"):
		exts = []string{".jpg", ".jpg", ".jpg"}
	case strings.Contains(strings.ToLower(url), "stories"):
		exts = []string{".jpg", ".mp4"}
	}

	digest := sha1.Sum([]byte(url))
	stub := hex.EncodeToString(digest[:])[:10]
	publish := os.Getenv("TTD_MOCK_PUBLISH")

	files := make([]model.MediaItem, 0, len(exts))
	for i, ext := range exts {
		name := fmt.Sprintf("mock_%s_%02d%s", stub, i+1, ext)
		path := filepath.Join(dest, name)
		body := fmt.Sprintf("mock media for %s\nkind=%s\nindex=%d\n", url, kind, i+1)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return Outcome{Error: err.Error()}
		}
		item := mediaItem(path, url)
		files = append(files, item)
		emit(model.Event{"type": "progress", "engine": "mock", "url": url, "percent": float64(i+1) / float64(len(exts)) * 100})
		emit(model.Event{"type": "file", "engine": "mock", "url": url, "file": item})

		if publish != "" {
			if err := writeStubSidecar(path, publish); err != nil {
				return Outcome{Error: err.Error()}
			}
		}
	}
	return Outcome{OK: true, Files: files}
}

func writeStubSidecar(mediaPath, uploadDate string) error {
	payload := map[string]any{"upload_date": uploadDate, "mock": true}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mediaPath+".info.json", data, 0o644)
}
