# Установка и запуск ttd

Пошаговая инструкция: от чистого компьютера до первой скачанной ссылки.
Поддерживаются **Linux**, **macOS** и **Windows 10/11**. На всех трёх
платформах работают CLI, TUI и локальный сайт; отличается только способ
установки.

---

## 1. Что понадобится

| Компонент | Версия | Зачем |
| --- | --- | --- |
| **Go** | 1.22 или новее | собрать саму программу `ttd` |
| **Python** | 3.10 или новее | для движков загрузки (gallery-dl, yt-dlp) |
| Интернет | — | только для самих видео и необязательного `ttd install --online` |

Движки `gallery-dl` и `yt-dlp` вместе с их Python-зависимостями уже встроены в
исходники и бинарник, поэтому для установки движков интернет **не нужен**.

Проверить, что уже установлено:

```bash
go version
python3 --version     # Windows: python --version
```

Если Go не найден — переходите к пункту 2. Если Python не найден — установите
его пакетным менеджером (`sudo dnf install python3`, `sudo apt install python3`,
`brew install python` на macOS, `winget install Python.Python.3.12` на Windows).

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

# macOS (Homebrew)
brew install go

# Windows (PowerShell)
winget install GoLang.Go
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
>
> На Windows проще всего `winget install GoLang.Go` или установщик с
> <https://go.dev/dl/>; после установки откройте новый терминал.

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

### Linux и macOS

```bash
./install.sh
```

### Windows 10/11

В PowerShell (или двойным щелчком по `install.cmd`):

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Если у вас установлен Git for Windows, можно использовать и bash-версию из
Git Bash:

```bash
./install.sh
```

### Что делает установщик

1. находит компилятор Go версии 1.22 или новее;
2. собирает бинарник из `./cmd/ttd` (`ttd` или `ttd.exe`);
3. копирует его в `~/.local/bin` (Windows: `%USERPROFILE%\.local\bin`);
4. запускает `ttd install` — создаёт отдельное окружение и ставит в него
   `gallery-dl` и `yt-dlp` из встроенного набора (интернет не нужен):
   `~/.local/share/ttd/venv` на Linux/macOS, `%APPDATA%\ttd\venv` на Windows;
5. добавляет каталог с бинарником в PATH: `~/.bashrc` и `~/.zshrc` на
   Linux/macOS, PATH пользователя на Windows.

После этого команда `ttd` работает из любого каталога, а каталог с исходниками
можно удалить — движки лежат отдельно, в каталоге данных.

Чтобы изменения `PATH` применились в текущем терминале:

```bash
source ~/.bashrc     # bash
source ~/.zshrc      # zsh
```

или просто откройте новый терминал.

### Параметры установщика

| `install.sh` | `install.ps1` | Что делает |
| --- | --- | --- |
| `--bin-dir <путь>` | `-BinDir <путь>` | куда положить бинарник |
| `--data-dir <путь>` | `-DataDir <путь>` | куда поставить движки |
| `--system` | — | установка в `/usr/local` (через `sudo`) |
| `--no-engines` | `-NoEngines` | не устанавливать движки |
| `--no-rc` | `-NoPath` | не менять PATH |
| `--uninstall` | `-Uninstall` | удалить программу и движки |
| `--purge` | `-Purge` | вместе с удалением убрать каталог данных |

Примеры:

```bash
./install.sh --bin-dir ~/bin --data-dir ~/.local/share/ttd
sudo ./install.sh --system            # в /usr/local/bin для всех пользователей
./install.sh --uninstall              # удалить программу и движки
```

```powershell
.\install.ps1 -BinDir D:\tools\ttd -NoEngines
.\install.ps1 -Uninstall
```

### Сборка вручную (без скрипта)

```bash
go build -trimpath -ldflags "-s -w" -o ttd ./cmd/ttd   # Windows: -o ttd.exe
./ttd install                                          # движки в каталог данных
install -m 0755 ./ttd ~/.local/bin/ttd                 # Linux/macOS
```

---

## 5. Проверка установки

```bash
ttd doctor
```

Ожидаемый вывод — примерно такой:

