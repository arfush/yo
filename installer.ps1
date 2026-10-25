param(
    [string]$Version = "",
    [string]$Arch = "amd64"
)

# Builds the per-user installer from an already compiled dist executable.
# The release workflow calls this after build.ps1; local builds can call it
# with the same Version and Arch to reproduce the asset.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (-not $Version) {
    $versionFile = Join-Path $PSScriptRoot "internal\version\version.go"
    $match = [regex]::Match((Get-Content $versionFile -Raw), 'Version\s*=\s*"([^"]+)"')
    if (-not $match.Success) {
        throw "Cannot read the default version from $versionFile"
    }
    $Version = $match.Groups[1].Value
}
$Version = $Version.Trim()
$number = $Version
if ($number -match '^[vV]') {
    $number = $number.Substring(1)
}
if (-not $number) {
    throw "Version is empty"
}
$tag = "v$number"

$exe = Join-Path $PSScriptRoot "dist\yonime_${tag}_${Arch}.exe"
if (-not (Test-Path $exe)) {
    throw "Missing $exe; run build.ps1 -Version $tag -Arch $Arch first"
}

$iscc = Get-Command "iscc.exe" -ErrorAction SilentlyContinue
if ($iscc) {
    $compiler = $iscc.Source
} else {
    $candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
        (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe"),
        (Join-Path $env:LOCALAPPDATA "Programs\Inno Setup 6\ISCC.exe")
    )
    $compiler = $candidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
}
if (-not $compiler) {
    throw "Inno Setup 6 not found. Install it from https://jrsoftware.org/isdl.php or run: choco install innosetup"
}

& $compiler "/DVersionTag=$tag" "/DVersionNumber=$number" "/DArch=$Arch" "installer\yonime.iss"
if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup failed with exit code $LASTEXITCODE"
}

$setup = Join-Path $PSScriptRoot "dist\yonime_${tag}_${Arch}_setup.exe"
if (-not (Test-Path $setup)) {
    throw "Expected installer was not produced: $setup"
}
Write-Output $setup
