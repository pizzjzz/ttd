package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ttd/internal/config"
	"ttd/internal/model"
	"ttd/internal/service"
	"ttd/internal/util"
)

// Шаги диалога выбора назначения после скачивания.
const (
	choiceStepSelect = iota
	choiceStepPath
	choiceStepMode
)

// tuiDestination — один пункт меню сохранения.
type tuiDestination struct {
	label     string
	path      string
	flat      bool
	permanent bool
	storage   bool
}

func hasChoiceFiles(results []model.JobResult) bool {
	for _, result := range results {
		if len(result.Files) > 0 {
			return true
		}
	}
	return false
}

func countChoiceFiles(results []model.JobResult) int {
	total := 0
	for _, result := range results {
		total += len(result.Files)
	}
	return total
}

// beginChoice готовит меню: библиотека, профили NAS/облака, разовая папка
// и смена постоянной директории.
func (t *Tui) beginChoice() {
	t.choosing = true
	t.choiceStep = choiceStepSelect
	t.choiceIndex = 0
	t.choiceInput = nil
	t.choiceError = ""

	source := filepath.Clean(t.cfg.MediaPath())
	t.choiceOptions = []tuiDestination{{label: "оставить в постоянной библиотеке", path: source}}
	for _, name := range t.cfg.StorageNames() {
		path := filepath.Clean(t.cfg.Storages[name])
		if path == source {
			continue
		}
		t.choiceOptions = append(t.choiceOptions, tuiDestination{label: name, path: path, storage: true})
	}
	t.choiceOptions = append(t.choiceOptions,
		tuiDestination{label: "разовая папка", flat: true},
		tuiDestination{label: "сделать постоянной другую папку", permanent: true},
	)
}

// handleChoiceKey обрабатывает клавиши, пока открыт диалог назначения.
func (t *Tui) handleChoiceKey(key byte) bool {
	switch t.choiceStep {
	case choiceStepSelect:
		switch {
		case key == 0x11: // Ctrl-Q — выйти из ttd
			return true
		case key == 0x1b: // Esc — оставить как есть
			t.choosing = false
			t.choiceError = ""
		case key == 0x0d || key == 0x0a: // Enter
			t.confirmChoice()
		case key >= '0' && key <= '9':
			number := int(key - '0')
			if number == 0 {
				t.choosing = false
				t.choiceError = ""
				return false
			}
			if number <= len(t.choiceOptions) {
				t.choiceIndex = number - 1
				t.confirmChoice()
			}
		}
	case choiceStepPath:
		switch {
		case key == 0x1b: // Esc — назад
			t.choiceStep = choiceStepSelect
			t.choiceError = ""
		case key == 0x0d || key == 0x0a:
			t.confirmPath()
		case key == 0x7f || key == 0x08:
			if len(t.choiceInput) > 0 {
				t.choiceInput = t.choiceInput[:len(t.choiceInput)-1]
				// Убираем остаток многобайтового UTF-8 символа.
				for len(t.choiceInput) > 0 && t.choiceInput[len(t.choiceInput)-1]&0xc0 == 0x80 {
					t.choiceInput = t.choiceInput[:len(t.choiceInput)-1]
				}
			}
		default:
			if key >= 0x20 && key != 0x7f {
				t.choiceInput = append(t.choiceInput, key)
			}
		}
	case choiceStepMode:
		switch {
		case key == 0x11:
			return true
		case key == 0x1b:
			t.choiceStep = choiceStepSelect
			t.choiceError = ""
		case key == 'm' || key == 'M':
			t.choiceMove = true
			t.applyChoice()
		case key == 'c' || key == 'C':
			t.choiceMove = false
			t.applyChoice()
		case key == 0x0d || key == 0x0a:
			t.applyChoice()
		}
	}
	return false
}

func (t *Tui) confirmChoice() {
	if t.choiceIndex < 0 || t.choiceIndex >= len(t.choiceOptions) {
		t.choosing = false
		return
	}
	choice := t.choiceOptions[t.choiceIndex]
	switch {
	case choice.flat || choice.permanent:
		t.choiceTarget = choice
		t.choiceInput = nil
		t.choiceError = ""
		t.choiceStep = choiceStepPath
	case choice.storage:
		if info, err := os.Stat(choice.path); err != nil || !info.IsDir() {
			t.choiceError = "каталог не найден (NAS/облако смонтировано?)"
			return
		}
		t.choiceTarget = choice
		t.choiceMove = true
		t.choiceError = ""
		t.choiceStep = choiceStepMode
	default:
		// «Оставить в постоянной библиотеке».
		t.choosing = false
		t.choiceError = ""
	}
}

