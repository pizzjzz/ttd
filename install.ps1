<#
.SYNOPSIS
    Установщик ttd для Windows (PowerShell 5.1+).

.DESCRIPTION
    Собирает ttd.exe, ставит его в каталог пользователя, добавляет этот каталог
    в PATH и устанавливает движки gallery-dl и yt-dlp в отдельное окружение.
    После установки команда ttd работает из cmd, PowerShell и Windows Terminal
    независимо от того, где лежат исходники.

.PARAMETER BinDir
    Куда положить ttd.exe. По умолчанию %USERPROFILE%\.local\bin.

.PARAMETER DataDir
    Куда поставить движки. По умолчанию %APPDATA%\ttd.

.PARAMETER NoEngines
    Не устанавливать gallery-dl и yt-dlp.

.PARAMETER NoPath
    Не добавлять BinDir в PATH пользователя.

.PARAMETER Uninstall
    Удалить ttd.exe, движки и запись в PATH.

.PARAMETER Purge
    Вместе с -Uninstall удалить весь каталог данных.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1

.EXAMPLE
    .\install.ps1 -BinDir D:\tools\ttd -NoEngines

.EXAMPLE
    .\install.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [string]$BinDir = (Join-Path $HOME '.local\bin'),
    [string]$DataDir = (Join-Path $env:APPDATA 'ttd'),
    [switch]$NoEngines,
    [switch]$NoPath,
    [switch]$Uninstall,
    [switch]$Purge
)

$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$Exe = Join-Path $BinDir 'ttd.exe'
$Venv = Join-Path $DataDir 'venv'

function Write-Step([string]$Text) {
    Write-Host "==> $Text" -ForegroundColor Cyan
}

function Get-UserPath {
    [Environment]::GetEnvironmentVariable('Path', 'User')
}

