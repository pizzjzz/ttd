// Package service связывает движки, библиотеку и вывод прогресса.
package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/library"
	"ttd/internal/model"
	"ttd/internal/util"
)

// StagingDirName — служебный каталог для промежуточной загрузки.
const StagingDirName = ".staging"

// Service выполняет скачивание и раскладку файлов по папкам-датам.
type Service struct {
	Config  config.Config
	Engines map[string]engines.Engine
	Tools   map[string]string
	OnEvent func(model.Event)
}

// New создаёт сервис по настройкам.
func New(cfg config.Config, onEvent func(model.Event)) *Service {
	tools := engines.FindTools(&cfg)
	return &Service{
		Config:  cfg,
		Engines: engines.BuildEngines(&cfg, tools),
		Tools:   tools,
		OnEvent: onEvent,
	}
}

// Plan разворачивает ссылки, добавляя истории для профилей.
func (s *Service) Plan(urls []string, includeStories bool) []model.PlannedURL {
	return engines.ExpandURLs(urls, includeStories)
}

// Run скачивает все ссылки и возвращает результаты по каждой.
func (s *Service) Run(ctx context.Context, urls []string, opts model.DownloadOptions) []model.JobResult {
	emit := engines.EventFunc(s.OnEvent)

	if opts.Date == "" {
		opts.Date = util.Today()
	}
	if opts.MediaRoot == "" {
		opts.MediaRoot = s.Config.MediaPath()
	}
	if opts.CacheDir == "" {
		opts.CacheDir = s.Config.CacheDir()
	}
	if opts.Engine == "" {
		opts.Engine = s.Config.Engine
	}
	if opts.Quality == "" {
		opts.Quality = s.Config.Quality
	}

	planned := s.Plan(urls, opts.IncludeStories)

	emit(model.Event{
		"type": "start", "count": len(planned), "date": opts.Date,
		"root": opts.MediaRoot, "engine": opts.Engine, "dry_run": opts.DryRun,
	})

	if !opts.DryRun {
		if err := os.MkdirAll(opts.MediaRoot, 0o755); err != nil {
			emit(model.Event{"type": "error", "error": err.Error()})
		}
		if opts.CacheDir != "" && !opts.NoArchive {
			_ = os.MkdirAll(opts.CacheDir, 0o755)
		}
	}

	results := make([]model.JobResult, 0, len(planned))
	for index, item := range planned {
		if ctx.Err() != nil {
			results = append(results, model.JobResult{
				URL: item.URL, Kind: item.Kind, Status: model.StatusCancelled, Error: "отменено",
			})
			break
		}
		result := s.runOne(ctx, index, len(planned), item, opts)
		results = append(results, result)

		emit(model.Event{
			"type": "result", "index": index + 1, "total": len(planned),
			"url": result.URL, "kind": result.Kind, "status": result.Status,
			"engine": result.Engine, "files": result.Files, "error": result.Error,
		})
	}

	total, size := summarize(results)
	emit(model.Event{
		"type": "done", "total": len(results), "files": total, "size": size,
		"ok":        countStatus(results, model.StatusOK),
		"skipped":   countStatus(results, model.StatusSkipped),
		"errors":    countStatus(results, model.StatusError),
		"cancelled": countStatus(results, model.StatusCancelled),
		"folder":    library.FolderName(opts.Date, opts.Date),
	})
	return results
}

