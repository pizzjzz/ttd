package engines

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"ttd/internal/config"
	"ttd/internal/model"
	"ttd/internal/util"
)

// ytFormat — подмножество полей формата из вывода yt-dlp -J.
type ytFormat struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Resolution     string  `json:"resolution"`
	Width          int     `json:"width"`
	Height         int     `json:"height"`
	FPS            float64 `json:"fps"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	FileSize       int64   `json:"filesize"`
	FileSizeApprox int64   `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
}

// ytProbe — подмножество полей из вывода yt-dlp -J.
type ytProbe struct {
	Type       string            `json:"_type"`
	Title      string            `json:"title"`
	Uploader   string            `json:"uploader"`
	Creator    string            `json:"creator"`
	Channel    string            `json:"channel"`
	Duration   float64           `json:"duration"`
	Timestamp  float64           `json:"timestamp"`
	UploadDate string            `json:"upload_date"`
	Entries    []json.RawMessage `json:"entries"`
	Formats    []ytFormat        `json:"formats"`
}

// Probe анализирует ссылку и сообщает доступное качество.
func Probe(ctx context.Context, rawURL string, cfg *config.Config) model.ProbeResult {
	url := NormalizeURL(rawURL)
	kind := Classify(url)
	result := model.ProbeResult{URL: url, Kind: kind}

	switch kind {
	case model.KindExternal:
		result.Error = "ссылка не относится к TikTok"
		return result
	case model.KindUnknown:
		result.Error = "неизвестный тип ссылки TikTok"
		return result
	}

	if cfg.Engine == "mock" {
		return mockProbe(result)
	}

	tools := FindTools(cfg)
	tool := tools["yt-dlp"]
	if tool == "" {
		result.Error = "yt-dlp не найден — качество определить нельзя"
		return result
	}

	args := []string{"--no-colors", "--no-warnings", "-J"}
	if kind == model.KindProfile || kind == model.KindCollection || kind == model.KindStory {
		args = append(args, "--flat-playlist")
	}
	if cfg.Cookies != "" {
		args = append(args, "--cookies", cfg.Cookies)
	}
	if cfg.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", cfg.CookiesFromBrowser)
	}
	args = append(args, url)

	out, errOut, err := util.Capture(ctx, tool, args, nil)
	trimmed := strings.TrimSpace(out)
	if trimmed == "" || trimmed == "null" {
		result.Error = errorText(err, errOut)
		return result
	}

	var data ytProbe
	if unmarshalErr := json.Unmarshal([]byte(trimmed), &data); unmarshalErr != nil {
		result.Error = "не удалось разобрать метаданные: " + unmarshalErr.Error()
		return result
	}
	// yt-dlp мог вернуть и ошибку, и пустой объект — тогда сообщаем причину.
	if err != nil && data.Title == "" && len(data.Formats) == 0 && len(data.Entries) == 0 {
		result.Error = errorText(err, errOut)
		return result
	}

	result.Engine = "yt-dlp"
	result.Title = data.Title
	result.Uploader = firstNonEmpty(data.Uploader, data.Creator, data.Channel)
	result.Duration = data.Duration
	result.IsPlaylist = data.Type == "playlist" || len(data.Entries) > 0
	result.EntryCount = len(data.Entries)

	for _, format := range data.Formats {
		size := format.FileSize
		if size == 0 {
			size = format.FileSizeApprox
		}
		result.Formats = append(result.Formats, model.ProbeFormat{
			FormatID:   format.FormatID,
			Ext:        format.Ext,
			Resolution: format.Resolution,
			Width:      format.Width,
			Height:     format.Height,
			FPS:        format.FPS,
			VCodec:     format.VCodec,
			ACodec:     format.ACodec,
			FileSize:   size,
			TBR:        format.TBR,
		})
	}
	if kind == model.KindVideoPost || !result.IsPlaylist {
		if best := bestFormat(result.Formats); best != nil {
			result.Best = best
		}
	}
	return result
}

// bestFormat выбирает лучшее качество из списка форматов.
func bestFormat(formats []model.ProbeFormat) *model.ProbeFormat {
	var best *model.ProbeFormat
	for i := range formats {
		candidate := formats[i]
		if candidate.VCodec == "none" {
			continue
		}
		if best == nil || betterFormat(candidate, *best) {
			copy := candidate
			best = &copy
		}
	}
	return best
}

func betterFormat(a, b model.ProbeFormat) bool {
	if a.Height != b.Height {
		return a.Height > b.Height
	}
	if a.TBR != b.TBR {
		return a.TBR > b.TBR
	}
	return a.FileSize > b.FileSize
}

func mockProbe(result model.ProbeResult) model.ProbeResult {
	result.Engine = "mock"
	result.Title = "mock: " + result.URL
	if result.Kind == model.KindProfile || result.Kind == model.KindCollection || result.Kind == model.KindStory {
		result.IsPlaylist = true
		result.EntryCount = 3
		return result
	}
	result.Formats = []model.ProbeFormat{
		{FormatID: "mock-1080", Ext: "mp4", Resolution: "1080x1920", Width: 1080, Height: 1920, VCodec: "avc1", ACodec: "mp4a", TBR: 2500},
		{FormatID: "mock-720", Ext: "mp4", Resolution: "720x1280", Width: 720, Height: 1280, VCodec: "avc1", ACodec: "mp4a", TBR: 1500},
		{FormatID: "mock-480", Ext: "mp4", Resolution: "480x864", Width: 480, Height: 864, VCodec: "avc1", ACodec: "mp4a", TBR: 800},
	}
	best := result.Formats[0]
	result.Best = &best
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func errorText(err error, stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "превышено время ожидания ответа TikTok"
		}
		return err.Error()
	}
	return "неизвестная ошибка"
}