function Add-ToUserPath([string]$Directory) {
    $current = Get-UserPath
    $parts = @()
    if ($current) {
        $parts = @($current -split ';' | Where-Object { $_ -ne '' })
    }
    $already = $false
    foreach ($part in $parts) {
        if ($part.TrimEnd('\') -ieq $Directory.TrimEnd('\')) { $already = $true }
    }
    if (-not $already) {
        $newPath = (@($parts) + $Directory) -join ';'
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Write-Host "    добавлен в PATH пользователя: $Directory"
    }
    else {
        Write-Host "    уже в PATH: $Directory"
    }
    $sessionParts = @($env:Path -split ';')
    if ($sessionParts -notcontains $Directory) {
        $env:Path = "$Directory;$env:Path"
    }
}

function Remove-FromUserPath([string]$Directory) {
    $current = Get-UserPath
    if (-not $current) { return }
    $parts = @($current -split ';' | Where-Object { $_ -ne '' })
    $kept = @($parts | Where-Object { $_.TrimEnd('\') -ine $Directory.TrimEnd('\') })
    if ($kept.Count -ne $parts.Count) {
        [Environment]::SetEnvironmentVariable('Path', ($kept -join ';'), 'User')
        Write-Host "    убран из PATH пользователя: $Directory"
    }
}

# ---------------------------------------------------------------------------
# Удаление
# ---------------------------------------------------------------------------
if ($Uninstall) {
    Write-Step 'Удаляю ttd'
    if (Test-Path -LiteralPath $Exe) {
        Remove-Item -LiteralPath $Exe -Force
        Write-Host "    удалён $Exe"
    }
    else {
        Write-Host "    $Exe не найден"
    }
    Remove-FromUserPath $BinDir
    if (Test-Path -LiteralPath $Venv) {
        Remove-Item -LiteralPath $Venv -Recurse -Force
        Write-Host "    удалены движки из $Venv"
    }
    if ($Purge -and (Test-Path -LiteralPath $DataDir)) {
        Remove-Item -LiteralPath $DataDir -Recurse -Force
        Write-Host "    удалён каталог данных $DataDir"
    }
    Write-Host ''
    Write-Host 'Готово. Библиотека со скачанными файлами не тронута.'
    exit 0
}

# ---------------------------------------------------------------------------
# Поиск Go
# ---------------------------------------------------------------------------
$go = $null
$fromPath = Get-Command go -ErrorAction SilentlyContinue
if ($fromPath) {
    $go = $fromPath.Source
    if (-not $go) { $go = $fromPath.Path }
}
if (-not $go) {
    $candidates = @(
        (Join-Path $HOME '.local\go\bin\go.exe'),
        'C:\Go\bin\go.exe',
        (Join-Path $env:ProgramFiles 'Go\bin\go.exe')
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate) { $go = $candidate; break }
    }
}
if (-not $go) {
    Write-Error @'
Go не найден. Установите Go 1.22 или новее:
  winget install GoLang.Go
или скачайте установщик с https://go.dev/dl/ и запустите install.ps1 снова.
'@
    exit 1
}

Write-Step "Платформа:  Windows ($([Environment]::OSVersion.VersionString))"
Write-Step "Компилятор: $(& $go version)"
Write-Step "Исходники:  $Root"

# ---------------------------------------------------------------------------
# Сборка
# ---------------------------------------------------------------------------
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ('ttd-build-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    Write-Step 'Собираю ttd'
    Push-Location $Root
    try {
        & $go build -trimpath -ldflags '-s -w' -o (Join-Path $tmp 'ttd.exe') ./cmd/ttd
        $buildCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    if ($buildCode -ne 0) {
        Write-Error "Сборка завершилась с кодом $buildCode"
        exit 1
    }
    Write-Step "Устанавливаю $Exe"
    Copy-Item -LiteralPath (Join-Path $tmp 'ttd.exe') -Destination $Exe -Force
}
finally {
    Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# ---------------------------------------------------------------------------
# Движки
# ---------------------------------------------------------------------------
$enginesOk = $true
if ($NoEngines) {
    Write-Step 'Движки пропущены (-NoEngines)'
}
else {
    Write-Step "Устанавливаю движки (gallery-dl, yt-dlp) в $Venv"
    $env:TTD_DATA_DIR = $DataDir
    & $Exe install
    if ($LASTEXITCODE -ne 0) {
        $enginesOk = $false
        Write-Warning "Движки установить не удалось. Повторите позже: `$env:TTD_DATA_DIR='$DataDir'; ttd install"
    }
}

# ---------------------------------------------------------------------------
# PATH
# ---------------------------------------------------------------------------
if (-not $NoPath) {
    Write-Step 'Настраиваю PATH'
    Add-ToUserPath $BinDir
}

# ---------------------------------------------------------------------------
# Проверка
# ---------------------------------------------------------------------------
Write-Host ''
Write-Step 'Проверка установки'
$env:TTD_DATA_DIR = $DataDir
& $Exe --version
if ($LASTEXITCODE -ne 0) {
    Write-Error 'Установленный ttd.exe не запускается'
    exit 1
}
if ($NoEngines) {
    & $Exe doctor
}

Write-Host ''
Write-Host '==========================================================================' -ForegroundColor Green
Write-Host " ttd установлен: $Exe"
if (-not $NoPath) {
    Write-Host ' Откройте новый терминал (или перезапустите Windows Terminal),'
    Write-Host ' чтобы команда ttd стала доступна в cmd и PowerShell.'
}
else {
    Write-Host " Добавьте каталог в PATH вручную: $BinDir"
}
if (-not $enginesOk) {
    Write-Host ''
    Write-Host ' ВНИМАНИЕ: движки не установлены — загрузка пока не работает.' -ForegroundColor Yellow
    Write-Host " Повторите:  `$env:TTD_DATA_DIR='$DataDir'; ttd install"
}
Write-Host ''
Write-Host ' Проверка:  ttd doctor'
Write-Host ' Загрузка:  ttd "https://www.tiktok.com/@user/video/..."'
Write-Host ' Сайт:      ttd web'
Write-Host '==========================================================================' -ForegroundColor Green

if (-not $enginesOk) { exit 1 }
