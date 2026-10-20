#!/usr/bin/env bash
# Установка ttd: сборка Go-бинарника, установка движков и создание ссылки в PATH.
set -euo pipefail

SOURCE="${BASH_SOURCE[0]}"
while [ -L "$SOURCE" ]; do
  DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
ROOT="$(cd -P "$(dirname "$SOURCE")/.." >/dev/null 2>&1 && pwd)"

echo "==> Каталог проекта: $ROOT"

# --- Поиск Go -------------------------------------------------------------
GO="$(command -v go || true)"
if [ -z "$GO" ] && [ -x "$HOME/.local/go/bin/go" ]; then
  GO="$HOME/.local/go/bin/go"
  export PATH="$HOME/.local/go/bin:$PATH"
fi
if [ -z "$GO" ]; then
  cat <<'EOF'
Ошибка: не найден компилятор Go.

Установите Go 1.22 или новее одним из способов:

  Fedora:        sudo dnf install golang
  Debian/Ubuntu: sudo apt install golang-go
  Arch:          sudo pacman -S go
  Вручную:       https://go.dev/dl/

Затем запустите этот скрипт снова.
EOF
  exit 1
fi

echo "==> Компилятор: $($GO version)"

# --- Сборка ---------------------------------------------------------------
echo "==> Собираю бинарник ./ttd"
( cd "$ROOT" && "$GO" build -trimpath -ldflags "-s -w" -o ttd ./cmd/ttd )

# --- Движки ---------------------------------------------------------------
echo "==> Устанавливаю движки (gallery-dl, yt-dlp) в .venv"
"$ROOT/ttd" install

# --- Ссылка в PATH --------------------------------------------------------
BIN_DIR="${TTD_BIN_DIR:-$HOME/.local/bin}"
mkdir -p "$BIN_DIR"
ln -sf "$ROOT/ttd" "$BIN_DIR/ttd"
echo "==> Создана ссылка: $BIN_DIR/ttd -> $ROOT/ttd"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "==> Добавьте каталог в PATH:  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac

echo
echo "Готово. Проверьте установку командой:  ttd doctor"
