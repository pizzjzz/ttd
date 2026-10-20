// Package installer ставит и проверяет внешние движки (gallery-dl, yt-dlp).
package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/model"
	"ttd/internal/util"
)

var pythonNames = []string{"python3", "python"}

// FindPython ищет интерпретатор Python.
func FindPython() string {
	return util.FindExecutable(pythonNames, nil)
}

// VenvBinDir — каталог с исполняемыми файлами окружения.
func VenvBinDir(cfg *config.Config) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(cfg.VenvDir(), "Scripts")
	}
	return filepath.Join(cfg.VenvDir(), "bin")
}

// EnsureTools возвращает пути к движкам, при необходимости устанавливая их.
func EnsureTools(ctx context.Context, cfg *config.Config, onEvent func(model.Event)) (map[string]string, error) {
	emit := engines.EventFunc(onEvent)
	tools := engines.FindTools(cfg)
	if tools["gallery-dl"] != "" && tools["yt-dlp"] != "" {
		return tools, nil
	}
	emit(model.Event{"type": "install", "line": "движки не найдены — создаю окружение .venv"})
	if err := Bootstrap(ctx, cfg, onEvent); err != nil {
		return engines.FindTools(cfg), err
	}
	tools = engines.FindTools(cfg)
	if tools["gallery-dl"] == "" || tools["yt-dlp"] == "" {
		return tools, fmt.Errorf("после установки движки всё ещё не найдены")
	}
	return tools, nil
}

// Bootstrap создаёт виртуальное окружение и ставит в него движки.
func Bootstrap(ctx context.Context, cfg *config.Config, onEvent func(model.Event)) error {
	emit := engines.EventFunc(onEvent)

	python := FindPython()
	if python == "" {
		return fmt.Errorf("не найден python3 — установите Python 3.9+ и повторите")
	}
	venv := cfg.VenvDir()
	if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(venv); err != nil {
		emit(model.Event{"type": "install", "line": "$ " + python + " -m venv " + venv})
		result := util.RunStream(ctx, python, []string{"-m", "venv", venv}, nil, func(line string) {
			emit(model.Event{"type": "install", "line": line})
		})
		if result.ExitCode != 0 {
			return fmt.Errorf("не удалось создать окружение: %v", result.Err)
		}
	}

	pip := filepath.Join(VenvBinDir(cfg), "pip")
	if _, err := os.Stat(pip); err != nil {
		return fmt.Errorf("в окружении нет pip: %s", pip)
	}
	pythonVenv := filepath.Join(VenvBinDir(cfg), "python")
	if _, err := os.Stat(pythonVenv); err != nil {
		pythonVenv = python
	}

	args := []string{"-m", "pip", "install", "--upgrade", "pip", "gallery-dl", "yt-dlp"}
	emit(model.Event{"type": "install", "line": "$ python -m pip install --upgrade pip gallery-dl yt-dlp"})
	result := util.RunStream(ctx, pythonVenv, args, nil, func(line string) {
		emit(model.Event{"type": "install", "line": line})
	})
	if result.ExitCode != 0 {
		return fmt.Errorf("не удалось установить движки: %v", result.Err)
	}
	return nil
}

// BootstrapAll устанавливает движки и возвращает их пути.
func BootstrapAll(ctx context.Context, cfg *config.Config) (map[string]string, error) {
	if err := Bootstrap(ctx, cfg, nil); err != nil {
		return engines.FindTools(cfg), err
	}
	return engines.FindTools(cfg), nil
}

// Doctor собирает отчёт о состоянии установки.
func Doctor(cfg *config.Config) []string {
	tools := engines.FindTools(cfg)
	lines := []string{}
	lines = append(lines, "каталог приложения: "+config.ProjectRoot())
	lines = append(lines, "библиотека:        "+cfg.MediaPath())
	lines = append(lines, "файл настроек:     "+config.ConfigFile())
	lines = append(lines, "локальные данные:  "+cfg.CacheDir())
	if python := FindPython(); python != "" {
		lines = append(lines, "python:            "+python)
	} else {
		lines = append(lines, "python:            НЕ НАЙДЕН")
	}
	if info, err := os.Stat(cfg.VenvDir()); err == nil && info.IsDir() {
		lines = append(lines, "виртуальное окр.:  "+cfg.VenvDir()+" (есть)")
	} else {
		lines = append(lines, "виртуальное окр.:  "+cfg.VenvDir()+" (нет)")
	}
	for _, name := range []string{"gallery-dl", "yt-dlp"} {
		path := tools[name]
		if path == "" {
			lines = append(lines, name+": не найден")
			continue
		}
		version := engines.ToolVersion(path)
		if version == "" {
			version = "версия неизвестна"
		}
		lines = append(lines, fmt.Sprintf("%s: %s — %s", name, path, version))
	}
	lines = append(lines, "поддерживаемые движки: "+strings.Join(config.SupportedEngines, ", "))
	return lines
}
