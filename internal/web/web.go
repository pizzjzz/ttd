// Package web поднимает локальный веб-интерфейс ttd.
package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/installer"
	"ttd/internal/library"
	"ttd/internal/model"
	"ttd/internal/service"
	"ttd/internal/util"
	"ttd/internal/version"
)

//go:embed index.html
var pageHTML []byte

// App — веб-приложение.
type App struct {
	cfg config.Config

	mu      sync.Mutex
	jobs    map[string]*Job
	counter int64
}

// Job — одна задача скачивания, запущенная веб-интерфейсом.
type Job struct {
	ID      string
	URLs    []string
	Created time.Time

	mu      sync.Mutex
	status  string
	events  []model.Event
	results []model.JobResult
	cancel  context.CancelFunc
}

// New создаёт веб-приложение.
func New(cfg config.Config) *App {
	return &App{cfg: cfg, jobs: map[string]*Job{}}
}

// Serve запускает веб-сервер и блокируется до отмены контекста.
func (a *App) Serve(ctx context.Context, host string, port int, launchBrowser bool) int {
	mux := a.Routes()

	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("ttd web: не удалось занять адрес: " + err.Error())
		return 1
	}

	displayHost := host
	if displayHost == "0.0.0.0" || displayHost == "::" || displayHost == "" {
		displayHost = "127.0.0.1"
	}
	url := fmt.Sprintf("http://%s/", net.JoinHostPort(displayHost, strconv.Itoa(port)))

	fmt.Println("ttd " + version.Version + " — веб-интерфейс запущен")
	fmt.Println("Библиотека: " + a.config().MediaPath())
	fmt.Println("Откройте в браузере: " + url)
	fmt.Println("Для остановки нажмите Ctrl-C")

	if launchBrowser {
		go openBrowser(url)
	}

	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Println("ttd web: сервер остановлен: " + err.Error())
		return 1
	}
	return 0
}

// config возвращает копию настроек (безопасно при параллельных запросах).
func (a *App) config() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// Routes собирает маршруты веб-приложения.
func (a *App) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/meta", a.handleMeta)
	mux.HandleFunc("/api/library", a.handleLibrary)
	mux.HandleFunc("/api/files", a.handleFile)
	mux.HandleFunc("/api/download", a.handleDownload)
	mux.HandleFunc("/api/job/", a.handleJob)
	mux.HandleFunc("/api/install", a.handleInstall)
	mux.HandleFunc("/", a.handleIndex)
	return mux
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(pageHTML)
}

func (a *App) handleMeta(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	tools := engines.FindTools(&cfg)
	versions := map[string]string{}
	for _, name := range []string{"gallery-dl", "yt-dlp"} {
		if tools[name] != "" {
			versions[name] = engines.ToolVersion(tools[name])
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":      version.Version,
		"media_root":   cfg.MediaPath(),
		"project_root": config.ProjectRoot(),
		"date":         util.Today(),
		"engine":       cfg.Engine,
		"quality":      cfg.Quality,
		"stories":      cfg.IncludeStories,
		"engines":      config.SupportedEngines,
		"tools":        tools,
		"versions":     versions,
	})
}

func (a *App) handleLibrary(w http.ResponseWriter, r *http.Request) {
	cfg := a.config()
	folders := library.ListFolders(cfg.MediaPath())
	writeJSON(w, http.StatusOK, map[string]any{
		"root":    cfg.MediaPath(),
		"folders": folders,
	})
}