func (s *Service) runOne(ctx context.Context, index, total int, item model.PlannedURL, opts model.DownloadOptions) model.JobResult {
	emit := engines.EventFunc(s.OnEvent)
	result := model.JobResult{URL: item.URL, Kind: item.Kind, Status: model.StatusError}

	order := engines.EngineOrder(item.Kind, opts.Engine)
	available := make([]string, 0, len(order))
	for _, name := range order {
		engine, ok := s.Engines[name]
		if ok && engine.Available() {
			available = append(available, name)
		}
	}
	if len(available) == 0 {
		result.Error = "нет доступных движков для ссылки: " + strings.Join(order, ", ")
		emit(model.Event{"type": "error", "url": item.URL, "error": result.Error})
		return result
	}

	var lastError string
	for _, name := range available {
		if ctx.Err() != nil {
			result.Status = model.StatusCancelled
			result.Error = "отменено"
			return result
		}
		engine := s.Engines[name]
		emit(model.Event{
			"type": "job", "index": index + 1, "total": total,
			"url": item.URL, "kind": item.Kind, "engine": name,
		})

		staging := filepath.Join(opts.MediaRoot, StagingDirName, fmt.Sprintf("job-%d-%d", index+1, time.Now().UnixNano()))
		if !opts.DryRun {
			if err := os.MkdirAll(staging, 0o755); err != nil {
				lastError = err.Error()
				continue
			}
		}

		outcome := engine.Download(ctx, item.URL, opts, staging, s.OnEvent)

		if outcome.Cancelled {
			cleanup(staging)
			result.Status = model.StatusCancelled
			result.Engine = name
			result.Error = "отменено"
			return result
		}

		files, placeErr := s.place(opts, staging)
		if placeErr != nil {
			lastError = placeErr.Error()
			cleanup(staging)
			continue
		}
		cleanup(staging)

		if !outcome.OK {
			lastError = outcome.Error
			if lastError == "" {
				lastError = "движок " + name + " завершился с ошибкой"
			}
			continue
		}

		result.Engine = name
		result.Files = files
		if len(files) == 0 {
			result.Status = model.StatusSkipped
		} else {
			result.Status = model.StatusOK
		}
		return result
	}

	result.Error = lastError
	if result.Error == "" {
		result.Error = "не удалось скачать"
	}
	return result
}

// place переносит скачанные файлы из промежуточного каталога в папки-даты.
func (s *Service) place(opts model.DownloadOptions, staging string) ([]model.MediaItem, error) {
	items := []model.MediaItem{}
	entries, err := os.ReadDir(staging)
	if err != nil {
		return items, nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if util.IsTempFile(name) || util.IsSidecar(name) {
			continue
		}
		source := filepath.Join(staging, name)
		publish := opts.Date
		if when, ok := engines.ReadPublishDate(source); ok {
			publish = util.FormatDate(when)
		}
		folder := library.FolderName(opts.Date, publish)
		targetDir := filepath.Join(opts.MediaRoot, folder)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return items, err
		}
		target := util.UniquePath(targetDir, name)
		if err := moveFile(source, target); err != nil {
			return items, err
		}
		item := model.MediaItem{
			Path:    target,
			Name:    filepath.Base(target),
			Kind:    model.KindOf(target),
			URL:     "",
			ModTime: time.Now(),
		}
		if info, err := os.Stat(target); err == nil {
			item.Size = info.Size()
			item.ModTime = info.ModTime()
		}
		items = append(items, item)
		engines.EventFunc(s.OnEvent)(model.Event{
			"type": "saved", "folder": folder, "file": item,
		})
	}
	return items, nil
}

func cleanup(staging string) {
	if strings.TrimSpace(staging) == "" {
		return
	}
	_ = os.RemoveAll(staging)
	// Убираем пустой родительский .staging.
	_ = os.Remove(filepath.Dir(staging))
}

func moveFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	// Запасной вариант — копирование между файловыми системами.
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(source)
}

// summarize считает файлы и их суммарный размер.
func summarize(results []model.JobResult) (int, int64) {
	total := 0
	var size int64
	for _, result := range results {
		total += len(result.Files)
		for _, file := range result.Files {
			size += file.Size
		}
	}
	return total, size
}

func countStatus(results []model.JobResult, status string) int {
	count := 0
	for _, result := range results {
		if result.Status == status {
			count++
		}
	}
	return count
}

// Summarize возвращает сводку по результатам.
func Summarize(results []model.JobResult) (files int, size int64, ok, skipped, errors int) {
	files, size = summarize(results)
	return files, size, countStatus(results, model.StatusOK), countStatus(results, model.StatusSkipped), countStatus(results, model.StatusError)
}

// FormatDone собирает текст итоговой строки.
func FormatDone(results []model.JobResult) string {
	files, size, ok, skipped, errors := Summarize(results)
	var parts []string
	parts = append(parts, "готово: "+strconv.Itoa(ok)+" ок")
	if skipped > 0 {
		parts = append(parts, strconv.Itoa(skipped)+" пропущено")
	}
	if errors > 0 {
		parts = append(parts, strconv.Itoa(errors)+" с ошибкой")
	}
	parts = append(parts, "файлов: "+strconv.Itoa(files), "объём: "+util.HumanSize(size))
	return strings.Join(parts, ", ")
}
