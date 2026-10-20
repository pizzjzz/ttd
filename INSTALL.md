# Установка и запуск ttd

Пошаговая инструкция: от чистого компьютера до первой скачанной ссылки.
Поддерживаются Linux и macOS (на Windows возможен запуск, но TUI работает в
упрощённом режиме).

---

## 1. Что понадобится

| Компонент | Версия | Зачем |
| --- | --- | --- |
| **Go** | 1.22 или новее | собрать саму программу `ttd` |
| **Python** | 3.9 или новее | для движков загрузки (gallery-dl, yt-dlp) |
| Интернет | — | для загрузки движков и самих видео |

Проверить, что уже установлено:

```bash
go version
python3 --version
```

Если Go не найден — переходите к пункту 2. Если Python не найден — установите
его пакетным менеджером (`sudo dnf install python3`, `sudo apt install python3`
и так далее).

---

## 2. Установка Go (если нужно)

### Вариант А. Через пакетный менеджер

```bash
# Fedora
sudo dnf install golang

# Debian / Ubuntu
sudo apt update && sudo apt install golang-go

# Arch
sudo pacman -S go
```

### Вариант Б. Без прав root, в домашний каталог

```bash
# узнать последнюю версию
VER=$(curl -s "https://go.dev/VERSION?m=text" | head -1)

# скачать и распаковать в ~/.local/go
mkdir -p ~/.local
curl -L -o /tmp/go.tar.gz "https://go.dev/dl/${VER}.linux-amd64.tar.gz"
rm -rf ~/.local/go
tar -C ~/.local -xzf /tmp/go.tar.gz

# добавить в PATH (для текущей сессии)
export PATH="$HOME/.local/go/bin:$PATH"

# и навсегда (для bash)
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc
```

Проверка: `go version` должен вывести версию.

> Для macOS во втором варианте замените имя файла на `…darwin-arm64.tar.gz`
> (Apple Silicon) или `…darwin-amd64.tar.gz` (Intel).

---

## 3. Получение исходников

```bash
git clone <адрес-репозитория> ttd
cd ttd
```

Если репозиторий уже скопирован — просто перейдите в его каталог:

```bash
cd ~/ttd
```

---

## 4. Установка одной командой

```bash
./scripts/install.sh
```

Скрипт делает четыре вещи:

1. находит компилятор Go;
2. собирает бинарник `./ttd` из `./cmd/ttd`;
3. запускает `ttd install` — создаёт окружение `.venv` и ставит в него
   `gallery-dl` и `yt-dlp`;
4. создаёт символическую ссылку `~/.local/bin/ttd → <каталог>/ttd`.

Если в конце появится подсказка про `PATH`, выполните её:

