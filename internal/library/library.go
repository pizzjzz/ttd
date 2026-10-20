// Package library работает с библиотекой скачанного: папками-датами и файлами.
package library

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"ttd/internal/model"
	"ttd/internal/util"
)

// Separator — разделитель между датой скачивания и датой публикации.
const Separator = " | "

var pairRe = regexp.MustCompile(`^(\d{2}\.\d{2}\.\d{4}) \| (\d{2}\.\d{2}\.\d{4})$`)

// FolderName собирает имя папки вида "19.10.2026 | 17.10.2026".
func FolderName(download, publish string) string {
	if publish == "" || publish == download {
		// всё равно показываем обе даты, как просили в формате
		publish = download
	}
	return download + Separator + publish
}

// ParseFolderName разбирает имя папки. Поддерживает и старый вид (одна дата).
func ParseFolderName(name string) (download, publish string, ok bool) {
	if m := pairRe.FindStringSubmatch(name); m != nil {
		return m[1], m[2], true
	}
	if util.IsDateName(name) {
		return name, name, true
	}
	return "", "", false
}

// IsFolderName сообщает, является ли имя папки папкой-датой.
func IsFolderName(name string) bool {
	_, _, ok := ParseFolderName(name)
	return ok
}

// ListFolders возвращает все папки библиотеки, новые сверху.
func ListFolders(root string) []model.Folder {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	folders := make([]model.Folder, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		download, publish, ok := ParseFolderName(entry.Name())
		if !ok {
			continue
		}
		folder := readFolder(root, entry.Name(), download, publish)
		folders = append(folders, folder)
	}
	sort.SliceStable(folders, func(i, j int) bool {
		di, _ := util.ParseDate(folders[i].DownloadDate)
		dj, _ := util.ParseDate(folders[j].DownloadDate)
		if !di.Equal(dj) {
			return di.After(dj)
		}
		pi, _ := util.ParseDate(folders[i].PublishDate)
		pj, _ := util.ParseDate(folders[j].PublishDate)
		return pi.After(pj)
	})
	return folders
}

// GetFolder читает одну папку по имени.
func GetFolder(root, name string) (model.Folder, bool) {
	download, publish, ok := ParseFolderName(name)
	if !ok {
		return model.Folder{}, false
	}
	path := filepath.Join(root, name)
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return model.Folder{}, false
	}
	return readFolder(root, name, download, publish), true
}

func readFolder(root, name, download, publish string) model.Folder {
	folder := model.Folder{
		Name:         name,
		Path:         filepath.Join(root, name),
		DownloadDate: download,
		PublishDate:  publish,
		Files:        []model.MediaItem{},
	}
	entries, err := os.ReadDir(folder.Path)
	if err != nil {
		return folder
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		kind := model.KindOf(entry.Name())
		folder.Files = append(folder.Files, model.MediaItem{
			Path:    filepath.Join(folder.Path, entry.Name()),
			Name:    entry.Name(),
			Size:    info.Size(),
			Kind:    kind,
			ModTime: info.ModTime(),
		})
		folder.Count++
		folder.Size += info.Size()
		switch kind {
		case model.KindVideo:
			folder.Videos++
		case model.KindPhoto:
			folder.Photos++
		case model.KindAudio:
			folder.Audio++
		}
	}
	sort.SliceStable(folder.Files, func(i, j int) bool { return folder.Files[i].Name < folder.Files[j].Name })
	return folder
}

// IterMedia возвращает все файлы библиотеки (или одной папки), свежие сверху.
func IterMedia(root, folderName string) []model.MediaItem {
	var items []model.MediaItem
	if folderName != "" {
		if folder, ok := GetFolder(root, folderName); ok {
			items = append(items, folder.Files...)
		}
	} else {
		for _, folder := range ListFolders(root) {
			items = append(items, folder.Files...)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ModTime.After(items[j].ModTime) })
	return items
}

// FolderNames возвращает только имена папок-дат.
func FolderNames(root string) []string {
	folders := ListFolders(root)
	names := make([]string, 0, len(folders))
	for _, folder := range folders {
		names = append(names, folder.Name)
	}
	return names
}
