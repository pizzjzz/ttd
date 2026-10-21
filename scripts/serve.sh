#!/usr/bin/env bash
# Запуск локального сайта ttd на localhost.
# Любые аргументы передаются команде `ttd web`, например:
#   scripts/serve.sh --port 9000
set -euo pipefail

SOURCE="${BASH_SOURCE[0]}"
while [ -L "$SOURCE" ]; do
  DIR="$(cd -P "$(dirname "$SOURCE")" >/dev/null 2>&1 && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
ROOT="$(cd -P "$(dirname "$SOURCE")/.." >/dev/null 2>&1 && pwd)"

# На Windows (Git Bash / MSYS2) бинарник называется ttd.exe.
EXE=""
case "$(uname -s 2>/dev/null || echo unknown)" in
  MINGW*|MSYS*|CYGWIN*) EXE=".exe" ;;
esac
BIN="$ROOT/ttd$EXE"

cd "$ROOT"

if [ ! -x "$BIN" ]; then
  GO="$(command -v go || true)"
  [ -z "$GO" ] && [ -x "$HOME/.local/go/bin/go" ] && GO="$HOME/.local/go/bin/go"
  if [ -z "$GO" ]; then
    echo "ttd ещё не собран, а Go не найден. Запустите ./install.sh" >&2
    exit 1
  fi
  echo "==> Собираю $BIN"
  "$GO" build -trimpath -ldflags "-s -w" -o "$BIN" ./cmd/ttd
fi

exec "$BIN" web "$@"
