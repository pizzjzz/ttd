// Package engines содержит обёртки над внешними загрузчиками (gallery-dl, yt-dlp),
// классификацию ссылок TikTok и зондирование доступного качества.
package engines

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ttd/internal/config"
	"ttd/internal/model"
	"ttd/internal/util"
)

// Progress — разобранная строка прогресса.
type Progress struct {
	Percent    float64
	HasPercent bool
	Speed      string
	Eta        string
}

// Outcome — результат работы движка над одной ссылкой.
type Outcome struct {
	OK        bool
	Files     []model.MediaItem
	Error     string
	Cancelled bool
}

// Engine — загрузчик.
type Engine interface {
	Name() string
	Available() bool
	Version() string
	Download(ctx context.Context, url string, opts model.DownloadOptions, dest string, onEvent func(model.Event)) Outcome
}

// commandEngine — общая основа для движков, работающих через внешний процесс.
type commandEngine struct {
	name          string
	tool          string
	build         func(url string, opts model.DownloadOptions, dest string) []string
	parseProgress func(line string) *Progress
}

func (e *commandEngine) Name() string    { return e.name }
func (e *commandEngine) Available() bool { return e.tool != "" }
func (e *commandEngine) Version() string { return ToolVersion(e.tool) }

func (e *commandEngine) Download(ctx context.Context, url string, opts model.DownloadOptions, dest string, onEvent func(model.Event)) Outcome {
	emit := EventFunc(onEvent)
	if !e.Available() {
		return Outcome{Error: e.name + ": движок не найден"}
	}
	args := e.build(url, opts, dest)
	if opts.DryRun {
		emit(model.Event{"type": "log", "engine": e.name, "line": "[dry-run] " + e.tool + " " + strings.Join(args, " ")})
		return Outcome{OK: true}
	}
	return runCommand(ctx, e.name, e.tool, args, dest, e.parseProgress, url, emit)
}

// runCommand запускает движок, транслирует прогресс и считает новые файлы.
func runCommand(
	ctx context.Context,
	name, tool string,
	args []string,
	dest string,
	parseProgress func(string) *Progress,
	url string,
	emit func(model.Event),
) Outcome {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return Outcome{Error: err.Error()}
	}
	before := util.TakeSnapshot(dest)
	seen := map[string]bool{}
	logTail := make([]string, 0, 8)

	emit(model.Event{"type": "log", "engine": name, "line": "$ " + tool + " " + strings.Join(args, " ")})

	onLine := func(line string) {
		if line != "" {
			logTail = append(logTail, line)
			if len(logTail) > 8 {
				logTail = logTail[1:]
			}
			emit(model.Event{"type": "log", "engine": name, "line": line})
		}
		if parseProgress != nil {
			if p := parseProgress(line); p != nil {
				emit(model.Event{
					"type": "progress", "engine": name, "url": url,
					"percent": p.Percent, "speed": p.Speed, "eta": p.Eta,
				})
			}
		}
		for _, file := range util.NewFiles(before, dest) {
			if seen[file] || util.IsSidecar(file) {
				continue
			}
			seen[file] = true
			emit(model.Event{"type": "file", "engine": name, "url": url, "file": mediaItem(filepath.Join(dest, file), url)})
		}
	}

	result := util.RunStream(ctx, tool, args, engineEnv(), onLine)
	files := make([]model.MediaItem, 0, len(seen))
	for file := range seen {
		files = append(files, mediaItem(filepath.Join(dest, file), url))
	}

	if result.Cancelled {
		return Outcome{Files: files, Cancelled: true, Error: "отменено"}
	}
	if result.ExitCode != 0 {
		return Outcome{Files: files, Error: errorHint(result.ExitCode, logTail)}
	}
	return Outcome{OK: true, Files: files}
}

func errorHint(code int, tail []string) string {
	last := ""
	for i := len(tail) - 1; i >= 0; i-- {
		if strings.TrimSpace(tail[i]) != "" {
			last = tail[i]
			break
		}
	}
	base := "движок завершился с кодом " + strconv.Itoa(code)
	if last != "" {
		return base + ": " + last
	}
	return base
}

func mediaItem(path, url string) model.MediaItem {
	var size int64
	mtime := time.Time{}
	if info, err := os.Stat(path); err == nil {
		size = info.Size()
		mtime = info.ModTime()
	}
	return model.MediaItem{
		Path:    path,
		Name:    filepath.Base(path),
		Size:    size,
		Kind:    model.KindOf(path),
		ModTime: mtime,
		URL:     url,
	}
}

// EventFunc возвращает безопасный приёмник событий.
func EventFunc(onEvent func(model.Event)) func(model.Event) {
	if onEvent != nil {
		return onEvent
	}
	return func(model.Event) {}
}

func engineEnv() []string {
	return append(os.Environ(), "PYTHONIOENCODING=utf-8")
}

// CandidateBinDirs — где искать движки.
func CandidateBinDirs() []string {
	root := config.ProjectRoot()
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(root, ".venv", "bin"),
		filepath.Join(root, ".venv", "Scripts"),
		filepath.Join(home, ".local", "share", "ttd", "venv", "bin"),
		filepath.Join(home, ".local", "bin"),
	}
}

// FindTools ищет gallery-dl и yt-dlp. Пустая строка — не найден.
func FindTools(cfg *config.Config) map[string]string {
	dirs := CandidateBinDirs()
	return map[string]string{
		"gallery-dl": util.FindExecutable([]string{"gallery-dl"}, append(explicit(cfg.GalleryDL), dirs...)),
		"yt-dlp":     util.FindExecutable([]string{"yt-dlp"}, append(explicit(cfg.YtDlp), dirs...)),
	}
}

func explicit(path string) []string {
	if path == "" {
		return nil
	}
	return []string{filepath.Dir(util.ExpandHome(path))}
}

// ToolVersion возвращает первую строку вывода <tool> --version.
func ToolVersion(tool string) string {
	if tool == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, errOut, err := util.Capture(ctx, tool, []string{"--version"}, nil)
	if err != nil && out == "" {
		out = errOut
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// BuildEngines создаёт движки по настройкам.
func BuildEngines(cfg *config.Config, tools map[string]string) map[string]Engine {
	if tools == nil {
		tools = FindTools(cfg)
	}
	return map[string]Engine{
		"gallery-dl": NewGalleryDL(tools["gallery-dl"]),
		"yt-dlp":     NewYtDlp(tools["yt-dlp"]),
		"mock":       NewMock(),
	}
}