```
каталог приложения: /home/user/.local/bin
каталог данных:    /home/user/.local/share/ttd
библиотека:        /home/user/ttd
файл настроек:     /home/user/.config/ttd/config.json
кэш библиотеки:    /home/user/ttd/.ttd-cache
python:            /usr/bin/python3
виртуальное окр.:  /home/user/.local/share/ttd/venv (есть)
встроенные движки: gallery-dl 1.32.17, yt-dlp 2026.8.19 (без интернета)
gallery-dl: /home/user/.local/share/ttd/venv/bin/gallery-dl — 1.32.17
yt-dlp: /home/user/.local/share/ttd/venv/bin/yt-dlp — 2026.08.19
поддерживаемые движки: auto, gallery-dl, yt-dlp, mock
```

Строки `gallery-dl` и `yt-dlp` должны содержать версии. Строка «встроенные
движки» показывает набор, который поставляется с бинарником. Если движки не
установлены — повторите `ttd install` (интернет не потребуется).

В Windows тот же вывод будет содержать пути `C:\Users\…\AppData\…`, а движки —
`…\venv\Scripts\gallery-dl.exe` и `…\yt-dlp.exe`. Это нормально.

Если вы собирали программу вручную и запускаете её из каталога с исходниками,
движки могут находиться в `<каталог>/.venv` — это тоже нормально: `ttd` сначала
ищет окружение рядом с исходниками, затем в каталоге данных.

---

## 6. Первая загрузка

```bash
ttd "https://www.tiktok.com/@user/video/7412345678901234567"
```

Результат появится в каталоге с двумя датами:

```
~/ttd/19.10.2026 | 17.10.2026/user_7412345678901234567.mp4
```

В Windows папка называется `19.10.2026 - 17.10.2026` (символ `|` там запрещён):

```
C:\Users\user\ttd\19.10.2026 - 17.10.2026\user_7412345678901234567.mp4
```

Без обращения к TikTok установку можно проверить на встроенном демо-движке:

```bash
ttd --engine mock --dir /tmp/ttd-demo "https://www.tiktok.com/@user/video/123"
ttd list --dir /tmp/ttd-demo --files
```

### Куда сохранять: NAS, облако или разовая папка

`ttd` пишет в смонтированные каталоги, поэтому NAS (SMB/NFS), sshfs и папка
синхронизации Nextcloud/ownCloud подходят без отдельной настройки протоколов.

```bash
# профиль для NAS или облака
ttd config --add-storage nas=/mnt/nas/ttd
ttd config --add-storage cloud=~/Nextcloud/ttd

# сохранить сразу в профиль
ttd --to nas "https://www.tiktok.com/@user/video/7412345678901234567"

# сделать папку постоянной библиотекой
ttd config --set-dir /mnt/nas/ttd

# разовая папка: без дат, индекса и архива
ttd --temp /tmp/ttd-oneoff "https://www.tiktok.com/@user/video/7412345678901234567"
```

Если запустить `ttd` в терминале без `--to/--temp/--dir`, после загрузки
появится вопрос, куда сохранить файлы (оставить, профиль, разовая папка,
сделать постоянной). В TUI и на сайте этот выбор показывается автоматически.
Отключить: `ttd --no-ask` или `ttd config --no-ask-destination`.

Если каталог профиля не существует (NAS не смонтирован), `ttd` сообщит об
ошибке и не станет создавать пустую локальную папку.

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
ttd --to nas <ссылка>                           # сразу на NAS/в облако
ttd --temp /tmp/ttd-oneoff <ссылка>             # разовая папка
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
# или (Linux/macOS/Git Bash):
./scripts/serve.sh
```

Откройте указанный в выводе адрес (по умолчанию <http://127.0.0.1:8787/>).
Скрипт `scripts/serve.sh` при необходимости сначала собирает бинарник и затем
запускает сервер; любые аргументы передаются дальше:

```bash
./scripts/serve.sh --port 9000 --no-browser
```

В Windows используйте `ttd web` (или `scripts/serve.sh` из Git Bash).

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
ExecStart=%h/.local/bin/ttd web --no-browser
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
./install.sh                  # пересоберёт бинарник и обновит движки
```

На Windows:

