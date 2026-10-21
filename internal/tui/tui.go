// Package tui реализует полноэкранный текстовый интерфейс ttd.
package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/installer"
	"ttd/internal/model"
	"ttd/internal/service"
	"ttd/internal/util"
	"ttd/internal/version"
)

// Статусы элементов очереди.
const (
	stPending   = "ожидает"
	stRunning   = "в работе"
	stOK        = "готово"
	stSkipped   = "пропущено"
	stError     = "ошибка"
	stCancelled = "отменено"
)

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cRed    = "\x1b[31m"
	cCyan   = "\x1b[36m"
	cBlue   = "\x1b[34m"
)

type queueItem struct {
	url    string
	status string
	files  int
	err    string
}

// Tui — состояние интерфейса.
type Tui struct {
	cfg     config.Config
	date    string
	verbose bool

	engine  string
	quality string
	stories bool

	items []*queueItem
	index map[string]*queueItem

	input  []rune
	cursor int

	running bool
	cancel  context.CancelFunc
	results []model.JobResult

	percent     float64
	speed       string
	eta         string
	currentFile string
	currentURL  string

	log    []string
	status string

	raw    *rawState
	keys   chan byte
	events chan model.Event
	out    *bufio.Writer
}

// Run запускает текстовый интерфейс.
func Run(ctx context.Context, cfg config.Config, date string, verbose bool) int {
	if date == "" {
		date = util.Today()
	}
	t := &Tui{
		cfg:     cfg,
		date:    date,
		verbose: verbose,
		engine:  cfg.Engine,
		quality: cfg.Quality,
		stories: cfg.IncludeStories,
		index:   map[string]*queueItem{},
		keys:    make(chan byte, 64),
		events:  make(chan model.Event, 512),
		out:     bufio.NewWriter(os.Stdout),
	}

	raw, err := enterRaw(int(os.Stdin.Fd()))
	if err != nil {
		return t.runFallback(ctx)
	}
	t.raw = raw
	defer func() {
		// Сначала показываем курсор (пока VT ещё включён), затем возвращаем режим.
		fmt.Fprint(t.out, "\x1b[?25h\r\n")
		t.out.Flush()
		raw.restore()
	}()
	fmt.Fprint(t.out, "\x1b[?25l")
	return t.loop(ctx)
}

func (t *Tui) loop(ctx context.Context) int {
	go t.readKeys()
	t.draw()

	for {
		select {
		case <-ctx.Done():
			if t.cancel != nil {
				t.cancel()
			}
			return 0
		case key, ok := <-t.keys:
			if !ok {
				return 0
			}
			if quit := t.handleKey(ctx, key); quit {
				if t.cancel != nil {
					t.cancel()
				}
				return 0
			}
			t.draw()
		case event := <-t.events:
			t.handleEvent(event)
			t.draw()
		}
	}
}

func (t *Tui) readKeys() {
	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			close(t.keys)
			return
		}
		t.keys <- b
	}
}

// handleKey обрабатывает нажатие; возвращает true, если нужно выйти.
func (t *Tui) handleKey(ctx context.Context, key byte) bool {
	switch key {
	case 0x04: // Ctrl-D
		t.start(ctx)
		return false
	case 0x11, 0x03: // Ctrl-Q / Ctrl-C
		return true
	case 0x0c: // Ctrl-L
		t.items = nil
		t.index = map[string]*queueItem{}
		t.status = "очередь очищена"
		return false
	case 0x09: // Tab
		t.stories = !t.stories
		return false
	case 0x1b: // ESC — возможно, последовательность
		return t.handleEscape(ctx, t.collectEscape())
	case 0x7f, 0x08: // Backspace
		if len(t.input) > 0 {
			t.input = t.input[:len(t.input)-1]
		}
		return false
	case 0x0d, 0x0a: // Enter
		t.addInput()
		return false
	}
	if key >= 0x20 && key != 0x7f {
		t.input = append(t.input, rune(key))
	}
	return false
}

func (t *Tui) collectEscape() []byte {
	sequence := []byte{0x1b}
	deadline := time.After(25 * time.Millisecond)
	for {
		select {
		case b, ok := <-t.keys:
			if !ok {
				return sequence
			}
			sequence = append(sequence, b)
			if len(sequence) >= 3 && sequence[1] == '[' && b >= 0x40 && b <= 0x7e {
				return sequence
			}
			if len(sequence) >= 6 {
				return sequence
			}
		case <-deadline:
			return sequence
		}
	}
}

