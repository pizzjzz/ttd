package engines

import (
	"fmt"
	"path/filepath"

	"ttd/internal/model"
)

// galleryFilenameFmt — шаблон имени файла gallery-dl.
// Для обычного видео {num} == 0 и опускается, для слайдшоу даёт _01, _02, …
const galleryFilenameFmt = "{user}_{id}{num:?_//>02}.{extension}"

// NewGalleryDL создаёт движок gallery-dl.
func NewGalleryDL(tool string) Engine {
	engine := &commandEngine{name: "gallery-dl", tool: tool}
	engine.build = func(url string, opts model.DownloadOptions, dest string) []string {
		args := []string{
			"--no-colors",
			"--config-ignore",
			"-D", dest,
			"-f", galleryFilenameFmt,
			"--write-metadata",
		}
		if opts.Cookies != "" {
			args = append(args, "-C", opts.Cookies)
		}
		if opts.CookiesFromBrowser != "" {
			args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
		}
		if opts.Limit > 0 {
			args = append(args, "--range", fmt.Sprintf("1-%d", opts.Limit))
		}
		if !opts.NoArchive && opts.CacheDir != "" {
			args = append(args, "--download-archive", filepath.Join(opts.CacheDir, "gallery-dl.archive"))
		}
		args = append(args, opts.ExtraArgs...)
		args = append(args, url)
		return args
	}
	// gallery-dl не печатает проценты — прогресс показывается по мере появления файлов.
	engine.parseProgress = nil
	return engine
}
