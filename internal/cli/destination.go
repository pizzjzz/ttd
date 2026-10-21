package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ttd/internal/config"
	"ttd/internal/model"
	"ttd/internal/service"
	"ttd/internal/util"
)

// looksLikePath — эвристика: значение похоже на путь, а не на имя профиля.
func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "~") || strings.HasPrefix(value, ".") {
		return true
	}
	if strings.ContainsAny(value, `/\`) {
		return true
	}
	if len(value) >= 2 && value[1] == ':' { // C:\...
		return true
	}
	return false
}

// resolveDestination превращает имя профиля или путь в путь.
// named == true для профиля из настроек: такой каталог должен существовать,
// иначе NAS/облако не смонтировано.
func resolveDestination(cfg *config.Config, value string) (string, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, fmt.Errorf("пустое назначение")
	}
	if path, ok := cfg.StoragePath(value); ok {
		lower := strings.ToLower(value)
		if lower == "local" || lower == "library" {
			return path, false, nil
		}
		return path, true, nil
	}
	if looksLikePath(value) {
		return util.ExpandHome(value), false, nil
	}
	return "", false, fmt.Errorf(
		"неизвестный профиль %q — добавьте его: ttd config --add-storage %s=/путь/к/папке",
		value, value)
}

func hasFiles(results []model.JobResult) bool {
	for _, result := range results {
		if len(result.Files) > 0 {
			return true
		}
	}
	return false
}

type destinationChoice struct {
	label   string
	path    string
	storage bool
}

// promptDestination спрашивает, куда сохранить уже скачанные файлы.
func promptDestination(cfg *config.Config, sourceRoot string, results []model.JobResult) error {
	sourceRoot = filepath.Clean(sourceRoot)
	reader := bufio.NewReader(os.Stdin)
	read := func(prompt string) string {
		fmt.Print(prompt)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}

	choices := []destinationChoice{{label: "оставить в постоянной библиотеке", path: sourceRoot}}
	for _, name := range cfg.StorageNames() {
		path := filepath.Clean(cfg.Storages[name])
		if path == sourceRoot {
			continue
		}
		choices = append(choices, destinationChoice{label: name, path: path, storage: true})
	}

	fmt.Println()
	fmt.Println("Куда сохранить скачанное?")
	for i, choice := range choices {
		fmt.Printf("  %d) %s — %s\n", i+1, choice.label, choice.path)
	}
	tempIndex := len(choices) + 1
	permIndex := tempIndex + 1
	fmt.Printf("  %d) разовая папка (без папок-дат, индекса и архива)\n", tempIndex)
	fmt.Printf("  %d) сделать постоянной другую папку\n", permIndex)
	fmt.Println("  0) ничего не делать")

	answer := read("Выбор [1]: ")
	if answer == "" {
		answer = "1"
	}
	number, err := strconv.Atoi(answer)
	if err != nil || number < 0 || number > permIndex {
		fmt.Println("неизвестный выбор — файлы остались на месте")
		return nil
	}

	switch {
	case number == 0 || number == 1:
		return nil
	case number <= len(choices):
		choice := choices[number-1]
		if choice.storage {
			if info, err := os.Stat(choice.path); err != nil || !info.IsDir() {
				return fmt.Errorf("профиль %q: каталог %s не найден (NAS/облако смонтировано?)",
					choice.label, choice.path)
			}
		}
		move := askMoveCopy(read, true)
		return routeAndReport(results, sourceRoot, choice.path, false, move)
	case number == tempIndex:
		path := strings.TrimSpace(read("Папка: "))
		if path == "" {
			return nil
		}
		path = util.ExpandHome(path)
		move := askMoveCopy(read, true)
		return routeAndReport(results, sourceRoot, path, true, move)
	case number == permIndex:
		path := strings.TrimSpace(read("Новая постоянная папка: "))
		if path == "" {
			return nil
		}
		path = util.ExpandHome(path)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			create := strings.ToLower(read("Папка не существует. Создать? [Y/n]: "))
			if create != "" && !strings.HasPrefix(create, "y") && !strings.HasPrefix(create, "д") {
				return nil
			}
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("не удалось создать %s: %v", path, err)
			}
		}
		if err := routeAndReport(results, sourceRoot, path, false, true); err != nil {
			return err
		}
		cfg.MediaRoot = path
		if _, err := config.UpdateMediaRoot(path); err != nil {
			return fmt.Errorf("файлы перенесены, но настройки не сохранены: %v", err)
		}
		fmt.Println("постоянная директория: " + path)
		return nil
	}
	return nil
}

func askMoveCopy(read func(string) string, defaultMove bool) bool {
	def := "c"
	if defaultMove {
		def = "m"
	}
	answer := strings.ToLower(read(fmt.Sprintf("Переместить или скопировать? [m/c] (%s): ", def)))
	if answer == "" {
		return defaultMove
	}
	switch {
	case strings.HasPrefix(answer, "m"), strings.HasPrefix(answer, "п"):
		return true
	case strings.HasPrefix(answer, "c"), strings.HasPrefix(answer, "к"):
		return false
	}
	return defaultMove
}

func routeAndReport(results []model.JobResult, sourceRoot, targetRoot string, flat, move bool) error {
	if filepath.Clean(sourceRoot) == filepath.Clean(targetRoot) {
		fmt.Println("назначение совпадает с текущей папкой")
		return nil
	}
	count, err := service.RouteResults(results, sourceRoot, targetRoot, flat, move)
	if count > 0 {
		verb := "скопировано"
		if move {
			verb = "перемещено"
		}
		fmt.Printf("%s: %d файлов → %s\n", verb, count, targetRoot)
	}
	if err != nil {
		return fmt.Errorf("не все файлы удалось обработать: %v", err)
	}
	return nil
}