// handleEscape обрабатывает управляющие последовательности; true — выход.
func (t *Tui) handleEscape(ctx context.Context, sequence []byte) bool {
	text := string(sequence)
	switch {
	case strings.HasSuffix(text, "[15~"): // F5
		t.start(ctx)
	case strings.HasSuffix(text, "[21~"): // F10
		return true
	}
	return false
}

// addInput разбирает введённую строку и добавляет ссылки в очередь.
func (t *Tui) addInput() {
	line := strings.TrimSpace(string(t.input))
	t.input = nil
	if line == "" {
		return
	}
	added := 0
	for _, field := range strings.Fields(line) {
		url := engines.NormalizeURL(field)
		if url == "" {
			continue
		}
		if _, exists := t.index[url]; exists {
			continue
		}
		item := &queueItem{url: url, status: stPending}
		t.items = append(t.items, item)
		t.index[url] = item
		added++
	}
	if added > 0 {
		t.status = fmt.Sprintf("добавлено ссылок: %d", added)
	} else {
		t.status = "ссылок не добавлено"
	}
}

func (t *Tui) start(ctx context.Context) {
	if t.running {
		t.status = "загрузка уже идёт"
		return
	}
	var urls []string
	for _, item := range t.items {
		if item.status == stPending {
			urls = append(urls, item.url)
		}
	}
	if len(urls) == 0 {
		t.status = "нет новых ссылок в очереди"
		return
	}
	t.running = true
	t.results = nil
	t.log = nil
	t.percent = 0
	t.speed = ""
	t.eta = ""
	t.currentFile = ""
	t.status = "загрузка начата"

	runCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	cfg := t.cfg
	opts := model.DownloadOptions{
		MediaRoot:          cfg.MediaPath(),
		CacheDir:           cfg.CacheDir(),
		Date:               t.date,
		Engine:             t.engine,
		Quality:            t.quality,
		Cookies:            cfg.Cookies,
		CookiesFromBrowser: cfg.CookiesFromBrowser,
		IncludeStories:     t.stories,
		Limit:              cfg.DownloadLimit,
		Verbose:            t.verbose,
		NoArchive:          !cfg.UseArchive,
	}

	go func() {
		if cfg.Engine != "mock" {
			if _, err := installer.EnsureTools(runCtx, &cfg, t.push); err != nil {
				t.push(model.Event{"type": "error", "error": err.Error()})
			}
		}
		svc := service.New(cfg, t.push)
		results := svc.Run(runCtx, urls, opts)
		t.events <- model.Event{"type": "finished", "results": results}
	}()
}

func (t *Tui) push(event model.Event) {
	select {
	case t.events <- event:
	default:
	}
}

func (t *Tui) handleEvent(event model.Event) {
	kind, _ := event["type"].(string)
	switch kind {
	case "start":
		t.appendLog(fmt.Sprintf("начало: ссылок %d, папка-дата %s", asInt(event["count"]), asString(event["date"])))
	case "job":
		url := asString(event["url"])
		if item := t.index[url]; item != nil {
			item.status = stRunning
		}
		t.currentURL = url
		t.percent = 0
		t.speed = ""
		t.eta = ""
		t.status = "движок: " + asString(event["engine"])
	case "progress":
		t.percent = asFloat(event["percent"])
		t.speed = asString(event["speed"])
		t.eta = asString(event["eta"])
	case "file":
		if file, ok := event["file"].(model.MediaItem); ok {
			t.currentFile = file.Name
			if item := t.index[t.currentURL]; item != nil {
				item.files++
			}
			t.appendLog("скачано: " + file.Name + " (" + util.HumanSize(file.Size) + ")")
		}
	case "saved":
		t.appendLog("сохранено в " + asString(event["folder"]))
	case "log":
		if t.verbose {
			t.appendLog(asString(event["line"]))
		}
	case "install":
		t.appendLog(asString(event["line"]))
	case "error":
		message := asString(event["error"])
		if message == "" {
			message = asString(event["line"])
		}
		if message != "" {
			t.appendLog("ошибка: " + message)
			t.status = message
		}
	case "result":
		url := asString(event["url"])
		if item := t.index[url]; item != nil {
			item.status = tuiStatus(asString(event["status"]))
			item.err = asString(event["error"])
			if files, ok := event["files"].([]model.MediaItem); ok {
				item.files = len(files)
			}
		}
	case "done":
		t.appendLog(fmt.Sprintf("итог: файлов %d, объём %s", asInt(event["files"]), util.HumanSize(asInt64(event["size"]))))
	case "finished":
		t.running = false
		t.currentFile = ""
		if results, ok := event["results"].([]model.JobResult); ok {
			t.results = results
			t.status = service.FormatDone(results)
		}
		t.appendLog("загрузка завершена")
	}
}

