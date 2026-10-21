// Package cli реализует командный интерфейс ttd.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/installer"
	"ttd/internal/library"
	"ttd/internal/model"
	"ttd/internal/service"
	"ttd/internal/tui"
	"ttd/internal/util"
	"ttd/internal/version"
	"ttd/internal/web"
)

// Version — версия приложения.
const Version = version.Version

// Main — точка входа: настраивает отмену по Ctrl-C и вызывает Run.
func Main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return Run(ctx, os.Args[1:])
}

// Run выполняет команду и возвращает код выхода.
func Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}
	command := args[0]
	rest := args[1:]

	switch command {
	case "help", "--help", "-h":
		printUsage()
		return 0
	case "version", "--version", "-V":
		fmt.Println("ttd " + Version)
		return 0
	case "download", "dl", "get", "d":
		return cmdDownload(ctx, rest)
	case "probe", "info", "p":
		return cmdProbe(ctx, rest)
	case "list", "ls", "l":
		return cmdList(rest)
	case "tui":
		return cmdTUI(ctx, rest)
	case "web", "serve", "server", "ui":
		return cmdWeb(ctx, rest)
	case "install", "setup":
		return cmdInstall(ctx, rest)
	case "doctor", "check":
		return cmdDoctor(rest)
	case "config", "settings":
		return cmdConfig(rest)
	default:
		// Ссылки без подкоманды трактуются как скачивание.
		return cmdDownload(ctx, args)
	}
}

// commonFlags — флаги, общие для скачивания, веб-сервера и TUI.
type commonFlags struct {
	dir                string
	engine             string
	quality            string
	cookies            string
	cookiesFromBrowser string
	date               string
	stories            bool
	limit              int
	noArchive          bool
	quiet              bool
	jsonOut            bool
	verbose            bool
}

func registerCommon(fs *flagSet, cf *commonFlags) {
	fs.String(&cf.dir, "dir", "d", "output")
	fs.String(&cf.engine, "engine", "e")
	fs.String(&cf.quality, "quality", "q")
	fs.String(&cf.cookies, "cookies")
	fs.String(&cf.cookiesFromBrowser, "cookies-from-browser")
	fs.String(&cf.date, "date")
	fs.BoolPair(&cf.stories, "stories", "no-stories")
	fs.Int(&cf.limit, "limit", "n")
	fs.Bool(&cf.noArchive, "no-archive")
	fs.Bool(&cf.quiet, "quiet", "Q")
	fs.Bool(&cf.jsonOut, "json", "j")
	fs.Bool(&cf.verbose, "verbose", "v")
}

func (cf *commonFlags) apply(cfg *config.Config, fs *flagSet) {
	if fs.Changed("dir") {
		cfg.MediaRoot = util.ExpandHome(cf.dir)
	}
	if fs.Changed("engine") {
		cfg.Engine = cf.engine
	}
	if fs.Changed("quality") {
		cfg.Quality = cf.quality
	}
	if fs.Changed("cookies") {
		cfg.Cookies = cf.cookies
	}
	if fs.Changed("cookies-from-browser") {
		cfg.CookiesFromBrowser = cf.cookiesFromBrowser
	}
	if fs.Changed("stories") || fs.Changed("no-stories") {
		cfg.IncludeStories = cf.stories
	}
	if fs.Changed("limit") {
		cfg.DownloadLimit = cf.limit
	}
	if fs.Changed("no-archive") {
		cfg.UseArchive = !cf.noArchive
	}
}

func loadConfigWith(cf *commonFlags, fs *flagSet) config.Config {
	cfg := config.Load()
	cf.apply(&cfg, fs)
	return cfg
}

