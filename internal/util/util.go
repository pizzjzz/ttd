// Package util содержит небольшие помощники, общие для модулей ttd.
package util

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DateLayout — формат даты в именах папок библиотеки.
const DateLayout = "02.01.2006"

var (
	dateRe       = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)
	unsafeRe     = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
	ansiRe       = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
	tempSuffixes = []string{".part", ".ytdl", ".tmp", ".temp", ".part-frag", ".frag"}
	tempFragment = regexp.MustCompile(`(?i)\.f\d+\.[a-z0-9]+$`)
)

// Today возвращает текущую дату в формате ДД.ММ.ГГГГ.
func Today() string { return time.Now().Format(DateLayout) }

// FormatDate приводит время к формату ДД.ММ.ГГГГ.
func FormatDate(t time.Time) string { return t.Format(DateLayout) }

// ParseDate разбирает ДД.ММ.ГГГГ.
func ParseDate(s string) (time.Time, bool) {
	t, err := time.Parse(DateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// IsDateName сообщает, похоже ли имя каталога на 19.10.2026.
func IsDateName(name string) bool { return dateRe.MatchString(name) }

// SanitizeFilename убирает из имени недопустимые символы.
func SanitizeFilename(name string, maxLen int) string {
	cleaned := strings.Trim(strings.TrimSpace(unsafeRe.ReplaceAllString(name, "_")), ".")
	if cleaned == "" {
		cleaned = "file"
	}
	if maxLen > 0 && len(cleaned) > maxLen {
		cleaned = cleaned[:maxLen]
	}
	return cleaned
}

// HumanSize форматирует размер в удобочитаемый вид.
func HumanSize(n int64) string {
	if n < 0 {
		return "?"
	}
	value := float64(n)
	for _, unit := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
		if value < 1024 || unit == "TiB" {
			if unit == "B" {
				return fmt.Sprintf("%d B", n)
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%.1f TiB", value)
}

// HumanSpeed форматирует скорость.
func HumanSpeed(bps float64) string {
	if bps <= 0 {
		return ""
	}
	return HumanSize(int64(bps)) + "/s"
}

// HumanETA форматирует оставшееся время.
func HumanETA(seconds float64) string {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return ""
	}
	total := int(seconds)
	h, rem := total/3600, total%3600
	m, s := rem/60, rem%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// StripANSI удаляет управляющие последовательности цвета.
func StripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// IsTempFile сообщает, является ли файл промежуточным артефактом загрузчика.
func IsTempFile(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
		return true
	}
	lower := strings.ToLower(name)
	for _, suffix := range tempSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return tempFragment.MatchString(lower)
}

// IsSidecar сообщает, является ли файл метаданными движка (info.json).
func IsSidecar(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".info.json") || strings.HasSuffix(lower, ".json")
}

// UniquePath подбирает свободное имя, добавляя _1, _2 и т.д.
func UniquePath(dir, name string) string {
	target := filepath.Join(dir, name)
	if !exists(target) {
		return target
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; i < 100000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", stem, i, ext))
		if !exists(candidate) {
			return candidate
		}
	}
	return target
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// SafeJoin соединяет пути и проверяет, что результат остаётся внутри root.
func SafeJoin(root string, parts ...string) (string, bool) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	joined := filepath.Join(append([]string{rootAbs}, parts...)...)
	rel, err := filepath.Rel(rootAbs, joined)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return joined, true
}

// ExpandHome разворачивает ведущий ~ в домашний каталог.
func ExpandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	prefix := "~/"
	if runtime.GOOS == "windows" {
		// В Windows пользователи часто пишут ~\ttd.
		if strings.HasPrefix(path, `~\`) {
			prefix = `~\`
		}
	}
	if strings.HasPrefix(path, prefix) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[len(prefix):])
		}
	}
	return path
}

// IsTerminal сообщает, подключён ли стандартный ввод к терминалу.
func IsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
