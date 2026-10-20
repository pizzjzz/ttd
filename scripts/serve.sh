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
cd "$ROOT"

if [ ! -x ./ttd ]; then
  GO="$(command -v go || true)"
  [ -z "$GO" ] && [ -x "$HOME/.local/go/bin/go" ] && GO="$HOME/.local/go/bin/go"
  if [ -z "$GO" ]; then
    echo "ttd ещё не собран, а Go не найден. Запустите scripts/install.sh" >&2
    exit 1
  fi
  echo "==> Собираю ./ttd"
  "$GO" build -trimpath -ldflags "-s -w" -o ttd ./cmd/ttd
fi

exec ./ttd web "$@"
