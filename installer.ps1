param(
    [string]$Version = "",
    [string]$Arch = "amd64"
)

# Builds the per-user installer from an already compiled dist executable.
# The release workflow calls this after build.ps1; local builds can call it
# with the same Version and Arch to reproduce the asset.
#
# Inno Setup is not a prerequisite: an installed copy is used when present,
# otherwise a pinned compiler is downloaded into dist\tools (gitignored) and
# installed per-user there. Nothing is asked from the user.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$innoVersion = "6.7.3"
$innoSha256 = "9c73c3bae7ed48d44112a0f48e66742c00090bdb5bef71d9d3c056c66e97b732"
$innoUrl = "https://github.com/jrsoftware/issrc/releases/download/is-6_7_3/innosetup-$innoVersion.exe"

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

$exe = Join-Path $PSScriptRoot "dist\yoanime_${tag}_${Arch}.exe"
if (-not (Test-Path $exe)) {
    throw "Missing $exe; run build.ps1 -Version $tag -Arch $Arch first"
}

function Get-Sha256([string]$Path) {
    return (Get-FileHash -Path $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

# Downloads the pinned compiler, verifies its checksum and installs it into the
# ignored dist\tools directory. The Inno Setup installer supports a per-user
# silent install, so no administrator rights are needed.
function Install-InnoSetup {
    $toolsDir = Join-Path $PSScriptRoot "dist\tools"
    $target = Join-Path $toolsDir "innosetup-$innoVersion"
    $setup = Join-Path $toolsDir "innosetup-$innoVersion.exe"
    New-Item -ItemType Directory -Force $toolsDir | Out-Null

    if (-not (Test-Path $setup) -or (Get-Sha256 $setup) -ne $innoSha256) {
        Write-Host "Downloading Inno Setup $innoVersion..."
        $previous = $ProgressPreference
        $ProgressPreference = "SilentlyContinue"
        try {
            Invoke-WebRequest -Uri $innoUrl -OutFile $setup -UseBasicParsing
        } finally {
            $ProgressPreference = $previous
        }
    }

    $actual = Get-Sha256 $setup
    if ($actual -ne $innoSha256) {
        Remove-Item -Force $setup -ErrorAction SilentlyContinue
        throw "Inno Setup checksum mismatch: expected $innoSha256, got $actual"
    }

    Write-Host "Installing Inno Setup $innoVersion into $target..."
    # A half-removed cache would make the installer stop on a non-empty directory.
    if (Test-Path $target) {
        Remove-Item $target -Recurse -Force -ErrorAction SilentlyContinue
    }
    $process = Start-Process -FilePath $setup -Wait -PassThru -ArgumentList @(
        "/VERYSILENT",
        "/SUPPRESSMSGBOXES",
        "/NORESTART",
        "/SP-",
        # Official switch for Inno Setup's own installer: unpack into /DIR
        # without registering an uninstall entry or shortcuts.
        "/PORTABLE=1",
        "/DIR=`"$target`""
    )
    if ($process.ExitCode -ne 0) {
        throw "Inno Setup installer failed with exit code $($process.ExitCode)"
    }

    $iscc = Join-Path $target "ISCC.exe"
    if (-not (Test-Path $iscc)) {
        throw "Inno Setup was installed but $iscc is missing"
    }
    return $iscc
}

$compiler = Join-Path $PSScriptRoot "dist\tools\innosetup-$innoVersion\ISCC.exe"
if (-not (Test-Path $compiler)) {
    $command = Get-Command "iscc.exe" -ErrorAction SilentlyContinue
    if ($command) {
        $compiler = $command.Source
    } else {
        $candidates = @(
            (Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"),
            (Join-Path $env:ProgramFiles "Inno Setup 6\ISCC.exe"),
            (Join-Path $env:LOCALAPPDATA "Programs\Inno Setup 6\ISCC.exe")
        )
        $compiler = $candidates | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    }
}
if (-not $compiler) {
    $compiler = Install-InnoSetup
}

& $compiler "/DVersionTag=$tag" "/DVersionNumber=$number" "/DArch=$Arch" "installer\yoanime.iss"
if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup failed with exit code $LASTEXITCODE"
}

$setup = Join-Path $PSScriptRoot "dist\yoanime_${tag}_${Arch}_setup.exe"
if (-not (Test-Path $setup)) {
    throw "Expected installer was not produced: $setup"
}

# Code signing is optional: it only runs when a certificate is configured, so
# the build keeps working for contributors without one.
if ($env:YOANIME_SIGN_PFX -or $env:YOANIME_SIGN_PFX_BASE64 -or $env:YOANIME_SIGN_THUMBPRINT) {
    & (Join-Path $PSScriptRoot "sign.ps1") -Path $setup
}

Write-Output $setup
