// Package model описывает структуры данных, общие для всех частей ttd.
package model

import "time"

// Типы медиафайлов.
const (
	KindVideo = "video"
	KindPhoto = "photo"
	KindAudio = "audio"
	KindOther = "other"
)

// Статусы обработки ссылки.
const (
	StatusPending   = "pending"
	StatusOK        = "ok"
	StatusSkipped   = "skipped"
	StatusError     = "error"
	StatusCancelled = "cancelled"
)

// Виды ссылок TikTok.
const (
	KindStory      = "story"
	KindVideoPost  = "video"
	KindPhotoPost  = "photo"
	KindProfile    = "profile"
	KindCollection = "collection"
	KindLive       = "live"
	KindShort      = "short"
	KindExternal   = "external"
	KindUnknown    = "unknown"
)

// MediaItem — один файл библиотеки или результат загрузки.
type MediaItem struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Kind    string    `json:"kind"`
	ModTime time.Time `json:"mod_time"`
	URL     string    `json:"url,omitempty"`
}

// ProbeFormat — один доступный поток (качество) из метаданных yt-dlp.
type ProbeFormat struct {
	FormatID   string  `json:"format_id"`
	Ext        string  `json:"ext,omitempty"`
	Resolution string  `json:"resolution,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	FPS        float64 `json:"fps,omitempty"`
	VCodec     string  `json:"vcodec,omitempty"`
	ACodec     string  `json:"acodec,omitempty"`
	FileSize   int64   `json:"filesize,omitempty"`
	TBR        float64 `json:"tbr,omitempty"`
}

// ProbeResult — итог анализа ссылки.
type ProbeResult struct {
	URL        string        `json:"url"`
	Kind       string        `json:"kind"`
	Title      string        `json:"title,omitempty"`
	Uploader   string        `json:"uploader,omitempty"`
	Duration   float64       `json:"duration,omitempty"`
	IsPlaylist bool          `json:"is_playlist,omitempty"`
	EntryCount int           `json:"entry_count,omitempty"`
	Formats    []ProbeFormat `json:"formats,omitempty"`
	Best       *ProbeFormat  `json:"best,omitempty"`
	Engine     string        `json:"engine,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// JobResult — результат обработки одной ссылки.
type JobResult struct {
	URL    string      `json:"url"`
	Kind   string      `json:"kind"`
	Engine string      `json:"engine"`
	Status string      `json:"status"`
	Files  []MediaItem `json:"files"`
	Error  string      `json:"error,omitempty"`
}

// DownloadOptions — параметры одной операции скачивания.
type DownloadOptions struct {
	MediaRoot          string
	CacheDir           string
	Date               string // папка-дата скачивания (ДД.ММ.ГГГГ); пусто = сегодня
	Engine             string
	Quality            string
	Cookies            string
	CookiesFromBrowser string
	IncludeStories     bool
	Limit              int
	AudioOnly          bool
	DryRun             bool
	Verbose            bool
	NoArchive          bool
	ExtraArgs          []string
}

// Event — событие прогресса для CLI/TUI/Web. Сериализуется в JSON как есть.
type Event = map[string]any

// PlannedURL — ссылка с определённым видом после разворачивания.
type PlannedURL struct {
	Kind string
	URL  string
}

// Folder — папка библиотеки вида "19.10.2026 | 17.10.2026".
type Folder struct {
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	DownloadDate string      `json:"download_date"`
	PublishDate  string      `json:"publish_date"`
	Count        int         `json:"count"`
	Videos       int         `json:"videos"`
	Photos       int         `json:"photos"`
	Audio        int         `json:"audio"`
	Size         int64       `json:"size"`
	Files        []MediaItem `json:"files"`
}
