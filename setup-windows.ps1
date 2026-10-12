# Run as administrator. Adds the firewall rule and the run-at-login task.
# Re-running is safe: both commands replace any existing entry of the same name.
$ErrorActionPreference = "Continue"
$proj = Split-Path -Parent $MyInvocation.MyCommand.Path
$log = Join-Path $proj "setup-windows.log"
$user = "$env:USERDOMAIN\$env:USERNAME"
"=== $(Get-Date) running as $user ===" | Out-File $log -Encoding utf8

netsh advfirewall firewall delete rule name="neoHub Dashboard" | Out-Null
netsh advfirewall firewall add rule name="neoHub Dashboard" dir=in action=allow protocol=TCP localport=8484 profile=private | Out-File $log -Append -Encoding utf8

$vbs = Join-Path $proj "start-hidden.vbs"
schtasks /create /f /tn "neoHub Dashboard" /sc onlogon /ru $user /rl limited /tr "wscript.exe `"$vbs`"" | Out-File $log -Append -Encoding utf8
schtasks /query /tn "neoHub Dashboard" /fo list | Out-File $log -Append -Encoding utf8
"=== done ===" | Out-File $log -Append -Encoding utf8
