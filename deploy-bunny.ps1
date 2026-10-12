<#
Uploads the site in docs\ (page, screenshots and downloads) to a Bunny.net storage zone.

Usage:
  $env:BUNNY_STORAGE_KEY = "<storage zone password / FTP & API access key>"
  .\deploy-bunny.ps1 -Zone my-zone-name              # region "" = Falkenstein (storage.bunnycdn.com)
  .\deploy-bunny.ps1 -Zone my-zone-name -Region uk   # uk, ny, la, sg, se, br, jh, syd ...
  .\deploy-bunny.ps1 -Zone my-zone-name -Path neohub # upload under a sub-folder of the zone

Run build.cmd first so docs\downloads holds the current packages. Existing files with the
same name are overwritten; nothing is deleted. Purge the pull zone afterwards if files were
replaced (Bunny dashboard > Pull zone > Purge), or use -Purge with BUNNY_API_KEY set.
#>
param(
  [Parameter(Mandatory = $true)][string]$Zone,
  [string]$Region = "",
  [string]$Path = "",
  [switch]$Purge,
  [string]$PullZoneHost = ""
)
$ErrorActionPreference = "Stop"
$key = $env:BUNNY_STORAGE_KEY
if (-not $key) { throw "Set BUNNY_STORAGE_KEY to the storage zone's access key first." }
$hostName = if ($Region) { "$Region.storage.bunnycdn.com" } else { "storage.bunnycdn.com" }
$base = "https://$hostName/$Zone/" + ($(if ($Path) { $Path.Trim('/') + '/' } else { '' }))
$root = Join-Path $PSScriptRoot "docs"
if (-not (Test-Path (Join-Path $root "index.html"))) { throw "docs\index.html not found; run build.cmd first." }

$files = Get-ChildItem -Path $root -Recurse -File | Where-Object { $_.Name -ne "page.html" }
$total = 0
foreach ($f in $files) {
  $rel = $f.FullName.Substring($root.Length + 1).Replace('\', '/')
  $url = $base + $rel
  $type = switch ($f.Extension.ToLower()) {
    ".html" { "text/html; charset=utf-8" } ".jpg" { "image/jpeg" } ".png" { "image/png" }
    ".exe" { "application/octet-stream" } ".deb" { "application/vnd.debian.binary-package" }
    default { "application/octet-stream" }
  }
  Write-Host ("  {0,-45} {1,8:N1} MB" -f $rel, ($f.Length / 1MB))
  Invoke-WebRequest -Uri $url -Method Put -InFile $f.FullName -ContentType $type -Headers @{ AccessKey = $key } -UseBasicParsing | Out-Null
  $total += $f.Length
}
Write-Host ("uploaded {0} files, {1:N1} MB, to {2}" -f $files.Count, ($total / 1MB), $base)

if ($Purge) {
  $api = $env:BUNNY_API_KEY
  if (-not $api -or -not $PullZoneHost) { Write-Warning "Purge needs BUNNY_API_KEY and -PullZoneHost (e.g. neohub.b-cdn.net); skipped."; return }
  $purgeUrl = "https://api.bunny.net/purge?url=" + [uri]::EscapeDataString("https://$PullZoneHost/*")
  Invoke-WebRequest -Uri $purgeUrl -Method Post -Headers @{ AccessKey = $api } -UseBasicParsing | Out-Null
  Write-Host "purged https://$PullZoneHost/*"
}