```bash
export PATH="$HOME/.local/bin:$PATH"
# чтобы сохранить навсегда:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

### Сборка вручную (без скрипта)

```bash
go build -trimpath -ldflags "-s -w" -o ttd ./cmd/ttd   # собрать
./ttd install                                          # поставить движки
sudo ln -sf "$PWD/ttd" /usr/local/bin/ttd              # по желанию: в PATH
```

---

## 5. Проверка установки

```bash
ttd doctor
```

Ожидаемый вывод — примерно такой:

```
каталог приложения: /home/user/ttd
библиотека:        /home/user/ttd
файл настроек:     /home/user/.config/ttd/config.json
локальные данные:  /home/user/ttd/.ttd-cache
python:            /usr/bin/python3
виртуальное окр.:  /home/user/ttd/.venv (есть)
gallery-dl: /home/user/ttd/.venv/bin/gallery-dl — 1.32.17
yt-dlp: /home/user/ttd/.venv/bin/yt-dlp — 2026.08.19
поддерживаемые движки: auto, gallery-dl, yt-dlp, mock
```

Строки `gallery-dl` и `yt-dlp` должны содержать версии. Если вместо этого
написано «не найден» — повторите `ttd install`.

---

## 6. Первая загрузка

```bash
ttd "https://www.tiktok.com/@user/video/7412345678901234567"
```

Результат появится в каталоге с двумя датами:

```
~/ttd/19.10.2026 | 17.10.2026/user_7412345678901234567.mp4
```

Без обращения к TikTok установку можно проверить на встроенном демо-движке:

```bash
ttd --engine mock --dir /tmp/ttd-demo "https://www.tiktok.com/@user/video/123"
ttd list --dir /tmp/ttd-demo --files
```

Посмотреть, что доступно, не скачивая:

```bash
ttd probe "https://www.tiktok.com/@user/video/7412345678901234567"
```

---

## 7. Три способа запуска

### Командная строка (CLI)

```bash
ttd <ссылка> [ещё ссылки…]
ttd --stories "https://www.tiktok.com/@user"   # профиль + истории
ttd list --files                                # что уже скачано
```

### Терминальный интерфейс (TUI)

```bash
ttd tui
```

Введите ссылку, `Enter` — добавить, `F5` (или `Ctrl-D`) — начать,
`F10` (или `Ctrl-Q`) — выйти.

### Локальный сайт (Web)

```bash
ttd web
# или
./scripts/serve.sh
```

Откройте указанный в выводе адрес (по умолчанию <http://127.0.0.1:8787/>).
Скрипт `scripts/serve.sh` при необходимости сначала собирает бинарник и затем
запускает сервер; любые аргументы передаются дальше:

```bash
./scripts/serve.sh --port 9000 --no-browser
```

---

## 8. Необязательно: сайт как служба systemd

Чтобы сайт поднимался автоматически (Linux, systemd пользователя):

```bash
mkdir -p ~/.config/systemd/user
cat > ~/.config/systemd/user/ttd-web.service <<EOF
[Unit]
Description=ttd — локальный сайт загрузчика TikTok
After=network.target

[Service]
ExecStart=$HOME/ttd/ttd web --no-browser
Restart=on-failure

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now ttd-web.service
systemctl --user status ttd-web.service
```

Сайт будет доступен по адресу <http://127.0.0.1:8787/>.

---

## 9. Обновление

```bash
cd ~/ttd
git pull
./scripts/install.sh          # пересоберёт бинарник и обновит движки
```

Обновить только движки:

```bash
ttd install
```

---

## 10. Удаление

```bash
rm -f ~/.local/bin/ttd                 # ссылка в PATH
rm -rf ~/.config/ttd                   # настройки
rm -rf ~/ttd                           # программа и скачанные файлы (!)
```

Каталог `~/ttd` содержит и программу, и библиотеку — удаляйте его только если
готовы потерять скачанное.

---

## Если что-то не работает

**`ttd: команда не найдена`**
Каталог `~/.local/bin` не в `PATH`. Выполните
`export PATH="$HOME/.local/bin:$PATH"` и добавьте строку в `~/.bashrc`, либо
запускайте программу из её каталога: `~/ttd/ttd`.

**`не найден компилятор Go`**
См. пункт 2. Если Go установлен в `~/.local/go`, добавьте его в `PATH`.

**`не удалось создать окружение` / движки не ставятся**
Проверьте наличие Python и доступ в интернет:
`python3 -m venv --help` и `python3 -m pip --version`.

**TikTok отвечает `403` или «Unable to download webpage»**
Возьмите cookies из браузера:

```bash
ttd --cookies-from-browser firefox "https://www.tiktok.com/@user/video/…"
```

**Видео скачивается без звука**
Обновите yt-dlp (`ttd install`) — старые версии иногда теряют аудиодорожку.

**Истории не качаются**
Добавьте флаг `--stories` или укажите ссылку `…/@user/stories` напрямую.

**Порт занят**
Запустите сайт на другом порту: `ttd web --port 9000`. Текущий занятый порт
можно найти командой `ss -ltnp 'sport = :8787'`.

**Нужна подробная диагностика**
Запустите с `-v` (журнал движков) или соберите отчёт `ttd doctor`.