func (t *Tui) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	t.log = append(t.log, line)
	if len(t.log) > 500 {
		t.log = t.log[len(t.log)-500:]
	}
}

func tuiStatus(status string) string {
	switch status {
	case model.StatusOK:
		return stOK
	case model.StatusSkipped:
		return stSkipped
	case model.StatusCancelled:
		return stCancelled
	case model.StatusError:
		return stError
	default:
		return status
	}
}

// draw перерисовывает экран.
func (t *Tui) draw() {
	width, height := termSize(int(os.Stdin.Fd()))
	middle := height - 12
	if middle < 2 {
		middle = 2
	}
	queueMax := middle / 2
	if queueMax < 1 {
		queueMax = 1
	}
	logMax := middle - queueMax

	lines := make([]string, 0, height)
	inputRow := 0

	title := cBold + cCyan + "ttd " + versionLabel + " — загрузчик TikTok" + cReset
	lines = append(lines, title)
	lines = append(lines, cDim+fmt.Sprintf("Библиотека: %s    Дата: %s    Движок: %s    Качество: %s    Истории: %s",
		t.cfg.MediaPath(), t.date, t.engine, t.quality, onOff(t.stories))+cReset)
	lines = append(lines, strings.Repeat("─", width))

	header := fmt.Sprintf("Очередь: %d", len(t.items))
	if t.status != "" {
		header += "    " + t.status
	}
	lines = append(lines, cBold+header+cReset)

	if len(t.items) == 0 {
		lines = append(lines, cDim+"  введите ссылку TikTok ниже и нажмите Enter"+cReset)
		for i := 1; i < queueMax; i++ {
			lines = append(lines, "")
		}
	} else {
		start := 0
		if len(t.items) > queueMax {
			start = len(t.items) - queueMax
		}
		if start > 0 {
			lines = append(lines, cDim+fmt.Sprintf("  … ещё %d выше", start)+cReset)
			for i := 1; i < queueMax; i++ {
				lines = append(lines, t.queueLine(t.items[start+i-1], width))
			}
		} else {
			for i := 0; i < queueMax; i++ {
				if i < len(t.items) {
					lines = append(lines, t.queueLine(t.items[i], width))
				} else {
					lines = append(lines, "")
				}
			}
		}
	}

	progress := "Прогресс: —"
	if t.running {
		progress = fmt.Sprintf("Прогресс: %.0f%%", t.percent)
		if t.speed != "" {
			progress += "  скорость " + t.speed
		}
		if t.eta != "" {
			progress += "  осталось " + t.eta
		}
	} else if len(t.results) > 0 {
		progress = service.FormatDone(t.results)
	}
	lines = append(lines, cGreen+progress+cReset)

	fileLine := "Файл: —"
	if t.currentFile != "" {
		fileLine = "Файл: " + t.currentFile
	}
	lines = append(lines, fileLine)

	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, cBold+"Журнал:"+cReset)

	startLog := 0
	if len(t.log) > logMax {
		startLog = len(t.log) - logMax
	}
	for i := 0; i < logMax; i++ {
		if startLog+i < len(t.log) {
			lines = append(lines, cDim+fit(t.log[startLog+i], width)+cReset)
		} else {
			lines = append(lines, "")
		}
	}

	lines = append(lines, cCyan+"┌ Ввод ссылки "+strings.Repeat("─", maxInt(width-16, 0))+cReset)
	inputRow = len(lines)
	lines = append(lines, cCyan+"│ "+cReset+string(t.input))
	lines = append(lines, cCyan+"└"+strings.Repeat("─", maxInt(width-1, 0))+cReset)

	footer := "F5/Ctrl-D начать   F10/Ctrl-Q выход   Tab истории   Ctrl-L очистить   Enter добавить"
	lines = append(lines, cBold+cBlue+fit(footer, width)+cReset)

	var builder strings.Builder
	builder.WriteString("\x1b[2J\x1b[H")
	for i, line := range lines {
		if i > 0 {
			builder.WriteString("\r\n")
		}
		builder.WriteString(line)
	}
	// Ставим курсор на строку ввода.
	column := 3 + len([]rune(string(t.input)))
	builder.WriteString(fmt.Sprintf("\x1b[%d;%dH\x1b[?25h", inputRow+1, column))
	// Скрываем курсор для следующих перерисовок.
	builder.WriteString("\x1b[?25l")

	t.out.WriteString(builder.String())
	t.out.Flush()
}

