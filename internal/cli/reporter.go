package cli

import (
	"fmt"
	"io"
	"os"
	"sync"

	"ttd/internal/model"
	"ttd/internal/util"
)

// Reporter печатает события загрузки в консоль.
type Reporter struct {
	out      io.Writer
	isTTY    bool
	showLogs bool
	quiet    bool

	mu        sync.Mutex
	lineOpen  bool
	milestone map[string]int
}

// NewReporter создаёт печатающий обработчик событий.
func NewReporter(out io.Writer, showLogs, quiet bool) *Reporter {
	isTTY := false
	if file, ok := out.(*os.File); ok {
		if info, err := file.Stat(); err == nil {
			isTTY = info.Mode()&os.ModeCharDevice != 0
		}
	}
	return &Reporter{out: out, isTTY: isTTY, showLogs: showLogs, quiet: quiet, milestone: map[string]int{}}
}

// Handle обрабатывает одно событие.
func (r *Reporter) Handle(event model.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch stringField(event, "type") {
	case "start":
		if r.quiet {
			return
		}
		count := intField(event, "count")
		date := stringField(event, "date")
		root := stringField(event, "root")
		engine := stringField(event, "engine")
		if boolField(event, "dry_run") {
			fmt.Fprintf(r.out, "Пробный запуск: %d ссылок, папка-дата %s\n", count, date)
		} else {
			fmt.Fprintf(r.out, "Загрузка %d ссылок в %s (дата скачивания %s, движок %s)\n", count, root, date, engine)
		}
	case "job":
		if r.quiet {
			return
		}
		r.closeLine()
		fmt.Fprintf(r.out, "\n[%d/%d] %s\n      вид: %s, движок: %s\n",
			intField(event, "index"), intField(event, "total"),
			stringField(event, "url"), stringField(event, "kind"), stringField(event, "engine"))
	case "log":
		if r.showLogs && !r.quiet {
			r.closeLine()
			fmt.Fprintf(r.out, "      · %s\n", stringField(event, "line"))
		}
	case "progress":
		if r.quiet {
			return
		}
		url := stringField(event, "url")
		percent := int(intFieldFloat(event, "percent"))
		speed := stringField(event, "speed")
		eta := stringField(event, "eta")
		text := fmt.Sprintf("      загрузка %d%%", percent)
		if speed != "" {
			text += ", " + speed
		}
		if eta != "" {
			text += ", осталось " + eta
		}
		if r.isTTY {
			fmt.Fprintf(r.out, "\r%s      ", text)
			r.lineOpen = true
			return
		}
		if percent >= r.milestone[url]+25 {
			r.milestone[url] = (percent/25 + 1) * 25
			fmt.Fprintf(r.out, "%s\n", text)
		}
	case "file":
		if r.quiet {
			return
		}
		r.closeLine()
		if item := mediaField(event, "file"); item != nil {
			fmt.Fprintf(r.out, "      + %s (%s)\n", item.Name, util.HumanSize(item.Size))
		}
	case "saved":
		if r.quiet {
			return
		}
		r.closeLine()
	case "result":
		if r.quiet {
			return
		}
		r.closeLine()
		status := stringField(event, "status")
		files := len(mediaList(event, "files"))
		switch status {
		case model.StatusOK:
			fmt.Fprintf(r.out, "      итог: скачано файлов %d\n", files)
		case model.StatusSkipped:
			fmt.Fprintf(r.out, "      итог: уже скачано (пропущено)\n")
		case model.StatusCancelled:
			fmt.Fprintf(r.out, "      итог: отменено\n")
		default:
			fmt.Fprintf(r.out, "      итог: ошибка — %s\n", stringField(event, "error"))
		}
	case "error":
		r.closeLine()
		msg := stringField(event, "error")
		if msg == "" {
			msg = stringField(event, "line")
		}
		if msg != "" {
			fmt.Fprintf(r.out, "ошибка: %s\n", msg)
		}
	case "install":
		if r.quiet {
			return
		}
		r.closeLine()
		fmt.Fprintf(r.out, "      · %s\n", stringField(event, "line"))
	case "done":
		r.closeLine()
	}
}

func (r *Reporter) closeLine() {
	if r.lineOpen {
		fmt.Fprint(r.out, "\n")
		r.lineOpen = false
	}
}

func stringField(event model.Event, key string) string {
	if value, ok := event[key].(string); ok {
		return value
	}
	return ""
}

func intField(event model.Event, key string) int {
	return int(intFieldFloat(event, key))
}

func intFieldFloat(event model.Event, key string) float64 {
	switch value := event[key].(type) {
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case float64:
		return value
	}
	return 0
}

func boolField(event model.Event, key string) bool {
	value, _ := event[key].(bool)
	return value
}

func mediaField(event model.Event, key string) *model.MediaItem {
	switch value := event[key].(type) {
	case model.MediaItem:
		copy := value
		return &copy
	case *model.MediaItem:
		return value
	case map[string]any:
		item := &model.MediaItem{
			Name: stringField(value, "name"),
			Path: stringField(value, "path"),
			Kind: stringField(value, "kind"),
		}
		if size, ok := value["size"].(float64); ok {
			item.Size = int64(size)
		}
		return item
	}
	return nil
}

func mediaList(event model.Event, key string) []model.MediaItem {
	switch value := event[key].(type) {
	case []model.MediaItem:
		return value
	case []any:
		items := make([]model.MediaItem, 0, len(value))
		for _, entry := range value {
			if mapped, ok := entry.(map[string]any); ok {
				item := model.MediaItem{Name: stringField(mapped, "name")}
				if size, ok := mapped["size"].(float64); ok {
					item.Size = int64(size)
				}
				items = append(items, item)
			}
		}
		return items
	}
	return nil
}
