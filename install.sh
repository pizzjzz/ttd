#!/usr/bin/env bash
#
# Установщик ttd.
#
# Собирает программу, ставит её в каталог из PATH и настраивает bash/zsh так,
# чтобы команда `ttd` работала из любого каталога — даже если исходники
# удалить. Движки (gallery-dl, yt-dlp) ставятся в отдельный каталог данных.
#
# Поддерживаются Linux, macOS и Windows (Git Bash / MSYS2 / Cygwin).
# Для cmd и PowerShell на Windows есть отдельный install.ps1.
#
# Примеры:
#   ./install.sh                       обычная установка для текущего пользователя
#   ./install.sh --system              системная установка (нужен root)
#   ./install.sh --bin-dir ~/bin       свой каталог для бинарника
#   ./install.sh --no-rc               не менять ~/.bashrc и ~/.zshrc
#   ./install.sh --uninstall           удалить программу
#
set -euo pipefail

BIN_DIR="${TTD_BIN_DIR:-$HOME/.local/bin}"
DATA_DIR="${TTD_DATA_DIR:-$HOME/.local/share/ttd}"
WITH_ENGINES=1
EDIT_RC=1
UNINSTALL=0
PURGE=0
SYSTEM=0

ROOT="$(cd -P "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# --------------------------------------------------------------------------
# Определение платформы: Linux, macOS или Windows (Git Bash / MSYS2 / Cygwin)
# --------------------------------------------------------------------------
UNAME="$(uname -s 2>/dev/null || echo unknown)"
EXE=""
case "$UNAME" in
  Linux*)                     PLATFORM="Linux" ;;
  Darwin*)                    PLATFORM="macOS" ;;
  MINGW*|MSYS*|CYGWIN*)       PLATFORM="Windows (Git Bash)"; EXE=".exe" ;;
  *)                          PLATFORM="$UNAME" ;;
esac

# В Windows по умолчанию используем тот же каталог данных, что и install.ps1:
# %APPDATA%\ttd. Если TTD_DATA_DIR задан явно — уважаем его.
if [ -z "${TTD_DATA_DIR:-}" ] && [ "$EXE" = ".exe" ] && [ -n "${APPDATA:-}" ]; then
  if command -v cygpath >/dev/null 2>&1; then
    DATA_DIR="$(cygpath -u "$APPDATA")/ttd"
  fi
fi

# native_path переводит путь в формат, понятный установленному бинарнику.
# На Windows (Git Bash) это C:\..., на Linux/macOS путь не меняется.
native_path() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -w "$1"
  else
    printf '%s' "$1"
  fi
}

usage() {
  cat <<'EOF'
Установщик ttd (Linux, macOS, Windows через Git Bash/MSYS2)

Использование:
  ./install.sh [параметры]

Параметры:
  --bin-dir <путь>    куда положить бинарник (по умолчанию ~/.local/bin)
  --data-dir <путь>   куда поставить движки
                      (Linux/macOS: ~/.local/share/ttd, Windows: %APPDATA%\ttd)
  --system            системная установка в /usr/local (запускать через sudo)
  --no-engines        не устанавливать gallery-dl и yt-dlp
  --no-rc             не добавлять PATH в ~/.bashrc и ~/.zshrc
  --uninstall         удалить бинарник, движки и строки из конфигов оболочек
  --purge             вместе с --uninstall удалить весь каталог данных
  -h, --help          эта справка

Переменные окружения:
  TTD_BIN_DIR, TTD_DATA_DIR — то же, что --bin-dir и --data-dir.

Windows:
  В Git Bash скрипт ставит ttd.exe и настраивает bash. Для cmd и PowerShell
  используйте native-установщик:  powershell -ExecutionPolicy Bypass -File install.ps1
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --bin-dir)
      [ $# -ge 2 ] || { echo "ttd: для --bin-dir нужно значение" >&2; exit 2; }
      BIN_DIR="$2"; shift 2 ;;
    --bin-dir=*) BIN_DIR="${1#*=}"; shift ;;
    --data-dir)
      [ $# -ge 2 ] || { echo "ttd: для --data-dir нужно значение" >&2; exit 2; }
      DATA_DIR="$2"; shift 2 ;;
    --data-dir=*) DATA_DIR="${1#*=}"; shift ;;
    --system) SYSTEM=1; shift ;;
    --no-engines) WITH_ENGINES=0; shift ;;
    --no-rc) EDIT_RC=0; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --purge) PURGE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "ttd: неизвестный аргумент: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [ "$SYSTEM" = 1 ]; then
  if [ "$(id -u)" != 0 ]; then
    echo "ttd: --system требует прав root. Запустите: sudo ./install.sh --system" >&2
    exit 1
  fi
  BIN_DIR="/usr/local/bin"
  DATA_DIR="/usr/local/share/ttd"
  # /usr/local/bin уже есть в PATH у всех оболочек.
  EDIT_RC=0
