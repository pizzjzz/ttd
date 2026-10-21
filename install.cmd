@echo off
rem Windows installer wrapper: runs install.ps1 with the right execution policy.
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
exit /b %ERRORLEVEL%
