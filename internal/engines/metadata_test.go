package engines

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSidecar(t *testing.T, mediaPath, name, body string) {
	t.Helper()
	if err := os.WriteFile(mediaPath, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(mediaPath), name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadPublishDateUploadDate(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "user_123.mp4")
	writeSidecar(t, media, "user_123.info.json", `{"upload_date":"20261017","id":"123"}`)

	when, ok := ReadPublishDate(media)
	if !ok {
		t.Fatal("дата не найдена")
	}
	if got := when.Format("02.01.2006"); got != "17.10.2026" {
		t.Fatalf("дата = %s", got)
	}
}

func TestReadPublishDateGalleryDLDate(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "user_456.jpg")
	writeSidecar(t, media, "user_456.jpg.json", `{"date":"2026-10-15 12:30:00","id":"456"}`)

	when, ok := ReadPublishDate(media)
	if !ok {
		t.Fatal("дата не найдена")
	}
	if got := when.Format("02.01.2006"); got != "15.10.2026" {
		t.Fatalf("дата = %s", got)
	}
}

func TestReadPublishDateUnixSeconds(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "clip.mp4")
	writeSidecar(t, media, "clip.mp4.json", `{"timestamp":1791504000}`)

	when, ok := ReadPublishDate(media)
	if !ok {
		t.Fatal("дата не найдена")
	}
	// Проверяем независимо от часового пояса.
	if when.Unix() != 1791504000 {
		t.Fatalf("unix = %d", when.Unix())
	}
}

func TestReadPublishDateMillis(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "photo.jpg")
	writeSidecar(t, media, "photo.jpg.json", `{"createTime":1791504000000}`)

	when, ok := ReadPublishDate(media)
	if !ok {
		t.Fatal("дата не найдена")
	}
	if when.Unix() != 1791504000 {
		t.Fatalf("unix = %d", when.Unix())
	}
}

func TestReadPublishDateMissing(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "no_meta.mp4")
	if err := os.WriteFile(media, []byte("media"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadPublishDate(media); ok {
		t.Fatal("без метаданных дата не должна находиться")
	}
}

func TestFindSidecarPrefersInfoJSON(t *testing.T) {
	dir := t.TempDir()
	media := filepath.Join(dir, "clip.mp4")
	writeSidecar(t, media, "clip.mp4.info.json", `{}`)
	if err := os.WriteFile(filepath.Join(dir, "clip.mp4.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindSidecar(media); got != media+".info.json" {
		t.Fatalf("FindSidecar = %q", got)
	}
}
