// Команда ttd — загрузчик видео и фото из TikTok.
package main

import (
	"os"

	"ttd/internal/cli"
)

func main() {
	os.Exit(cli.Main())
}
