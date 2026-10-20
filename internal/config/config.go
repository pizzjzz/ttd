// Package config отвечает за настройки ttd и пути к каталогам.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ttd/internal/util"
)

// SupportedEngines — допустимые значения движка.
var SupportedEngines = []string{"auto", "gallery-dl", "yt-dlp", "mock"}

// Config — настройки приложения.
type Config struct {
	MediaRoot          string `json:"media_root"`
	Engine             string `json:"engine"`
	Quality            string `json:"quality"`
	Cookies            string `json:"cookies"`
	CookiesFromBrowser string `json:"cookies_from_browser"`
	IncludeStories     bool   `json:"include_stories"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	OpenBrowser        bool   `json:"open_browser"`
	DownloadLimit      int    `json:"download_limit"`
	GalleryDL          string `json:"gallery_dl"`
	YtDlp              string `json:"yt_dlp"`
	UseArchive         bool   `json:"use_archive"`
}

// Default возвращает настройки по умолчанию.
func Default() Config {
	return Config{
		MediaRoot:   DefaultMediaRoot(),
		Engine:      "auto",
		Quality:     "best",
		Host:        "127.0.0.1",
		Port:        8787,
		OpenBrowser: true,
		UseArchive:  true,
	}
}

// DefaultMediaRoot — ~/ttd.
func DefaultMediaRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "ttd"
	}
	return filepath.Join(home, "ttd")
}

// ProjectRoot — каталог установки ttd (где лежат .venv и сами движки).
//
// Определяется по расположению исполняемого файла с учётом символических
// ссылок; переопределяется переменной окружения TTD_HOME.
func ProjectRoot() string {
	if env := os.Getenv("TTD_HOME"); env != "" {
		return util.ExpandHome(env)
	}
	exe, err := os.Executable()
	if err != nil {
		if wd, err2 := os.Getwd(); err2 == nil {
			return wd
		}
		return "."
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	// go run собирает бинарник во временный каталог — тогда берём рабочий каталог.
	if strings.Contains(dir, "go-build") {
		if wd, err := os.Getwd(); err == nil {
			return wd
		}
	}
	return dir
}

// ConfigFile — путь к файлу настроек (~/.config/ttd/config.json).
func ConfigFile() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "config.json"
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "ttd", "config.json")
}

// Load читает файл настроек и применяет переменные окружения.
func Load() Config {
	cfg := Default()
	if data, err := os.ReadFile(ConfigFile()); err == nil {
		// Заполняем поверх значений по умолчанию.
		_ = json.Unmarshal(data, &cfg)
	}
	applyEnv(&cfg)
	cfg.normalize()
	return cfg
}

// Save записывает настройки в файл.
func Save(cfg Config) (string, error) {
	path := ConfigFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return path, err
	}
	data = append(data, '\n')
	return path, os.WriteFile(path, data, 0o644)
}

func (c *Config) normalize() {
	if c.MediaRoot == "" {
		c.MediaRoot = DefaultMediaRoot()
	}
	c.MediaRoot = util.ExpandHome(c.MediaRoot)
	if c.Engine == "" {
		c.Engine = "auto"
	}
	if c.Quality == "" {
		c.Quality = "best"
	}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 8787
	}
}

// MediaPath — абсолютный путь к библиотеке.
func (c Config) MediaPath() string {
	path := util.ExpandHome(c.MediaRoot)
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// CacheDir — служебный каталог внутри библиотеки (архивы, временные данные).
func (c Config) CacheDir() string { return filepath.Join(c.MediaPath(), ".ttd-cache") }

// DataDir — стабильный каталог данных установленного ttd (движки, окружение).
// По умолчанию ~/.local/share/ttd; переопределяется TTD_DATA_DIR или
// XDG_DATA_HOME.
func DataDir() string {
	if value := os.Getenv("TTD_DATA_DIR"); value != "" {
		return util.ExpandHome(value)
	}
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		return filepath.Join(util.ExpandHome(value), "ttd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(ProjectRoot(), "data")
	}
	return filepath.Join(home, ".local", "share", "ttd")
}

// DataDirs перечисляет каталоги, где может лежать установленный ttd:
// пользовательский, системный и каталог по умолчанию.
func DataDirs() []string {
	dirs := []string{DataDir()}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "ttd"))
	}
	dirs = append(dirs, "/usr/local/share/ttd", "/usr/share/ttd")
	return dedupePaths(dirs)
}

// VenvCandidates перечисляет возможные расположения окружения с движками.
// Порядок: явный TTD_VENV, окружение рядом с исходниками, каталоги данных.
func VenvCandidates() []string {
	dirs := make([]string, 0, 8)
	if value := os.Getenv("TTD_VENV"); value != "" {
		dirs = append(dirs, util.ExpandHome(value))
	}
	dirs = append(dirs, filepath.Join(ProjectRoot(), ".venv"))
	for _, dataDir := range DataDirs() {
		dirs = append(dirs, filepath.Join(dataDir, "venv"))
	}
	return dedupePaths(dirs)
}

// VenvDir — локальное окружение с движками. Если какое-то из возможных мест
// уже существует, используется оно; иначе выбирается стабильный каталог данных.
func (c Config) VenvDir() string {
	for _, candidate := range VenvCandidates() {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join(DataDir(), "venv")
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		cleaned := filepath.Clean(path)
		if seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		result = append(result, cleaned)
	}
	return result
}

func applyEnv(cfg *Config) {
	setStr := func(env string, target *string) {
		if v, ok := os.LookupEnv(env); ok && v != "" {
			*target = v
		}
	}
	setStr("TTD_DIR", &cfg.MediaRoot)
	setStr("TTD_ENGINE", &cfg.Engine)
	setStr("TTD_QUALITY", &cfg.Quality)
	setStr("TTD_COOKIES", &cfg.Cookies)
	setStr("TTD_COOKIES_FROM_BROWSER", &cfg.CookiesFromBrowser)
	setStr("TTD_HOST", &cfg.Host)
	setStr("TTD_GALLERY_DL", &cfg.GalleryDL)
	setStr("TTD_YT_DLP", &cfg.YtDlp)
	if v, ok := os.LookupEnv("TTD_PORT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v, ok := os.LookupEnv("TTD_STORIES"); ok {
		cfg.IncludeStories = parseBool(v)
	}
	if v, ok := os.LookupEnv("TTD_LIMIT"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DownloadLimit = n
		}
	}
	if v, ok := os.LookupEnv("TTD_ARCHIVE"); ok {
		cfg.UseArchive = parseBool(v)
	}
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y":
		return true
	}
	return false
}
