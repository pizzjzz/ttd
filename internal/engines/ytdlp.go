package engines

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"ttd/internal/model"
)

// ytdlpOutTmpl — шаблон имени файла yt-dlp.
const ytdlpOutTmpl = "%(uploader,creator,channel,id)s_%(id)s.%(ext)s"

// ytdlpProgressPrefix — маркер наших строк прогресса.
const ytdlpProgressPrefix = "TTD-PROGRESS "

// ytdlpProgressTmpl — шаблон строки прогресса yt-dlp.
const ytdlpProgressTmpl = "download:" + ytdlpProgressPrefix +
	"%(progress._percent_str)s|%(progress._speed_str)s|%(progress._eta_str)s|" +
	"%(progress.downloaded_bytes)s|%(progress.total_bytes_estimate)s"

var ytdlpProgressRe = regexp.MustCompile(`TTD-PROGRESS\s+([\d.]+)%\|([^|]*)\|([^|]*)\|`)

// NewYtDlp создаёт движок yt-dlp.
func NewYtDlp(tool string) Engine {
	engine := &commandEngine{name: "yt-dlp", tool: tool}
	engine.build = func(url string, opts model.DownloadOptions, dest string) []string {
		args := []string{
			"--no-colors",
			"--newline",
			"--no-warnings",
			"--no-progress",
			"--no-simulate",
			"--print", "after_move:filepath",
			"--write-info-json",
			"--progress",
			"--progress-template", ytdlpProgressTmpl,
			"-P", dest,
			"-o", ytdlpOutTmpl,
		}
		if opts.AudioOnly {
			args = append(args, "-x", "--audio-format", "mp3")
		} else {
			args = append(args, "-f", FormatSelector(opts.Quality), "--merge-output-format", "mp4")
		}
		if opts.Cookies != "" {
			args = append(args, "--cookies", opts.Cookies)
		}
		if opts.CookiesFromBrowser != "" {
			args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
		}
		if opts.Limit > 0 {
			args = append(args, "--playlist-end", strconv.Itoa(opts.Limit))
		}
		if !opts.NoArchive && opts.CacheDir != "" {
			args = append(args, "--download-archive", filepath.Join(opts.CacheDir, "yt-dlp.archive"))
		}
		args = append(args, opts.ExtraArgs...)
		args = append(args, url)
		return args
	}
	engine.parseProgress = parseYtDlpProgress
	return engine
}

func parseYtDlpProgress(line string) *Progress {
	match := ytdlpProgressRe.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	percent, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	speed := strings.TrimSpace(match[2])
	eta := strings.TrimSpace(match[3])
	if speed == "N/A" || speed == "Unknown" {
		speed = ""
	}
	if eta == "N/A" || eta == "Unknown" {
		eta = ""
	}
	return &Progress{Percent: percent, HasPercent: true, Speed: speed, Eta: eta}
}

// FormatSelector превращает понятное имя качества в строку -f для yt-dlp.
func FormatSelector(quality string) string {
	value := strings.TrimSpace(strings.ToLower(quality))
	switch value {
	case "", "best", "max", "maximum", "maximal", "highest":
		return "bestvideo*+bestaudio/best"
	}
	if strings.ContainsAny(value, "+[]()") {
		// Пользователь передал готовый селектор — используем как есть.
		return quality
	}
	if height, err := strconv.Atoi(strings.TrimSuffix(value, "p")); err == nil && height > 0 {
		return fmt.Sprintf("bestvideo[height<=%d]+bestaudio/best[height<=%d]", height, height)
	}
	return quality
}
