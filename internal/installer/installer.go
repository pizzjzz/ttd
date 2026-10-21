// Package installer ставит и проверяет внешние движки (gallery-dl, yt-dlp).
package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"ttd/internal/config"
	"ttd/internal/engines"
	"ttd/internal/model"
	"ttd/internal/util"
)

var pythonNames = []string{"python3", "python", "py"}

// Минимальная поддерживаемая версия Python (её требует yt-dlp).
const (
	minPythonMajor = 3
	minPythonMinor = 10
	// MinPython — минимальная версия для сообщений об ошибке.
	MinPython = "3.10"
)

// Options управляет источником движков при установке.
type Options struct {
	// Online — обновить движки с PyPI (нужен интернет).
	Online bool
	// WheelsDir — локальный каталог с .whl для офлайн-обновления.
	WheelsDir string
}

// FindPython ищет интерпретатор Python.
func FindPython() string {
	return util.FindExecutable(pythonNames, nil)
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
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
	emit(model.Event{"type": "install", "line": "движки не найдены — ставлю встроенный набор (без интернета)"})
	if err := Bootstrap(ctx, cfg, onEvent); err != nil {
		return engines.FindTools(cfg), err
	}
	tools = engines.FindTools(cfg)
	if tools["gallery-dl"] == "" || tools["yt-dlp"] == "" {
		return tools, fmt.Errorf("после установки движки всё ещё не найдены")
	}
	return tools, nil
}

// Bootstrap создаёт окружение и ставит в него встроенные движки без интернета.
func Bootstrap(ctx context.Context, cfg *config.Config, onEvent func(model.Event)) error {
	return BootstrapWith(ctx, cfg, onEvent, Options{})
}

// BootstrapWith ставит движки из выбранного источника:
// встроенный набор (по умолчанию), локальный каталог колёс или PyPI.
func BootstrapWith(ctx context.Context, cfg *config.Config, onEvent func(model.Event), opts Options) error {
	emit := engines.EventFunc(onEvent)

	python := FindPython()
	if python == "" {
		return fmt.Errorf("не найден python3 — установите Python %s+ и повторите", MinPython)
	}
	if err := checkPythonVersion(ctx, python); err != nil {
		return err
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
			return fmt.Errorf("не удалось создать окружение: %v (нужен модуль venv, обычно пакет python3-venv)", result.Err)
		}
	}

	pip := filepath.Join(VenvBinDir(cfg), exeName("pip"))
	if _, err := os.Stat(pip); err != nil {
		return fmt.Errorf("в окружении нет pip: %s", pip)
	}
	pythonVenv := filepath.Join(VenvBinDir(cfg), exeName("python"))
	if _, err := os.Stat(pythonVenv); err != nil {
		pythonVenv = python
	}

	args, cleanup, err := pipInstallArgs(opts)
	if err != nil {
		return err
	}
	defer cleanup()

	if !opts.Online && opts.WheelsDir == "" {
		versions := BundledVersions()
		emit(model.Event{"type": "install", "line": fmt.Sprintf(
			"встроенный набор: gallery-dl %s, yt-dlp %s",
			versions["gallery-dl"], versions["yt-dlp"],
		)})
	}
	command := "$ " + filepath.Base(pythonVenv) + " " + strings.Join(args, " ")
	emit(model.Event{"type": "install", "line": command})
	env := append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1", "PIP_NO_INPUT=1")
	result := util.RunStream(ctx, pythonVenv, args, env, func(line string) {
		emit(model.Event{"type": "install", "line": line})
	})
	if result.ExitCode != 0 {
		return fmt.Errorf("не удалось установить движки: %v", result.Err)
	}
	return nil
}

// pipInstallArgs собирает аргументы pip для выбранного источника движков.
func pipInstallArgs(opts Options) ([]string, func(), error) {
	noop := func() {}
	base := []string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", "--no-cache-dir"}
	if opts.Online {
		return append(base, "--upgrade", "pip", "gallery-dl", "yt-dlp"), noop, nil
	}
	if opts.WheelsDir != "" {
		dir, err := filepath.Abs(util.ExpandHome(opts.WheelsDir))
		if err != nil {
			return nil, noop, err
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return nil, noop, fmt.Errorf("каталог с движками не найден: %s", dir)
		}
		return append(base, "--no-index", "--find-links", dir, "--only-binary=:all:", "gallery-dl", "yt-dlp"), noop, nil
	}
	dir, err := os.MkdirTemp("", "ttd-wheels-")
	if err != nil {
		return nil, noop, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if _, err := extractEmbeddedWheels(dir); err != nil {
		cleanup()
		return nil, noop, err
	}
	return append(base, "--no-index", "--find-links", dir, "--only-binary=:all:", "gallery-dl", "yt-dlp"), cleanup, nil
}

// checkPythonVersion проверяет, что интерпретатор достаточно новый.
func checkPythonVersion(ctx context.Context, python string) error {
	out, errOut, err := util.Capture(ctx, python,
		[]string{"-c", "import sys; print('%d.%d' % sys.version_info[:2])"}, nil)
	if err != nil {
		message := strings.TrimSpace(errOut)
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("не удалось запустить Python %s: %s", python, message)
	}
	version := strings.TrimSpace(out)
	if !pythonVersionOK(version) {
		return fmt.Errorf("нужен Python %s или новее, найден %s", MinPython, version)
	}
	return nil
}

func pythonVersionOK(version string) bool {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	return major > minPythonMajor || (major == minPythonMajor && minor >= minPythonMinor)
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
	lines = append(lines, "каталог данных:    "+config.DataDir())
	lines = append(lines, "библиотека:        "+cfg.MediaPath())
	lines = append(lines, "файл настроек:     "+config.ConfigFile())
	lines = append(lines, "кэш библиотеки:    "+cfg.CacheDir())
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
	if versions := BundledVersions(); versions["gallery-dl"] != "" && versions["yt-dlp"] != "" {
		lines = append(lines, fmt.Sprintf("встроенные движки: gallery-dl %s, yt-dlp %s (без интернета)",
			versions["gallery-dl"], versions["yt-dlp"]))
	} else {
		lines = append(lines, "встроенные движки: нет")
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
