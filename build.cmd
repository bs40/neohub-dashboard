@echo off
rem Builds dist\neoHub.exe (Windows tray app), the Raspberry Pi .deb packages and, if Inno Setup
rem is installed, the Windows installer.
setlocal
cd /d "%~dp0"
if "%NEOHUB_VERSION%"=="" set NEOHUB_VERSION=1.3.1
if not exist dist mkdir dist

echo [1/5] exe resources (icon, manifest, version info)
go run github.com/tc-hib/go-winres@v0.3.3 make --in winres\winres.json --out rsrc --product-version %NEOHUB_VERSION% --file-version %NEOHUB_VERSION%
if errorlevel 1 echo    (go-winres failed; building without an exe icon)

echo [2/5] Windows build
go build -trimpath -ldflags "-s -w -H windowsgui -X main.Version=%NEOHUB_VERSION%" -o dist\neoHub.exe .
if errorlevel 1 exit /b 1

echo [3/5] Raspberry Pi builds (linux arm64 + armhf)
if not exist dist\linux-arm64 mkdir dist\linux-arm64
if not exist dist\linux-armhf mkdir dist\linux-armhf
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=arm64
go build -trimpath -ldflags "-s -w -X main.Version=%NEOHUB_VERSION%" -o dist\linux-arm64\neohub .
if errorlevel 1 exit /b 1
set GOARCH=arm
set GOARM=7
go build -trimpath -ldflags "-s -w -X main.Version=%NEOHUB_VERSION%" -o dist\linux-armhf\neohub .
if errorlevel 1 exit /b 1
set GOOS=
set GOARCH=
set GOARM=
set CGO_ENABLED=

echo [4/5] .deb packages
pushd packaging
go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.1 package --config nfpm-arm64.yaml --packager deb --target ..\dist\
if errorlevel 1 echo    (nfpm failed for arm64)
go run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.1 package --config nfpm-armhf.yaml --packager deb --target ..\dist\
if errorlevel 1 echo    (nfpm failed for armhf)
popd

echo [5/5] Windows installer
set ISCC="%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
if not exist %ISCC% set ISCC="%ProgramFiles%\Inno Setup 6\ISCC.exe"
if not exist %ISCC% set ISCC="%LocalAppData%\Programs\Inno Setup 6\ISCC.exe"
if exist %ISCC% (
  %ISCC% /Q installer\neohub.iss
) else (
  echo    Inno Setup not found; skipped. Install it with: winget install JRSoftware.InnoSetup
)
rem Stage the site's downloads next to the page for deploy-bunny.ps1.
if not exist docs\downloads mkdir docs\downloads
rem The package is still called neohub inside; only the file name carries the product name.
if exist dist\neohub_%NEOHUB_VERSION%_arm64.deb move /y dist\neohub_%NEOHUB_VERSION%_arm64.deb dist\neoheat_%NEOHUB_VERSION%_arm64.deb >nul
if exist dist\neohub_%NEOHUB_VERSION%_armhf.deb move /y dist\neohub_%NEOHUB_VERSION%_armhf.deb dist\neoheat_%NEOHUB_VERSION%_armhf.deb >nul
copy /y dist\neoHeat-Setup-%NEOHUB_VERSION%.exe docs\downloads\ >nul 2>&1
copy /y dist\neoheat_%NEOHUB_VERSION%_arm64.deb docs\downloads\ >nul 2>&1
copy /y dist\neoheat_%NEOHUB_VERSION%_armhf.deb docs\downloads\ >nul 2>&1

echo done: dist\
dir /b /s dist
