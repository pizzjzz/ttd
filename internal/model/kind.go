package model

import (
	"path/filepath"
	"strings"
)

var (
	videoExt = map[string]bool{
		".mp4": true, ".mov": true, ".mkv": true, ".webm": true, ".m4v": true, ".avi": true,
	}
	photoExt = map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true,
		".avif": true, ".gif": true, ".bmp": true,
	}
	audioExt = map[string]bool{
		".mp3": true, ".m4a": true, ".aac": true, ".opus": true, ".ogg": true, ".wav": true,
	}
)

// KindOf определяет тип файла по расширению.
func KindOf(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case videoExt[ext]:
		return KindVideo
	case photoExt[ext]:
		return KindPhoto
	case audioExt[ext]:
		return KindAudio
	default:
		return KindOther
	}
}