```powershell
cd ttd
git pull
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

Если исходники уже удалены, а обновлять нужно только движки:

```bash
ttd install              # из встроенного набора, без интернета
ttd install --online     # свежие версии с PyPI (нужен интернет)
ttd install --wheels ~/ttd-wheels   # офлайн из своих колёс
```

Важно: `ttd install --online` заменяет встроенные версии более свежими — это
первое, что стоит попробовать, если TikTok перестал скачиваться после
очередного изменения сайта.

---

## 10. Удаление

Linux/macOS:

```bash
./install.sh --uninstall      # бинарник, движки и строки PATH в ~/.bashrc и ~/.zshrc
./install.sh --uninstall --purge   # плюс весь каталог данных ~/.local/share/ttd
```

Windows (PowerShell):

```powershell
.\install.ps1 -Uninstall
.\install.ps1 -Uninstall -Purge
```

Вручную:

```bash
rm -f ~/.local/bin/ttd                 # программа (Windows: ttd.exe)
rm -rf ~/.local/share/ttd              # движки и данные (Windows: %APPDATA%\ttd)
rm -rf ~/.config/ttd                   # настройки (Windows: %APPDATA%\ttd\config.json)
rm -rf ~/ttd                           # скачанные файлы (!)
```

Каталог `~/ttd` по умолчанию содержит скачанные файлы. Удаляйте его только если
готовы потерять библиотеку.

---

## Если что-то не работает

**`ttd: команда не найдена`**
Каталог `~/.local/bin` не в `PATH`. Выполните
`source ~/.bashrc` (или `source ~/.zshrc`) либо откройте новый терминал. Если
строки с `PATH` нет — запустите `./install.sh` ещё раз или добавьте вручную:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
```

В Windows откройте новый терминал после `install.ps1` и проверьте:
`ttd doctor`. PATH пользователя можно посмотреть так:

```powershell
[Environment]::GetEnvironmentVariable('Path', 'User')
```

**Windows: `install.ps1` не запускается («выполнение сценариев отключено»)**
Это стандартная политика PowerShell. Запускайте так:

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

или двойным щелчком по `install.cmd`.

**Windows: в TUI не работают стрелки/F-клавиши**
Используйте Windows Terminal (рекомендуется) или conhost из Windows 10+.
Если консоль не поддерживает виртуальный терминал, `ttd tui` автоматически
перейдёт в построчный режим — загрузка при этом работает.

**Windows: «не найден python3» при `ttd install`**
Установите Python 3.10+ (`winget install Python.Python.3.12`) и откройте новый
терминал. Установщик ищет `python3`, `python` и `py`.

**`нужен Python 3.10 или новее`**
Встроенный yt-dlp требует Python 3.10+. Обновите Python; на Windows это
`winget upgrade Python.Python.3.12`.

**`ttd` работает, но «движки не найдены» после переноса каталога**
Движки ищутся в каталоге данных: `~/.local/share/ttd/venv` на Linux/macOS,
`%APPDATA%\ttd\venv` на Windows. Запустите `ttd install` — он поставит их туда
независимо от места бинарника.

**`go.mod requires go >= …`**
Установщик выбирает Go 1.22+ автоматически. Если в `PATH` оказался старый Go,
укажите новый явно:

```bash
PATH="$HOME/.local/go/bin:$PATH" ./install.sh
```

**`не найден компилятор Go`**
См. пункт 2. Если Go установлен в `~/.local/go`, добавьте его в `PATH`.

**`не удалось создать окружение` / движки не ставятся**
Проверьте, что Python 3.10+ и модуль `venv` доступны:
`python3 -m venv --help` и `python3 -m pip --version`. Интернет для установки
движков не нужен — они встроены; если нужна свежая версия, выполните
`ttd install --online`.

**TikTok отвечает `403` или «Unable to download webpage»**
Возьмите cookies из браузера:

```bash
ttd --cookies-from-browser firefox "https://www.tiktok.com/@user/video/…"
```

**Видео скачивается без звука**
Обновите yt-dlp (`ttd install --online`) — старые версии иногда теряют
аудиодорожку. Если интернета нет, используйте `ttd install --wheels <каталог>`.

**Истории не качаются**
Добавьте флаг `--stories` или укажите ссылку `…/@user/stories` напрямую.

**Порт занят**
Запустите сайт на другом порту: `ttd web --port 9000`. Текущий занятый порт
можно найти командой `ss -ltnp 'sport = :8787'`.

**Нужна подробная диагностика**
Запустите с `-v` (журнал движков) или соберите отчёт `ttd doctor`.