fi

expand_tilde() {
  case "$1" in
    "~") printf '%s' "$HOME" ;;
    "~/"*) printf '%s' "$HOME/${1:2}" ;;
    *) printf '%s' "$1" ;;
  esac
}

BIN_DIR="$(expand_tilde "$BIN_DIR")"
DATA_DIR="$(expand_tilde "$DATA_DIR")"

# --------------------------------------------------------------------------
# Удаление
# --------------------------------------------------------------------------
remove_rc_block() {
  local rc="$1"
  [ -f "$rc" ] || return 0
  if grep -q "# >>> ttd >>>" "$rc" 2>/dev/null; then
    cp "$rc" "$rc.ttd-backup"
    sed -i.bak '/# >>> ttd >>>/,/# <<< ttd <<</d' "$rc"
    rm -f "$rc.bak"
    echo "    убраны строки из $rc (копия: $rc.ttd-backup)"
  fi
}

if [ "$UNINSTALL" = 1 ]; then
  echo "==> Удаляю ttd"
  if [ -e "$BIN_DIR/ttd$EXE" ] || [ -L "$BIN_DIR/ttd$EXE" ]; then
    rm -f "$BIN_DIR/ttd$EXE"
    echo "    удалён $BIN_DIR/ttd$EXE"
  else
    echo "    $BIN_DIR/ttd$EXE не найден"
  fi
  remove_rc_block "$HOME/.bashrc"
  remove_rc_block "$HOME/.zshrc"
  if [ -d "$DATA_DIR/venv" ]; then
    rm -rf "$DATA_DIR/venv"
    echo "    удалены движки из $DATA_DIR/venv"
  fi
  if [ "$PURGE" = 1 ] && [ -d "$DATA_DIR" ]; then
    rm -rf "$DATA_DIR"
    echo "    удалён каталог данных $DATA_DIR"
  fi
  echo
  echo "Готово. Библиотека со скачанными файлами не тронута."
  exit 0
fi

# --------------------------------------------------------------------------
# Поиск Go
# --------------------------------------------------------------------------
go_version_ok() {
  local bin="$1" version major minor rest
  version="$("$bin" env GOVERSION 2>/dev/null || true)"
  version="${version#go}"
  [ -n "$version" ] || return 1
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"
  case "$major$minor" in
    *[!0-9]*) return 1 ;;
  esac
  [ -n "$major" ] && [ -n "$minor" ] || return 1
  [ "$major" -gt 1 ] && return 0
  [ "$major" -eq 1 ] && [ "$minor" -ge 22 ]
}

GO=""
for candidate in "$HOME/.local/go/bin/go" "$(command -v go || true)"; do
  if [ -n "$candidate" ] && [ -x "$candidate" ] && go_version_ok "$candidate"; then
    GO="$candidate"
    break
  fi
done

if [ -z "$GO" ]; then
  if command -v go >/dev/null 2>&1 || [ -x "$HOME/.local/go/bin/go" ]; then
    echo "ttd: найден Go старее 1.22 — нужна версия 1.22 или новее." >&2
    command -v go >/dev/null 2>&1 && "$(command -v go)" version >&2 || true
  fi
  cat >&2 <<'EOF'

Установите Go 1.22+ любым способом:
  Fedora:        sudo dnf install golang
  Debian/Ubuntu: sudo apt install golang-go
  Arch:          sudo pacman -S go
  Вручную:       https://go.dev/dl/

Затем запустите ./install.sh снова.
EOF
  exit 1
fi

echo "==> Платформа:  $PLATFORM"
echo "==> Компилятор: $("$GO" version)"
echo "==> Исходники:  $ROOT"

# --------------------------------------------------------------------------
# Сборка
# --------------------------------------------------------------------------
if ! mkdir -p "$BIN_DIR" 2>/dev/null || [ ! -w "$BIN_DIR" ]; then
  echo "ttd: нет прав на запись в $BIN_DIR" >&2
  echo "     запустите с sudo: sudo ./install.sh --system" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Собираю ttd"
( cd "$ROOT" && "$GO" build -trimpath -ldflags "-s -w" -o "$TMP_DIR/ttd$EXE" ./cmd/ttd )

