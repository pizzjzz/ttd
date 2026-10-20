package engines

import (
	"net/url"
	"regexp"
	"strings"

	"ttd/internal/model"
)

var tiktokDomains = []string{"tiktok.com", "tiktokv.com", "tiktokcdn.com"}

var tiktokHostRe = regexp.MustCompile(`^(?:www\.)?(?:tiktok\.com|tiktokv\.com|vm\.tiktok\.com|vt\.tiktok\.com|m\.tiktok\.com)/`)

// HostOf возвращает хост ссылки (без порта и учётных данных).
func HostOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	return host
}

// NormalizeURL приводит короткие формы к полноценной ссылке.
func NormalizeURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "@") {
		return "https://www.tiktok.com/" + value
	}
	if !strings.Contains(value, "://") && tiktokHostRe.MatchString(value) {
		return "https://" + value
	}
	return value
}

// IsTikTokURL сообщает, относится ли ссылка к TikTok.
func IsTikTokURL(raw string) bool {
	host := HostOf(raw)
	for _, domain := range tiktokDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// Classify определяет вид ссылки.
func Classify(raw string) string {
	value := NormalizeURL(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		return model.KindUnknown
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.TrimRight(strings.ToLower(parsed.Path), "/")

	if !IsTikTokURL(value) {
		return model.KindExternal
	}
	switch {
	case strings.Contains(path, "/stories"):
		return model.KindStory
	case strings.Contains(path, "/video/"):
		return model.KindVideoPost
	case strings.Contains(path, "/photo/"):
		return model.KindPhotoPost
	case strings.Contains(path, "/collection/"):
		return model.KindCollection
	case strings.HasSuffix(path, "/live"):
		return model.KindLive
	}
	if regexp.MustCompile(`/@[\w.\-]+`).MatchString(parsed.Path) {
		return model.KindProfile
	}
	for _, prefix := range []string{"/music/", "/tag/", "/stickers/", "/sticker/", "/discover/"} {
		if strings.HasPrefix(path, prefix) {
			return model.KindCollection
		}
	}
	if host == "vm.tiktok.com" || host == "vt.tiktok.com" || strings.HasPrefix(path, "/t/") {
		return model.KindShort
	}
	return model.KindUnknown
}

// StoriesURL возвращает ссылку на истории профиля.
func StoriesURL(raw string) string {
	value := NormalizeURL(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(strings.ToLower(path), "/stories") {
		path += "/stories"
	}
	parsed.Path = path
	return parsed.String()
}

// ExpandURLs разворачивает список ссылок, добавляя истории для профилей.
func ExpandURLs(urls []string, includeStories bool) []model.PlannedURL {
	planned := make([]model.PlannedURL, 0, len(urls))
	seen := map[string]bool{}
	add := func(kind, url string) {
		key := kind + "\x00" + url
		if seen[key] {
			return
		}
		seen[key] = true
		planned = append(planned, model.PlannedURL{Kind: kind, URL: url})
	}
	for _, raw := range urls {
		value := NormalizeURL(raw)
		if value == "" {
			continue
		}
		kind := Classify(value)
		add(kind, value)
		if includeStories && kind == model.KindProfile {
			add(model.KindStory, StoriesURL(value))
		}
	}
	return planned
}

// EngineOrder — порядок движков для данного вида ссылки.
func EngineOrder(kind, requested string) []string {
	if requested != "" && requested != "auto" {
		return []string{requested}
	}
	switch kind {
	case model.KindStory:
		return []string{"gallery-dl"}
	case model.KindVideoPost:
		return []string{"yt-dlp", "gallery-dl"}
	case model.KindPhotoPost:
		return []string{"gallery-dl", "yt-dlp"}
	case model.KindProfile, model.KindCollection:
		return []string{"gallery-dl", "yt-dlp"}
	default:
		return []string{"yt-dlp", "gallery-dl"}
	}
}
