package library

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFolderName(t *testing.T) {
	if got := FolderName("19.10.2026", "17.10.2026"); got != "19.10.2026"+Separator+"17.10.2026" {
		t.Fatalf("FolderName = %q", got)
	}
	// Без даты публикации показываем обе одинаковыми.
	if got := FolderName("19.10.2026", ""); got != "19.10.2026"+Separator+"19.10.2026" {
		t.Fatalf("FolderName без публикации = %q", got)
	}
	// В имени папки не должно быть символов, запрещённых в текущей ОС.
	if runtime.GOOS == "windows" && strings.Contains(FolderName("19.10.2026", "17.10.2026"), "|") {
		t.Fatal("на Windows символ | недопустим в имени папки")
	}
}

func TestParseFolderName(t *testing.T) {
	// Понимаем оба разделителя независимо от платформы.
	for _, name := range []string{
		"19.10.2026 | 17.10.2026",
		"19.10.2026 - 17.10.2026",
	} {
		download, publish, ok := ParseFolderName(name)
		if !ok || download != "19.10.2026" || publish != "17.10.2026" {
			t.Fatalf("разбор %q не удался: %q %q %v", name, download, publish, ok)
		}
	}
	// Поддерживаем и старый формат с одной датой.
	download, publish, ok := ParseFolderName("19.10.2026")
	if !ok || download != "19.10.2026" || publish != "19.10.2026" {
		t.Fatalf("разбор одиночной даты не удался: %q %q %v", download, publish, ok)
	}
	if _, _, ok := ParseFolderName("не дата"); ok {
		t.Fatal("обычное имя не должно разбираться")
	}
	made := FolderName("01.02.2026", "03.04.2025")
	if download, publish, ok := ParseFolderName(made); !ok || download != "01.02.2026" || publish != "03.04.2025" {
		t.Fatalf("собственное имя не разбирается: %q", made)
	}
}

func TestListFoldersSortingAndCounts(t *testing.T) {
	root := t.TempDir()
	write := func(folder, name string, size int) {
		dir := filepath.Join(root, folder)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(FolderName("19.10.2026", "17.10.2026"), "a.mp4", 100)
	write(FolderName("19.10.2026", "17.10.2026"), "b.jpg", 50)
	write(FolderName("20.10.2026", "18.10.2026"), "c.mp4", 10)
	// Служебный каталог не должен попадать в список.
	if err := os.MkdirAll(filepath.Join(root, ".staging"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ttd-cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	folders := ListFolders(root)
	if len(folders) != 2 {
		t.Fatalf("ожидалось 2 папки, получено %d: %+v", len(folders), folders)
	}
	// Новые сверху.
	if folders[0].DownloadDate != "20.10.2026" {
		t.Fatalf("первой должна быть свежая папка: %+v", folders[0])
	}
	second := folders[1]
	if second.PublishDate != "17.10.2026" {
		t.Fatalf("дата публикации не разобрана: %+v", second)
	}
	if second.Count != 2 || second.Videos != 1 || second.Photos != 1 {
		t.Fatalf("неверные счётчики: %+v", second)
	}
	if second.Size != 150 {
		t.Fatalf("неверный размер: %d", second.Size)
	}
}

func TestIterMediaAcrossFolders(t *testing.T) {
	root := t.TempDir()
	for _, folder := range []string{FolderName("19.10.2026", "17.10.2026"), FolderName("18.10.2026", "18.10.2026")} {
		dir := filepath.Join(root, folder)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "clip.mp4"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	items := IterMedia(root, "")
	if len(items) != 2 {
		t.Fatalf("ожидалось 2 файла, получено %d", len(items))
	}
	one := IterMedia(root, FolderName("18.10.2026", "18.10.2026"))
	if len(one) != 1 {
		t.Fatalf("ожидался 1 файл в папке, получено %d", len(one))
	}
}