echo "==> Устанавливаю бинарник в $BIN_DIR/ttd$EXE"
# В Git Bash на Windows команды install может не быть — тогда cp + chmod.
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$TMP_DIR/ttd$EXE" "$BIN_DIR/ttd$EXE"
else
  cp "$TMP_DIR/ttd$EXE" "$BIN_DIR/ttd$EXE"
  chmod 0755 "$BIN_DIR/ttd$EXE" 2>/dev/null || true
fi

# Путь, который поймёт сам бинарник (на Windows — C:\...).
DATA_DIR_NATIVE="$(native_path "$DATA_DIR")"

# --------------------------------------------------------------------------
# Движки в постоянный каталог данных
# --------------------------------------------------------------------------
ENGINES_OK=1
if [ "$WITH_ENGINES" = 1 ]; then
  echo "==> Устанавливаю движки (gallery-dl, yt-dlp) в $DATA_DIR/venv"
  if ! TTD_DATA_DIR="$DATA_DIR_NATIVE" "$BIN_DIR/ttd$EXE" install; then
    ENGINES_OK=0
    echo "ttd: движки установить не удалось. Программа установлена," >&2
    echo "     повторите позже:  TTD_DATA_DIR=\"$DATA_DIR_NATIVE\" ttd install" >&2
  fi
else
  echo "==> Движки пропущены (--no-engines)"
fi

# --------------------------------------------------------------------------
# Прописываем PATH в конфиги bash и zsh
# --------------------------------------------------------------------------
add_path_block() {
  local rc="$1"
  local shell_name="$2"
  if [ ! -f "$rc" ]; then
    touch "$rc"
  fi
  if grep -q "# >>> ttd >>>" "$rc" 2>/dev/null; then
    echo "    $shell_name: PATH уже настроен в $rc"
    return 0
  fi
  if grep -qF "$BIN_DIR" "$rc" 2>/dev/null; then
    echo "    $shell_name: $BIN_DIR уже упоминается в $rc — пропускаю"
    return 0
  fi
  {
    echo ""
    echo "# >>> ttd >>>"
    echo "export PATH=\"$BIN_DIR:\$PATH\""
    echo "# <<< ttd <<<"
  } >> "$rc"
  echo "    $shell_name: добавлен PATH в $rc"
}

if [ "$EDIT_RC" = 1 ]; then
  echo "==> Настраиваю оболочки"
  if command -v bash >/dev/null 2>&1; then
    add_path_block "$HOME/.bashrc" "bash"
  fi
  if command -v zsh >/dev/null 2>&1; then
    add_path_block "$HOME/.zshrc" "zsh"
  fi
  if ! command -v bash >/dev/null 2>&1 && ! command -v zsh >/dev/null 2>&1; then
    echo "    bash и zsh не найдены — добавьте вручную: export PATH=\"$BIN_DIR:\$PATH\""
  fi
fi

# --------------------------------------------------------------------------
# Проверка
# --------------------------------------------------------------------------
echo
echo "==> Проверка установки"
if ! TTD_DATA_DIR="$DATA_DIR_NATIVE" "$BIN_DIR/ttd$EXE" --version; then
  echo "ttd: установленный бинарник не запускается" >&2
  exit 1
fi
if [ "$WITH_ENGINES" = 0 ]; then
  TTD_DATA_DIR="$DATA_DIR_NATIVE" "$BIN_DIR/ttd$EXE" doctor || true
fi

echo
echo "=========================================================================="
echo " ttd установлен: $BIN_DIR/ttd$EXE ($PLATFORM)"
if [ "$SYSTEM" = 1 ]; then
  echo " Программа доступна всем пользователям как /usr/local/bin/ttd."
elif [ "$EDIT_RC" = 1 ]; then
  echo " Чтобы команда заработала в текущем терминале, выполните:"
  echo "   source ~/.bashrc    # для bash"
  echo "   source ~/.zshrc     # для zsh"
  echo " или просто откройте новый терминал."
else
  echo " Добавьте каталог в PATH вручную: export PATH=\"$BIN_DIR:\$PATH\""
fi
if [ "$EXE" = ".exe" ]; then
  echo
  echo " Для cmd/PowerShell используйте native-установщик: .\\install.ps1"
fi
if [ "$ENGINES_OK" = 0 ]; then
  echo
  echo " ВНИМАНИЕ: движки не установлены — загрузка пока не работает."
  echo " Повторите:  TTD_DATA_DIR=\"$DATA_DIR_NATIVE\" ttd install"
fi
echo
echo " Проверка:  ttd doctor"
echo " Загрузка:  ttd \"https://www.tiktok.com/@user/video/...\""
echo " Сайт:      ttd web"
echo "=========================================================================="

if [ "$ENGINES_OK" = 0 ]; then
  exit 1
fi
