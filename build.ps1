param(
    [string]$Version = "",
    [string]$Repo = "arfush/yo",
    [string]$Arch = "amd64",
    [string]$Output = ""
)

# Builds a stripped Windows GUI executable. The release workflow passes the
# version, repository and architecture; local builds default to the current
# machine's architecture and yoanime.exe.
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
        $Output = "dist\yoanime_${Version}_${Arch}.exe"
    } else {
        $Output = "yoanime.exe"
    }
}

$outDir = Split-Path -Parent $Output
if ($outDir) {
    New-Item -ItemType Directory -Force $outDir | Out-Null
}

$ldflags = "-s -w -H=windowsgui"
if ($Version) {
    $ldflags += " -X github.com/arfush/yo/version.Version=$Version"
}
if ($Repo) {
    $ldflags += " -X github.com/arfush/yo/update.Repository=$Repo"
}

# Party relays are deployed once and baked into the official build so the
# shipped exe needs no party.json. Environment variables win; the rest comes
# from party.json in the repository root. That file is never written here.
$partyFile = Join-Path $PSScriptRoot "party.json"
$partyDefaults = $null
if (Test-Path $partyFile) {
    try {
        $partyDefaults = Get-Content -Raw -Path $partyFile | ConvertFrom-Json
    } catch {
        Write-Warning "party.json: $($_.Exception.Message)"
    }
}

$partyFlags = @(
    @{ Env = "YOANIME_PARTY_CLOUDFLARE_URL"; Field = "cloudflare_url"; Symbol = "CloudflareURL" },
    @{ Env = "YOANIME_PARTY_SUPABASE_URL"; Field = "supabase_url"; Symbol = "SupabaseURL" },
    @{ Env = "YOANIME_PARTY_SUPABASE_KEY"; Field = "supabase_key"; Symbol = "SupabaseKey" }
)
$partyBaked = @()
foreach ($flag in $partyFlags) {
    $value = [Environment]::GetEnvironmentVariable($flag.Env)
    $source = $flag.Env
    if (-not $value -and $partyDefaults) {
        $value = $partyDefaults.$($flag.Field)
        $source = "party.json"
    }
    if (-not $value) {
        continue
    }
    $value = $value.Trim()
    if (-not $value) {
        continue
    }
    # The repository file ships placeholders; baking one would give users a dead
    # relay that reports "room not found" instead of "not configured".
    if ($value -match 'example\.|abcdefghijkl|\.\.\.$') {
        Write-Warning "party: $($flag.Field) is still the party.json example, skip it"
        continue
    }
    $ldflags += " -X github.com/arfush/yo/party.$($flag.Symbol)=$value"
    $partyBaked += "$($flag.Field) ($source)"
}
if ($partyBaked.Count -gt 0) {
    Write-Host ("Party relay: " + ($partyBaked -join ", "))
}

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = $Arch

go build -trimpath -ldflags="$ldflags" -o $Output .\cmd\yoanime

$size = (Get-Item $Output).Length
Write-Host ("Built {0}: {1:N1} MB" -f $Output, ($size / 1MB))

# Signing is optional and controlled by environment variables documented in
# sign.ps1, so a build without a certificate still succeeds.
if ($env:YOANIME_SIGN_PFX -or $env:YOANIME_SIGN_PFX_BASE64 -or $env:YOANIME_SIGN_THUMBPRINT) {
    & (Join-Path $PSScriptRoot "sign.ps1") -Path $Output
}