func cmdDownload(ctx context.Context, args []string) int {
	fs := newFlagSet("download")
	var cf commonFlags
	registerCommon(fs, &cf)
	var dryRun, audioOnly, installNow bool
	fs.Bool(&dryRun, "dry-run", "plan")
	fs.Bool(&audioOnly, "audio-only", "audio")
	fs.Bool(&installNow, "install")
	var engineArgs []string
	fs.List(&engineArgs, "engine-arg")

	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		printDownloadHelp()
		return 2
	}
	urls := fs.Positionals()
	if len(urls) == 0 {
		errorf("не указана ни одна ссылка TikTok")
		printDownloadHelp()
		return 2
	}

	cfg := loadConfigWith(&cf, fs)

	if installNow {
		if _, err := installer.BootstrapAll(ctx, &cfg); err != nil {
			errorf("установка движков не удалась: %v", err)
		}
	} else if cfg.Engine != "mock" && !dryRun {
		reporter := NewReporter(os.Stdout, cf.verbose, cf.quiet)
		if _, err := installer.EnsureTools(ctx, &cfg, reporter.Handle); err != nil {
			errorf("движки недоступны: %v", err)
		}
	}

	opts := model.DownloadOptions{
		MediaRoot:          cfg.MediaPath(),
		CacheDir:           cfg.CacheDir(),
		Date:               cf.date,
		Engine:             cfg.Engine,
		Quality:            cfg.Quality,
		Cookies:            cfg.Cookies,
		CookiesFromBrowser: cfg.CookiesFromBrowser,
		IncludeStories:     cfg.IncludeStories,
		Limit:              cfg.DownloadLimit,
		AudioOnly:          audioOnly,
		DryRun:             dryRun,
		Verbose:            cf.verbose,
		NoArchive:          !cfg.UseArchive,
		ExtraArgs:          engineArgs,
	}

	var handler func(model.Event)
	if !cf.jsonOut {
		handler = NewReporter(os.Stdout, cf.verbose, cf.quiet).Handle
	}

	svc := service.New(cfg, handler)
	results := svc.Run(ctx, urls, opts)

	if cf.jsonOut {
		return printResultsJSON(results)
	}
	fmt.Println()
	fmt.Println(service.FormatDone(results))
	return exitCode(ctx, results)
}

