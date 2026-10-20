#!/usr/bin/env bash
# Совместимость: старый путь установщика. Делегирует в ./install.sh.
set -euo pipefail
ROOT="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/.." >/dev/null 2>&1 && pwd)"
exec "$ROOT/install.sh" "$@"