func (a *App) handleFile(w http.ResponseWriter, r *http.Request) {
	folder := r.URL.Query().Get("folder")
	name := r.URL.Query().Get("name")
	path, ok := util.SafeJoin(a.config().MediaPath(), folder, name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if info, err := filepath.EvalSymlinks(path); err == nil {
		path = info
	}
	http.ServeFile(w, r, path)
}

type downloadRequest struct {
	URLs      []string `json:"urls"`
	Text      string   `json:"text"`
	Stories   bool     `json:"stories"`
	Engine    string   `json:"engine"`
	Quality   string   `json:"quality"`
	Date      string   `json:"date"`
	AudioOnly bool     `json:"audio_only"`
	Limit     int      `json:"limit"`
	NoArchive bool     `json:"no_archive"`
}

func (a *App) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "нужен POST", http.StatusMethodNotAllowed)
		return
	}
	var request downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "некорректный JSON: " + err.Error()})
		return
	}
	urls := request.URLs
	if strings.TrimSpace(request.Text) != "" {
		urls = append(urls, strings.Fields(request.Text)...)
	}
	if len(urls) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "не передано ни одной ссылки"})
		return
	}

	job := a.newJob(urls)
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel

	cfg := a.config()
	if request.Engine != "" {
		cfg.Engine = request.Engine
	}
	if request.Quality != "" {
		cfg.Quality = request.Quality
	}
	if request.Stories {
		cfg.IncludeStories = true
	}

	opts := model.DownloadOptions{
		MediaRoot:          cfg.MediaPath(),
		CacheDir:           cfg.CacheDir(),
		Date:               request.Date,
		Engine:             cfg.Engine,
		Quality:            cfg.Quality,
		Cookies:            cfg.Cookies,
		CookiesFromBrowser: cfg.CookiesFromBrowser,
		IncludeStories:     request.Stories,
		Limit:              request.Limit,
		AudioOnly:          request.AudioOnly,
		NoArchive:          request.NoArchive,
	}

	go func() {
		defer cancel()
		if cfg.Engine != "mock" {
			if _, err := installer.EnsureTools(ctx, &cfg, job.Add); err != nil {
				job.Add(model.Event{"type": "error", "error": err.Error()})
			}
		}
		svc := service.New(cfg, job.Add)
		results := svc.Run(ctx, urls, opts)
		job.Finish(results)
	}()

	writeJSON(w, http.StatusOK, map[string]any{"id": job.ID, "urls": urls})
}

func (a *App) handleJob(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/job/")
	rest = strings.Trim(rest, "/")
	if rest == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "не указан идентификатор задачи"})
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]

	a.mu.Lock()
	job := a.jobs[id]
	a.mu.Unlock()
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "задача не найдена"})
		return
	}

	if len(parts) > 1 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			http.Error(w, "нужен POST", http.StatusMethodNotAllowed)
			return
		}
		if job.cancel != nil {
			job.cancel()
		}
		writeJSON(w, http.StatusOK, map[string]any{"cancelled": true})
		return
	}

	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	status, events, results := job.Snapshot(since)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      job.ID,
		"status":  status,
		"events":  events,
		"results": results,
		"next":    since + len(events),
	})
}

func (a *App) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "нужен POST", http.StatusMethodNotAllowed)
		return
	}
	cfg := a.config()
	lines := []string{}
	handler := func(event model.Event) {
		if line, ok := event["line"].(string); ok {
			lines = append(lines, line)
		}
	}
	if err := installer.Bootstrap(r.Context(), &cfg, handler); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error(), "lines": lines})
		return
	}
	tools := engines.FindTools(&cfg)
	a.mu.Lock()
	a.cfg = cfg
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tools": tools, "lines": lines})
}

func (a *App) newJob(urls []string) *Job {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.counter++
	job := &Job{
		ID:      fmt.Sprintf("job-%d", a.counter),
		URLs:    urls,
		Created: time.Now(),
		status:  "running",
	}
	a.jobs[job.ID] = job
	return job
}

// Add добавляет событие в журнал задачи.
func (j *Job) Add(event model.Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.events = append(j.events, event)
	if len(j.events) > 4000 {
		j.events = j.events[len(j.events)-4000:]
	}
}

// Finish помечает задачу завершённой.
func (j *Job) Finish(results []model.JobResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.results = results
	j.status = "done"
}

// Snapshot возвращает состояние задачи и события начиная с индекса since.
func (j *Job) Snapshot(since int) (string, []model.Event, []model.JobResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if since < 0 {
		since = 0
	}
	if since > len(j.events) {
		since = len(j.events)
	}
	events := make([]model.Event, len(j.events)-since)
	copy(events, j.events[since:])
	return j.status, events, j.results
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

func openBrowser(url string) {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"open", url}}
	case "windows":
		candidates = [][]string{{"cmd", "/c", "start", "", url}}
	default:
		candidates = [][]string{{"xdg-open", url}, {"gio", "open", url}}
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate[0]); err == nil {
			_ = exec.Command(candidate[0], candidate[1:]...).Start()
			return
		}
	}
}