func (t *Tui) queueLine(item *queueItem, width int) string {
	marker, color := statusMarker(item.status)
	text := fmt.Sprintf("  %s %s", marker, item.url)
	if item.files > 0 {
		text += fmt.Sprintf("  (файлов: %d)", item.files)
	}
	if item.err != "" && item.status == stError {
		text += "  " + item.err
	}
	return color + fit(text, width) + cReset
}

func statusMarker(status string) (string, string) {
	switch status {
	case stOK:
		return "[ok ]", cGreen
	case stRunning:
		return "[>>>]", cYellow
	case stSkipped:
		return "[skip]", cDim
	case stError:
		return "[err ]", cRed
	case stCancelled:
		return "[stop]", cRed
	default:
		return "[   ]", ""
	}
}

func fit(text string, width int) string {
	runes := []rune(text)
	if width <= 0 {
		return ""
	}
	if len(runes) > width {
		return string(runes[:width])
	}
	return text + strings.Repeat(" ", width-len(runes))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func onOff(value bool) string {
	if value {
		return "вкл"
	}
	return "выкл"
}

func asString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func asInt(value any) int {
	return int(asInt64(value))
}

func asInt64(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	}
	return 0
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	}
	return 0
}

// runFallback — режим без управления терминалом: чтение ссылок строками.
func (t *Tui) runFallback(ctx context.Context) int {
	fmt.Println("Терминал не поддерживает полноэкранный режим — простой режим.")
	fmt.Println("Введите ссылки TikTok (по одной в строке). Пустая строка — начать загрузку.")
	scanner := bufio.NewScanner(os.Stdin)
	var urls []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}
		urls = append(urls, line)
	}
	if len(urls) == 0 {
		fmt.Println("ссылок нет — выход")
		return 0
	}
	cfg := t.cfg
	reporter := plainReporter
	opts := model.DownloadOptions{
		MediaRoot:          cfg.MediaPath(),
		CacheDir:           cfg.CacheDir(),
		Date:               t.date,
		Engine:             t.engine,
		Quality:            t.quality,
		IncludeStories:     t.stories,
		Limit:              cfg.DownloadLimit,
		Cookies:            cfg.Cookies,
		CookiesFromBrowser: cfg.CookiesFromBrowser,
	}
	if cfg.Engine != "mock" {
		if _, err := installer.EnsureTools(ctx, &cfg, reporter); err != nil {
			fmt.Fprintln(os.Stderr, "ttd: "+err.Error())
		}
	}
	svc := service.New(cfg, reporter)
	results := svc.Run(ctx, urls, opts)
	fmt.Println()
	fmt.Println(service.FormatDone(results))
	return 0
}

// versionLabel — версия для заголовка (не импортируем cli, чтобы избежать цикла).
const versionLabel = version.Version

// plainReporter — простой вывод событий для режима без полноэкранного интерфейса.
func plainReporter(event model.Event) {
	switch asString(event["type"]) {
	case "start":
		fmt.Printf("Загрузка %d ссылок в %s (движок %s)\n", asInt(event["count"]), asString(event["root"]), asString(event["engine"]))
	case "job":
		fmt.Printf("[%d/%d] %s\n", asInt(event["index"]), asInt(event["total"]), asString(event["url"]))
	case "log":
		fmt.Println("  · " + asString(event["line"]))
	case "install":
		fmt.Println("  · " + asString(event["line"]))
	case "progress":
		fmt.Printf("  загрузка %.0f%% %s %s\n", asFloat(event["percent"]), asString(event["speed"]), asString(event["eta"]))
	case "file":
		if file, ok := event["file"].(model.MediaItem); ok {
			fmt.Printf("  + %s (%s)\n", file.Name, util.HumanSize(file.Size))
		}
	case "result":
		fmt.Printf("  итог: %s\n", asString(event["status"]))
	case "error":
		message := asString(event["error"])
		if message == "" {
			message = asString(event["line"])
		}
		fmt.Println("  ошибка: " + message)
	case "done":
		fmt.Printf("  файлов %d, объём %s\n", asInt(event["files"]), util.HumanSize(asInt64(event["size"])))
	}
}