func cmdProbe(ctx context.Context, args []string) int {
	fs := newFlagSet("probe")
	var cf commonFlags
	fs.String(&cf.engine, "engine", "e")
	fs.String(&cf.cookies, "cookies")
	fs.String(&cf.cookiesFromBrowser, "cookies-from-browser")
	fs.String(&cf.dir, "dir", "d")
	fs.Bool(&cf.jsonOut, "json", "j")

	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	urls := fs.Positionals()
	if len(urls) == 0 {
		errorf("укажите хотя бы одну ссылку TikTok")
		return 2
	}
	cfg := config.Load()
	cf.apply(&cfg, fs)

	results := make([]model.ProbeResult, 0, len(urls))
	for _, url := range urls {
		result := engines.Probe(ctx, url, &cfg)
		results = append(results, result)
		if !cf.jsonOut {
			printProbeResult(result)
		}
	}
	if cf.jsonOut {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	failed := 0
	for _, result := range results {
		if result.Error != "" {
			failed++
		}
	}
	if failed > 0 {
		return 1
	}
	return 0
}

func cmdList(args []string) int {
	fs := newFlagSet("list")
	var dir, dateFilter string
	var jsonOut, showFiles bool
	fs.String(&dir, "dir", "d")
	fs.String(&dateFilter, "date")
	fs.Bool(&jsonOut, "json", "j")
	fs.Bool(&showFiles, "files", "f")

	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	cfg := config.Load()
	if dir != "" {
		cfg.MediaRoot = util.ExpandHome(dir)
	}
	root := cfg.MediaPath()

	folders := library.ListFolders(root)
	if dateFilter != "" {
		folders = filterByDate(folders, dateFilter)
	}
	if jsonOut {
		data, _ := json.MarshalIndent(folders, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	if len(folders) == 0 {
		fmt.Println("Библиотека пуста: " + root)
		return 0
	}
	fmt.Println("Библиотека: " + root)
	fmt.Println()
	var totalFiles int
	var totalSize int64
	for _, folder := range folders {
		fmt.Printf("%s  (видео: %d, фото: %d, аудио: %d, %s)\n",
			folder.Name, folder.Videos, folder.Photos, folder.Audio, util.HumanSize(folder.Size))
		if showFiles {
			for _, file := range folder.Files {
				fmt.Printf("    %-50s %10s\n", file.Name, util.HumanSize(file.Size))
			}
		}
		totalFiles += folder.Count
		totalSize += folder.Size
	}
	fmt.Println()
	fmt.Printf("Всего: папок %d, файлов %d, объём %s\n", len(folders), totalFiles, util.HumanSize(totalSize))
	return 0
}

func cmdTUI(ctx context.Context, args []string) int {
	fs := newFlagSet("tui")
	var cf commonFlags
	registerCommon(fs, &cf)
	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	cfg := loadConfigWith(&cf, fs)
	return tui.Run(ctx, cfg, cf.date, cf.verbose)
}

func cmdWeb(ctx context.Context, args []string) int {
	fs := newFlagSet("web")
	var cf commonFlags
	registerCommon(fs, &cf)
	var host string
	var port int
	var noBrowser bool
	fs.String(&host, "host", "H")
	fs.Int(&port, "port", "p")
	fs.Bool(&noBrowser, "no-browser")

	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	cfg := loadConfigWith(&cf, fs)
	if host == "" {
		host = cfg.Host
	}
	if port == 0 {
		port = cfg.Port
	}
	app := web.New(cfg)
	return app.Serve(ctx, host, port, !noBrowser)
}

func cmdInstall(ctx context.Context, args []string) int {
	fs := newFlagSet("install")
	var dir string
	var online bool
	var wheelsDir string
	var help bool
	fs.String(&dir, "dir", "d")
	fs.Bool(&online, "online")
	fs.String(&wheelsDir, "wheels")
	fs.Bool(&help, "help", "h")
	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	if help {
		printInstallHelp()
		return 0
	}
	cfg := config.Load()
	if dir != "" {
		cfg.MediaRoot = util.ExpandHome(dir)
	}
	reporter := NewReporter(os.Stdout, true, false)
	if err := installer.BootstrapWith(ctx, &cfg, reporter.Handle, installer.Options{
		Online:    online,
		WheelsDir: wheelsDir,
	}); err != nil {
		errorf("установка не удалась: %v", err)
		return 1
	}
	tools := engines.FindTools(&cfg)
	fmt.Println()
	for _, line := range installer.Doctor(&cfg) {
		fmt.Println(line)
	}
	if tools["gallery-dl"] == "" || tools["yt-dlp"] == "" {
		return 1
	}
	return 0
}

func cmdDoctor(args []string) int {
	fs := newFlagSet("doctor")
	var dir string
	fs.String(&dir, "dir", "d")
	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	cfg := config.Load()
	if dir != "" {
		cfg.MediaRoot = util.ExpandHome(dir)
	}
	for _, line := range installer.Doctor(&cfg) {
		fmt.Println(line)
	}
	return 0
}

func cmdConfig(args []string) int {
	fs := newFlagSet("config")
	var pathOnly bool
	fs.Bool(&pathOnly, "path")
	if err := fs.parse(args); err != nil {
		errorf("%v", err)
		return 2
	}
	path := config.ConfigFile()
	if pathOnly {
		fmt.Println(path)
		return 0
	}
	fmt.Println("# файл настроек: " + path)
	cfg := config.Load()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(data))
	return 0
}

// printProbeResult печатает результат анализа в человекочитаемом виде.
func printProbeResult(result model.ProbeResult) {
	fmt.Println("Ссылка:  " + result.URL)
	fmt.Println("Вид:     " + result.Kind)
	if result.Title != "" {
		fmt.Println("Название: " + result.Title)
	}
	if result.Uploader != "" {
		fmt.Println("Автор:   " + result.Uploader)
	}
	if result.Duration > 0 {
		fmt.Printf("Длительность: %.1f с\n", result.Duration)
	}
	if result.IsPlaylist {
		fmt.Printf("Это список/профиль: записей %d\n", result.EntryCount)
	} else if result.Best != nil {
		fmt.Printf("Лучшее качество: %s, %s", result.Best.Resolution, result.Best.Ext)
		if result.Best.TBR > 0 {
			fmt.Printf(", %.0f кбит/с", result.Best.TBR)
		}
		if result.Best.FileSize > 0 {
			fmt.Printf(", %s", util.HumanSize(result.Best.FileSize))
		}
		fmt.Println()
	}
	if len(result.Formats) > 0 && !result.IsPlaylist {
		fmt.Println("Доступные форматы:")
		formats := append([]model.ProbeFormat(nil), result.Formats...)
		sort.SliceStable(formats, func(i, j int) bool { return formats[i].Height > formats[j].Height })
		for _, format := range formats {
			if format.VCodec == "none" {
				continue
			}
			fmt.Printf("  %-12s %-5s %-8s %-8s", format.Resolution, format.Ext, format.VCodec, format.ACodec)
			if format.TBR > 0 {
				fmt.Printf(" %.0fk", format.TBR)
			}
			fmt.Println()
		}
	}
	if result.Error != "" {
		fmt.Println("Ошибка:  " + result.Error)
	}
	fmt.Println()
}

func printResultsJSON(results []model.JobResult) int {
	data, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(data))
	errs := 0
	for _, result := range results {
		if result.Status == model.StatusError {
			errs++
		}
	}
	if errs > 0 {
		return 1
	}
	return 0
}

func exitCode(ctx context.Context, results []model.JobResult) int {
	if ctx.Err() != nil {
		return 130
	}
	for _, result := range results {
		if result.Status == model.StatusError {
			return 1
		}
	}
	return 0
}

func filterByDate(folders []model.Folder, date string) []model.Folder {
	filtered := make([]model.Folder, 0, len(folders))
	for _, folder := range folders {
		if folder.DownloadDate == date || folder.PublishDate == date {
			filtered = append(filtered, folder)
		}
	}
	return filtered
}

func errorf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ttd: "+format+"\n", a...)
}

