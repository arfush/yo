# Signs Windows binaries with an Authenticode certificate.
#
# The certificate comes from the environment so build.ps1 and installer.ps1 can
# call this script without arguments:
#
#   YONIME_SIGN_PFX          path to a .pfx file
#   YONIME_SIGN_PFX_BASE64   base64-encoded .pfx, for CI secrets
#   YONIME_SIGN_PFX_PASSWORD password for the .pfx (may be empty)
#   YONIME_SIGN_THUMBPRINT   thumbprint of a certificate already in the store
#
# A trusted signature requires a certificate issued by a public CA; a
# self-signed certificate only helps on machines that trust it explicitly.
param(
    [Parameter(Mandatory = $true, Position = 0, ValueFromRemainingArguments = $true)]
    [string[]]$Path,
    [string]$Thumbprint = $env:YONIME_SIGN_THUMBPRINT,
    [string]$PfxPath = $env:YONIME_SIGN_PFX,
    [string]$PfxBase64 = $env:YONIME_SIGN_PFX_BASE64,
    [string]$PfxPassword = $env:YONIME_SIGN_PFX_PASSWORD,
    [string]$TimestampServer = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

$tempPfx = $null
$importedThumbprint = $null

try {
    if ($PfxBase64) {
        $tempPfx = Join-Path ([IO.Path]::GetTempPath()) ("yonime-sign-" + [guid]::NewGuid().ToString("N") + ".pfx")
        [IO.File]::WriteAllBytes($tempPfx, [Convert]::FromBase64String($PfxBase64))
        $PfxPath = $tempPfx
    }

    if ($PfxPath) {
        $secure = $null
        if ($PfxPassword) {
            $secure = ConvertTo-SecureString -String $PfxPassword -AsPlainText -Force
        }
        $importParameters = @{
            FilePath          = $PfxPath
            CertStoreLocation = "Cert:\CurrentUser\My"
        }
        if ($secure) {
            $importParameters.Password = $secure
        }
        $before = @(Get-ChildItem "Cert:\CurrentUser\My" | Select-Object -ExpandProperty Thumbprint)
        $imported = Import-PfxCertificate @importParameters
        if (-not $imported) {
            throw "Import-PfxCertificate did not return a certificate for $PfxPath"
        }
        $certificate = Get-Item "Cert:\CurrentUser\My\$($imported.Thumbprint)"
        # Only remove the certificate again when the import created it.
        if ($before -notcontains $imported.Thumbprint) {
            $importedThumbprint = $imported.Thumbprint
        }
    } elseif ($Thumbprint) {
        $certificate = Get-ChildItem "Cert:\CurrentUser\My", "Cert:\LocalMachine\My" -ErrorAction SilentlyContinue |
            Where-Object { $_.Thumbprint -eq $Thumbprint } |
            Select-Object -First 1
        if (-not $certificate) {
            throw "No code signing certificate with thumbprint $Thumbprint was found"
        }
    } else {
        throw "No signing certificate configured. Set YONIME_SIGN_PFX, YONIME_SIGN_PFX_BASE64 or YONIME_SIGN_THUMBPRINT."
    }

    foreach ($file in $Path) {
        if (-not (Test-Path $file)) {
            throw "Cannot sign missing file: $file"
        }
        $result = Set-AuthenticodeSignature -FilePath $file -Certificate $certificate -HashAlgorithm SHA256 -TimestampServer $TimestampServer
        switch ($result.Status) {
            "Valid" {
                Write-Host "Signed $file"
            }
            "NotTrusted" {
                Write-Warning "Signed $file, but the certificate chain is not trusted (test certificate?)"
            }
            "UnknownError" {
                Write-Warning "Signed $file, but Windows could not verify the certificate chain: $($result.StatusMessage)"
            }
            default {
                throw "Signing $file failed: $($result.Status) $($result.StatusMessage)"
            }
        }
    }
} finally {
    if ($tempPfx) {
        Remove-Item -Force $tempPfx -ErrorAction SilentlyContinue
    }
    if ($importedThumbprint) {
        Remove-Item -Path "Cert:\CurrentUser\My\$importedThumbprint" -Force -ErrorAction SilentlyContinue
    }
}