func (t *Tui) confirmPath() {
	path := strings.TrimSpace(string(t.choiceInput))
	if path == "" {
		return
	}
	path = util.ExpandHome(path)
	if t.choiceTarget.permanent {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.choiceError = "не удалось создать: " + err.Error()
				return
			}
		}
	}
	t.choiceTarget.path = path
	t.choiceMove = true
	t.choiceError = ""
	t.choiceStep = choiceStepMode
}

func (t *Tui) applyChoice() {
	target := t.choiceTarget
	source := t.cfg.MediaPath()
	count, err := service.RouteResults(t.results, source, target.path, target.flat, t.choiceMove)
	if err != nil {
		t.appendLog("ошибка сохранения: " + err.Error())
		t.status = "не удалось сохранить в " + target.path
		t.choosing = false
		t.choiceError = ""
		return
	}
	verb := "скопировано"
	if t.choiceMove {
		verb = "перемещено"
	}
	t.appendLog(fmt.Sprintf("%s: %d файлов → %s", verb, count, target.path))
	t.status = fmt.Sprintf("%s: %d файлов", verb, count)
	if target.permanent {
		t.cfg.MediaRoot = target.path
		if _, saveErr := config.UpdateMediaRoot(target.path); saveErr != nil {
			t.appendLog("настройки не сохранены: " + saveErr.Error())
		} else {
			t.appendLog("постоянная директория: " + target.path)
		}
	}
	t.choosing = false
	t.choiceError = ""
}

// drawChoice рисует диалог выбора назначения на весь экран.
func (t *Tui) drawChoice(width, height int) {
	if width < 20 {
		width = 20
	}
	lines := make([]string, 0, height)
	lines = append(lines, cBold+cCyan+"ttd — куда сохранить скачанное"+cReset)
	lines = append(lines, strings.Repeat("─", width))
	lines = append(lines, cDim+fmt.Sprintf("Файлов: %d    Постоянная библиотека: %s",
		countChoiceFiles(t.results), t.cfg.MediaPath())+cReset)
	lines = append(lines, "")

	switch t.choiceStep {
	case choiceStepSelect:
		for i, choice := range t.choiceOptions {
			label := choice.label
			if choice.path != "" {
				label += " — " + choice.path
			}
			prefix := "  "
			color := ""
			if i == t.choiceIndex {
				prefix = "> "
				color = cGreen
			}
			lines = append(lines, color+prefix+fmt.Sprintf("%d) %s", i+1, label)+cReset)
		}
		lines = append(lines, "  0) ничего не делать")
		lines = append(lines, "")
		lines = append(lines, cDim+"1-9 — выбрать, Esc — оставить как есть"+cReset)
	case choiceStepPath:
		title := "разовой папки"
		if t.choiceTarget.permanent {
			title = "новой постоянной папки"
		}
		lines = append(lines, cBold+"Путь для "+title+":"+cReset)
		lines = append(lines, "  "+string(t.choiceInput)+"_")
		lines = append(lines, "")
		lines = append(lines, cDim+"Enter — подтвердить, Esc — назад"+cReset)
	case choiceStepMode:
		lines = append(lines, cBold+"Куда: "+t.choiceTarget.label+" — "+t.choiceTarget.path+cReset)
		lines = append(lines, "")
		lines = append(lines, "Переместить (m) или скопировать (c)?")
		lines = append(lines, cDim+"Enter — переместить, Esc — назад"+cReset)
	}
	if t.choiceError != "" {
		lines = append(lines, "", cRed+t.choiceError+cReset)
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	lines = append(lines, cBold+cBlue+"Esc — закрыть   Enter — подтвердить"+cReset)

	var builder strings.Builder
	builder.WriteString("\x1b[2J\x1b[H")
	for i, line := range lines {
		if i > 0 {
			builder.WriteString("\r\n")
		}
		builder.WriteString(line)
	}
	builder.WriteString("\x1b[?25l")
	_, _ = t.out.WriteString(builder.String())
	_ = t.out.Flush()
}
