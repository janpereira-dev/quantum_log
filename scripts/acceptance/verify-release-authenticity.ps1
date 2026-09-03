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

$platform = if ([System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT) { 'windows' } else {
    try {
        if ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::OSX)) { 'darwin' } else { 'linux' }
    } catch { 'linux' }
}
$architecture = try { [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString().ToLowerInvariant() } catch { $env:PROCESSOR_ARCHITECTURE.ToLowerInvariant() }
$arch = switch -Regex ($architecture) {
    '^(x64|amd64)$' { 'amd64'; break }
    '^(arm64|aarch64)$' { 'arm64'; break }
    default { Fail "unsupported architecture: $architecture" }
}
$extension = if ($platform -eq 'windows') { 'zip' } else { 'tar.gz' }
$plainVersion = $Version.Substring(1)
$archive = "qlog_${plainVersion}_${platform}_${arch}.${extension}"
$sbom = "$archive.sbom.json"

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
        foreach ($name in @('checksums.txt', 'checksums.txt.sigstore.json', $archive, $sbom)) {
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
    foreach ($required in @($manifest, $bundle, (Join-Path $ArtifactDir $archive), (Join-Path $ArtifactDir $sbom))) {
        if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
            Fail "required artifact is missing: $([System.IO.Path]::GetFileName($required))"
        }
    }

    $identity = "https://github.com/janpereira-dev/quantum_log/.github/workflows/release.yml@refs/tags/$Version"
    & cosign verify-blob --bundle $bundle --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' --certificate-identity $identity $manifest | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Fail 'checksum signature or workflow identity is invalid'
    }

    foreach ($name in @($archive, $sbom)) {
        $matches = @(Get-Content -LiteralPath $manifest | ForEach-Object {
            if ($_ -match '^([0-9A-Fa-f]{64})\s{2}(.+)$' -and $Matches[2] -ceq $name) { $Matches[1].ToLowerInvariant() }
        })
        if ($matches.Count -ne 1) {
            Fail "checksums.txt must contain exactly one checksum entry for $name"
        }
        $actual = Get-Sha256 (Join-Path $ArtifactDir $name)
        if ($actual -cne $matches[0]) {
            Fail "checksum mismatch for $name"
        }
    }
    Write-Output "PASS authenticity: $Version ($archive and $sbom)"
} finally {
    if ($null -ne $temporaryDirectory -and (Test-Path -LiteralPath $temporaryDirectory)) {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}
