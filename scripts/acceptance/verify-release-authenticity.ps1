[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$ArtifactDir,
    [string]$ReleaseBase
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0

function Fail([string]$Message) {
    throw "release authenticity verification failed: $Message"
}

function Get-Sha256([string]$Path) {
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $algorithm = [System.Security.Cryptography.SHA256]::Create()
        try {
            return ([BitConverter]::ToString($algorithm.ComputeHash($stream))).Replace('-', '').ToLowerInvariant()
        } finally {
            $algorithm.Dispose()
        }
    } finally {
        $stream.Dispose()
    }
}

if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$') {
    Fail 'version must be an exact immutable vX.Y.Z tag, never latest'
}
if ([string]::IsNullOrWhiteSpace($ArtifactDir) -eq [string]::IsNullOrWhiteSpace($ReleaseBase)) {
    Fail 'choose exactly one mode: -ArtifactDir or -ReleaseBase'
}
if (-not (Get-Command cosign -ErrorAction SilentlyContinue)) {
    Fail 'cosign is required'
}

$plainVersion = $Version.Substring(1)
$expectedAssets = @(
    "qlog_${plainVersion}_darwin_amd64.tar.gz",
    "qlog_${plainVersion}_darwin_amd64.tar.gz.sbom.json",
    "qlog_${plainVersion}_darwin_arm64.tar.gz",
    "qlog_${plainVersion}_darwin_arm64.tar.gz.sbom.json",
    "qlog_${plainVersion}_linux_amd64.tar.gz",
    "qlog_${plainVersion}_linux_amd64.tar.gz.sbom.json",
    "qlog_${plainVersion}_linux_arm64.tar.gz",
    "qlog_${plainVersion}_linux_arm64.tar.gz.sbom.json",
    "qlog_${plainVersion}_windows_amd64.zip",
    "qlog_${plainVersion}_windows_amd64.zip.sbom.json",
    "qlog_${plainVersion}_windows_arm64.zip",
    "qlog_${plainVersion}_windows_arm64.zip.sbom.json"
)

$temporaryDirectory = $null
try {
    if (-not [string]::IsNullOrWhiteSpace($ReleaseBase)) {
        $uri = $null
        if (-not [Uri]::TryCreate($ReleaseBase, [UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -ne 'https') {
            Fail 'release base must use HTTPS'
        }
        if (-not [string]::IsNullOrEmpty($uri.Query) -or -not [string]::IsNullOrEmpty($uri.Fragment)) {
            Fail 'release base must not contain a query or fragment'
        }
        $temporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("qlog-auth-" + [Guid]::NewGuid().ToString('N'))
        [void](New-Item -ItemType Directory -Path $temporaryDirectory)
        $ArtifactDir = $temporaryDirectory
        $base = $ReleaseBase.TrimEnd('/')
        foreach ($name in @('checksums.txt', 'checksums.txt.sigstore.json') + $expectedAssets) {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "$base/$Version/$name" -OutFile (Join-Path $ArtifactDir $name) -PassThru
            $baseResponse = $response.BaseResponse
            $finalUri = $null
            if ($null -ne $baseResponse.PSObject.Properties['ResponseUri']) {
                $finalUri = $baseResponse.ResponseUri
            } elseif ($null -ne $baseResponse.PSObject.Properties['RequestMessage']) {
                $finalUri = $baseResponse.RequestMessage.RequestUri
            }
            if ($null -eq $finalUri -or $finalUri.Scheme -ne 'https') {
                Fail "download redirected outside HTTPS: $name"
            }
        }
    } elseif (-not (Test-Path -LiteralPath $ArtifactDir -PathType Container)) {
        Fail 'artifact directory does not exist'
    }

    $manifest = Join-Path $ArtifactDir 'checksums.txt'
    $bundle = Join-Path $ArtifactDir 'checksums.txt.sigstore.json'
    foreach ($required in @($manifest, $bundle) + @($expectedAssets | ForEach-Object { Join-Path $ArtifactDir $_ })) {
        if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
            Fail "required artifact is missing: $([System.IO.Path]::GetFileName($required))"
        }
    }

    $identity = "https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/$Version"
    & cosign verify-blob --bundle $bundle --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' --certificate-identity $identity $manifest | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Fail 'checksum signature or workflow identity is invalid'
    }

    $manifestEntries = @(Get-Content -LiteralPath $manifest | ForEach-Object {
        if ($_ -notmatch '^([0-9A-Fa-f]{64})\s{2}([^/\\]+)$') {
            Fail 'checksums.txt does not contain the exact expected asset set'
        }
        [PSCustomObject]@{ Hash = $Matches[1].ToLowerInvariant(); Name = $Matches[2] }
    })
    if ($manifestEntries.Count -ne $expectedAssets.Count) {
        Fail 'checksums.txt does not contain the exact expected asset set'
    }
    foreach ($name in $expectedAssets) {
        $matches = @($manifestEntries | Where-Object { $_.Name -ceq $name })
        if ($matches.Count -ne 1) {
            Fail "checksums.txt must contain exactly one checksum entry for $name"
        }
        $actual = Get-Sha256 (Join-Path $ArtifactDir $name)
        if ($actual -cne $matches[0].Hash) {
            Fail "checksum mismatch for $name"
        }
    }
    Write-Output "PASS authenticity: $Version (all 12 archives and SBOMs)"
} finally {
    if ($null -ne $temporaryDirectory -and (Test-Path -LiteralPath $temporaryDirectory)) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
