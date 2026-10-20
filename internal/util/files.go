package util

import (
	"os"
	"path/filepath"
	"sort"
)

// Snapshot — снимок файлов каталога: имя → время изменения (UnixNano).
type Snapshot map[string]int64

// TakeSnapshot собирает снимок каталога, игнорируя временные файлы.
func TakeSnapshot(dir string) Snapshot {
	result := Snapshot{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() || IsTempFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result[entry.Name()] = info.ModTime().UnixNano()
	}
	return result
}

// NewFiles возвращает файлы, появившиеся или обновлённые после снимка.
func NewFiles(before Snapshot, dir string) []string {
	after := TakeSnapshot(dir)
	names := make([]string, 0, len(after))
	for name, mtime := range after {
		if prev, ok := before[name]; !ok || mtime > prev {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		ai, _ := os.Stat(filepath.Join(dir, names[i]))
		aj, _ := os.Stat(filepath.Join(dir, names[j]))
		if ai == nil || aj == nil {
			return names[i] < names[j]
		}
		return ai.ModTime().Before(aj.ModTime())
	})
	return names
}

// ListFiles возвращает имена всех обычных файлов каталога (без скрытых и временных).
func ListFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || IsTempFile(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
