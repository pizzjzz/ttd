package installer

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// embeddedWheels — набор Python-колёс (gallery-dl, yt-dlp и их зависимости),
// встроенный в бинарник. Благодаря ему `ttd install` не обращается к интернету.
//
//go:embed wheels/*.whl
var embeddedWheels embed.FS

// BundledVersions возвращает версии встроенных движков:
// {"gallery-dl": "1.32.17", "yt-dlp": "2026.8.19"}.
func BundledVersions() map[string]string {
	versions := map[string]string{}
	for _, name := range bundledWheelNames() {
		dist, version := wheelNameParts(name)
		switch dist {
		case "gallery-dl", "yt-dlp":
			versions[dist] = version
		}
	}
	return versions
}

// BundledWheelNames перечисляет встроенные .whl (для диагностики и тестов).
func BundledWheelNames() []string {
	return bundledWheelNames()
}

func bundledWheelNames() []string {
	entries, err := fs.Glob(embeddedWheels, "wheels/*.whl")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, filepath.Base(entry))
	}
	return names
}

// wheelNameParts разбирает имя колеса PEP 427:
// gallery_dl-1.32.17-py3-none-any.whl → ("gallery-dl", "1.32.17").
func wheelNameParts(name string) (string, string) {
	base := strings.TrimSuffix(name, ".whl")
	parts := strings.Split(base, "-")
	if len(parts) < 2 {
		return "", ""
	}
	dist := strings.ReplaceAll(parts[0], "_", "-")
	return dist, parts[1]
}

// extractEmbeddedWheels раскладывает встроенные колёса в каталог dst.
// Возвращает количество файлов.
func extractEmbeddedWheels(dst string) (int, error) {
	entries, err := fs.Glob(embeddedWheels, "wheels/*.whl")
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, fmt.Errorf("встроенный набор движков пуст")
	}
	for _, entry := range entries {
		data, err := embeddedWheels.ReadFile(entry)
		if err != nil {
			return 0, err
		}
		path := filepath.Join(dst, filepath.Base(entry))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return 0, err
		}
	}
	return len(entries), nil
}
