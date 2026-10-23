param(
    [string]$Version = "",
    [string]$Repo = "arfush/yonime",
    [string]$Arch = "amd64",
    [string]$Output = ""
)

# Builds a stripped Windows GUI executable. The release workflow passes the
# version, repository and architecture; local builds default to the current
# machine's architecture and yonime.exe.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not $Arch) {
    $Arch = "amd64"
}
if ($Version) {
    $Version = $Version.Trim()
    if ($Version -match '^[vV]') {
        $Version = "v" + $Version.Substring(1)
    } else {
        $Version = "v$Version"
    }
}

if (-not $Output) {
    if ($Version) {
        $Output = "dist\yonime_${Version}_${Arch}.exe"
    } else {
        $Output = "yonime.exe"
    }
}

$outDir = Split-Path -Parent $Output
if ($outDir) {
    New-Item -ItemType Directory -Force $outDir | Out-Null
}

$ldflags = "-s -w -H=windowsgui"
if ($Version) {
    $ldflags += " -X github.com/arfush/yonime/internal/version.Version=$Version"
}
if ($Repo) {
    $ldflags += " -X github.com/arfush/yonime/internal/updates.Repository=$Repo"
}

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = $Arch

go build -trimpath -ldflags="$ldflags" -o $Output .\cmd\yonime

$size = (Get-Item $Output).Length
Write-Host ("Built {0}: {1:N1} MB" -f $Output, ($size / 1MB))
