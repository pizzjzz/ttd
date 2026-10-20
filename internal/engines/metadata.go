package engines

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FindSidecar ищет файл метаданных, созданный движком рядом с медиафайлом.
// yt-dlp пишет "<имя>.info.json", gallery-dl — "<имя>.json".
func FindSidecar(mediaPath string) string {
	candidates := []string{
		mediaPath + ".info.json",
		mediaPath + ".json",
	}
	stem := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	candidates = append(candidates,
		stem+".info.json",
		stem+".json",
	)
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ReadPublishDate возвращает дату публикации для медиафайла по его метаданным.
func ReadPublishDate(mediaPath string) (time.Time, bool) {
	sidecar := FindSidecar(mediaPath)
	if sidecar == "" {
		return time.Time{}, false
	}
	data, err := os.ReadFile(sidecar)
	if err != nil {
		return time.Time{}, false
	}
	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return time.Time{}, false
	}
	return dateFromMeta(meta)
}

func dateFromMeta(meta map[string]any) (time.Time, bool) {
	// 1. Явная дата вида YYYYMMDD (yt-dlp upload_date).
	if value, ok := meta["upload_date"].(string); ok && len(value) == 8 {
		if t, err := time.ParseInLocation("20060102", value, time.Local); err == nil {
			return t, true
		}
	}
	// 2. Строковая дата в разных форматах (gallery-dl date).
	for _, key := range []string{"date", "published", "created_at", "created"} {
		if value, ok := meta[key].(string); ok {
			if t, ok := parseDateString(value); ok {
				return t, true
			}
		}
	}
	// 3. Числовые метки времени в секундах.
	for _, key := range []string{"timestamp", "create_time", "created_time", "date_utc"} {
		if t, ok := numberToTime(meta[key], false); ok {
			return t, true
		}
	}
	// 4. Метки времени в миллисекундах.
	for _, key := range []string{"createTime", "create_time_ms", "timestamp_ms", "date_ms"} {
		if t, ok := numberToTime(meta[key], true); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseDateString(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02",
		"20060102",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func numberToTime(value any, millis bool) (time.Time, bool) {
	switch typed := value.(type) {
	case float64:
		return unixToTime(int64(typed), millis)
	case int64:
		return unixToTime(typed, millis)
	case int:
		return unixToTime(int64(typed), millis)
	case string:
		if typed == "" {
			return time.Time{}, false
		}
		if n, err := strconv.ParseInt(typed, 10, 64); err == nil {
			return unixToTime(n, millis)
		}
	}
	return time.Time{}, false
}

func unixToTime(value int64, millis bool) (time.Time, bool) {
	if value <= 0 {
		return time.Time{}, false
	}
	// Определяем единицы по величине, если явно не указано.
	if !millis && value > 1_000_000_000_000 {
		millis = true
	}
	if millis {
		return time.Unix(value/1000, 0).Local(), true
	}
	return time.Unix(value, 0).Local(), true
}