func printUsage() {
	fmt.Println(fmt.Sprintf("ttd %s — загрузчик видео и фото из TikTok", version.Version) + `

Использование:
  ttd <ссылка> [ссылки...]              скачать (то же, что ttd download)
  ttd download [флаги] <ссылки...>      скачать видео, фото, профиль, истории
  ttd probe <ссылка>                    показать доступное качество
  ttd list [--files]                    показать библиотеку
  ttd tui                               текстовый интерфейс
  ttd web [--port 8787]                 поднять сайт на localhost
  ttd install                           поставить движки (встроены, без интернета)
  ttd doctor                            проверить окружение
  ttd config                            показать настройки
  ttd version                           версия

Куда сохраняется:
  ~/ttd/<ДАТА СКАЧИВАНИЯ> | <ДАТА ПУБЛИКАЦИЙ>/...
  например: ~/ttd/19.10.2026 | 17.10.2026/user_7412345678.mp4
  в Windows разделитель "-" (символ "|" запрещён в именах файлов):
  ~/ttd/19.10.2026 - 17.10.2026/user_7412345678.mp4

Подробности: ttd download --help
`)
}

func printDownloadHelp() {
	fmt.Print(`ttd download — скачивание из TikTok

Использование:
  ttd download [флаги] <ссылки...>

Флаги:
  -d, --dir <путь>            каталог библиотеки (по умолчанию ~/ttd)
  -e, --engine <движок>       auto | gallery-dl | yt-dlp | mock
  -q, --quality <качество>    best | 1080 | 720 | 480 (по умолчанию best)
      --date <ДД.ММ.ГГГГ>     дата скачивания для папки (по умолчанию сегодня)
      --stories               добавить истории профиля
      --no-stories            не скачивать истории (по умолчанию)
  -n, --limit <число>         ограничить число записей
      --audio-only            скачать только звук (mp3)
      --cookies <файл>        cookies в формате Netscape
      --cookies-from-browser <браузер>
                              взять cookies из браузера (firefox, chrome, ...)
      --no-archive            не пропускать уже скачанное
      --engine-arg <аргумент> дополнительный аргумент движка (можно повторять)
      --dry-run, --plan       показать план без скачивания
      --install               принудительно переустановить движки
                              (из встроенного набора, без интернета)
  -j, --json                  вывести результат в JSON
  -v, --verbose               подробный журнал
  -Q, --quiet                 без промежуточного вывода
  -h, --help                  эта справка
`)
}

func printInstallHelp() {
	fmt.Print(`ttd install — установка движков (gallery-dl, yt-dlp)

По умолчанию движки и их зависимости ставятся из встроенного набора:
интернет для этого не нужен. Уже установленное окружение не пересоздаётся.

Флаги:
      --online                обновить движки с PyPI (нужен интернет)
      --wheels <каталог>      поставить из локального каталога с .whl
  -d, --dir <путь>            каталог библиотеки
  -h, --help                  эта справка

Примеры:
  ttd install
  ttd install --online
  ttd install --wheels ~/ttd-wheels
`)
}
