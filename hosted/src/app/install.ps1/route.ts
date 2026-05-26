const script = `param(
  [Parameter(Mandatory = $true)]
  [string]$Api,

  [Parameter(Mandatory = $true)]
  [string]$Token
)

$ErrorActionPreference = "Stop"

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
if ($arch -eq "x64") {
  $arch = "amd64"
} elseif ($arch -eq "arm64") {
  $arch = "arm64"
} else {
  throw "Unsupported CPU architecture: $arch"
}

$installDir = Join-Path $env:LOCALAPPDATA "Vibeengine\\bin"
$enginePath = Join-Path $installDir "vibeengine.exe"
$artifact = "vibeengine-windows-$arch.exe"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

try {
  $tmp = "$enginePath.tmp"
  $sig = "$tmp.sig"
  $checksum = "$tmp.sha256"
  $pub = "$tmp.pub"
  Invoke-WebRequest -Uri "$Api/releases/$artifact" -OutFile "$tmp"
  Invoke-WebRequest -Uri "$Api/releases/$artifact.sig" -OutFile "$sig"
  Invoke-WebRequest -Uri "$Api/releases/$artifact.sha256" -OutFile "$checksum"
  Invoke-WebRequest -Uri "$Api/releases/release-signing-public.pem" -OutFile "$pub"

  $expected = ((Get-Content $checksum -Raw).Trim() -split "\\s+")[0].ToLowerInvariant()
  $actual = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLowerInvariant()
  if ($expected -ne $actual) {
    throw "Vibeengine release checksum verification failed."
  }

  $rsa = [System.Security.Cryptography.RSA]::Create()
  $rsa.ImportFromPem((Get-Content $pub -Raw))
  $body = [System.IO.File]::ReadAllBytes($tmp)
  $signature = [System.IO.File]::ReadAllBytes($sig)
  $ok = $rsa.VerifyData(
    $body,
    $signature,
    [System.Security.Cryptography.HashAlgorithmName]::SHA256,
    [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
  )
  if (-not $ok) {
    throw "Vibeengine release signature verification failed."
  }
  Move-Item -Force $tmp $enginePath
  Remove-Item -Force $sig,$checksum,$pub -ErrorAction SilentlyContinue
} catch {
  Remove-Item -Force "$enginePath.tmp","$enginePath.tmp.sig","$enginePath.tmp.sha256","$enginePath.tmp.pub" -ErrorAction SilentlyContinue
  $existing = Get-Command vibeengine -ErrorAction SilentlyContinue
  if ($existing) {
    $enginePath = $existing.Source
  } else {
    throw "No verified Vibeengine release artifact is available for windows/$arch yet."
  }
}

& $enginePath bootstrap --api $Api --token $Token --repo (Get-Location).Path --adapter codex --adapter claude-code
& $enginePath status

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not ($userPath -split ";" | Where-Object { $_ -eq $installDir })) {
  [Environment]::SetEnvironmentVariable("Path", ($userPath.TrimEnd(";") + ";" + $installDir), "User")
  Write-Host "Added $installDir to your user PATH. Open a new terminal to use vibeengine anywhere."
}
`;

export function GET() {
  return new Response(script, {
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
