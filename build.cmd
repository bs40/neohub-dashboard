@echo off
rem Builds dist\neoHub.exe (tray app, no console) and, if Inno Setup is installed, the installer.
setlocal
cd /d "%~dp0"
if "%NEOHUB_VERSION%"=="" set NEOHUB_VERSION=1.1.0

echo [1/3] exe resources (icon, manifest, version info)
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres\winres.json --out rsrc --product-version %NEOHUB_VERSION% --file-version %NEOHUB_VERSION%
if errorlevel 1 echo    (go-winres failed; building without an exe icon)

echo [2/3] go build
if not exist dist mkdir dist
go build -trimpath -ldflags "-s -w -H windowsgui -X main.Version=%NEOHUB_VERSION%" -o dist\neoHub.exe .
if errorlevel 1 exit /b 1

echo [3/3] installer
set ISCC="%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not exist %ISCC% set ISCC="%ProgramFiles%\Inno Setup 6\ISCC.exe"
if not exist %ISCC% set ISCC="%LocalAppData%\Programs\Inno Setup 6\ISCC.exe"
if exist %ISCC% (
  %ISCC% /Q installer\neohub.iss
) else (
  echo    Inno Setup not found; skipped. Install it with: winget install JRSoftware.InnoSetup
)
echo done: dist\
dir /b dist
