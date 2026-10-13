; Inno Setup script for neoHub. Build with:  ISCC installer\neohub.iss
; Expects dist\neoHub.exe to exist (see build.cmd).

#define AppName "neoHub"
#define AppVersion GetEnv("NEOHUB_VERSION")
#if AppVersion == ""
  #define AppVersion "1.2.2"
#endif

[Setup]
AppId={{7D5C2C6E-2B6B-4C6A-9B1E-5A0C3F1E7B21}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=bs40
AppPublisherURL=https://github.com/bs40/neohub-dashboard
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=neoHub-Setup-{#AppVersion}
SetupIconFile=..\assets\neohub.ico
UninstallDisplayIcon={app}\neoHub.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no

[Tasks]
Name: "startup"; Description: "Start {#AppName} when you sign in to Windows"; GroupDescription: "Startup:"
Name: "firewall"; Description: "Allow phones and tablets on your home network to open the dashboard (adds a Windows Firewall rule for TCP 8484)"; GroupDescription: "Network:"
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "..\dist\neoHub.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\neoHub.exe"
Name: "{group}\Open neoHub dashboard"; Filename: "http://localhost:8484/"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\neoHub.exe"; Tasks: desktopicon

[Registry]
; Per-user autostart. The tray menu's "Start with Windows" toggles the same value.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "neoHub"; ValueData: """{app}\neoHub.exe"""; Flags: uninsdeletevalue; Tasks: startup

[Run]
; Retire the Node-based dashboard if it was set up by the earlier script, so the port is free.
Filename: "schtasks.exe"; Parameters: "/end /tn ""neoHub Dashboard"""; Flags: runhidden; StatusMsg: "Stopping the previous dashboard..."
Filename: "schtasks.exe"; Parameters: "/delete /tn ""neoHub Dashboard"" /f"; Flags: runhidden
Filename: "powershell.exe"; Parameters: "-NoProfile -Command ""Get-CimInstance Win32_Process -Filter \""Name='node.exe'\"" | Where-Object {{ $_.CommandLine -like '*neohub-dashboard*server.js*' } | ForEach-Object {{ Stop-Process -Id $_.ProcessId -Force }"""; Flags: runhidden
Filename: "netsh.exe"; Parameters: "advfirewall firewall delete rule name=""neoHub Dashboard"""; Flags: runhidden
Filename: "netsh.exe"; Parameters: "advfirewall firewall add rule name=""neoHub"" dir=in action=allow protocol=TCP localport=8484 profile=private program=""{app}\neoHub.exe"""; Flags: runhidden; Tasks: firewall; StatusMsg: "Adding firewall rule..."
Filename: "{app}\neoHub.exe"; Parameters: "-open"; Description: "Start {#AppName} now and open the dashboard"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "taskkill.exe"; Parameters: "/IM neoHub.exe /F"; Flags: runhidden; RunOnceId: "killapp"
Filename: "netsh.exe"; Parameters: "advfirewall firewall delete rule name=""neoHub"""; Flags: runhidden; RunOnceId: "fw"

[UninstallDelete]
Type: filesandordirs; Name: "{localappdata}\neoHub"
