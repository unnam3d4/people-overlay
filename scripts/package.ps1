# Pack complete, OFFLINE Windows EXEs on the GitHub runner.
# Fail closed: neither a model nor a real inference can be skipped.
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$project = Split-Path -Parent $PSScriptRoot
$cache = Join-Path $project '_download'
$out = Join-Path $project 'RELEASE'
New-Item -ItemType Directory -Path $cache,$out -Force | Out-Null

function Fetch([string]$url,[string]$dest,[long]$minimum) {
  if ((Test-Path -LiteralPath $dest) -and (Get-Item -LiteralPath $dest).Length -ge $minimum) { return }
  $temp = "$dest.part"
  for ($i=1; $i -le 3; $i++) {
    try {
      Write-Host "Downloading $url (attempt $i)"
      Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $temp -MaximumRedirection 12 -TimeoutSec 900
      $size = (Get-Item -LiteralPath $temp).Length
      if ($size -lt $minimum) { throw "Download unexpectedly small: $size" }
      Move-Item -LiteralPath $temp -Destination $dest -Force
      return
    } catch {
      Write-Warning $_
      Remove-Item -LiteralPath $temp -Force -ErrorAction SilentlyContinue
      if ($i -eq 3) { throw }
      Start-Sleep -Seconds ($i*3)
    }
  }
}
$model = Join-Path $cache 'model.onnx'
Fetch 'https://huggingface.co/inference4j/yolov8n/resolve/main/model.onnx?download=true' $model 5000000
& python (Join-Path $project 'scripts/normalize_onnx.py') $model
if ($LASTEXITCODE -ne 0) { throw 'ONNX normalization failed' }
foreach ($arch in @('x64','x86')) {
  $archive = Join-Path $cache "onnxruntime-win-$arch-1.11.1.zip"
  $url = "https://github.com/microsoft/onnxruntime/releases/download/v1.11.1/onnxruntime-win-$arch-1.11.1.zip"
  Fetch $url $archive 10000000
  $unpacked = Join-Path $cache "runtime-$arch"
  Expand-Archive -LiteralPath $archive -DestinationPath $unpacked -Force
}
$sourceDir = Join-Path $project 'src'
function Package([string]$arch,[string]$goarch) {
  $runtime = Join-Path (Join-Path $cache "runtime-$arch") "onnxruntime-win-$arch-1.11.1\lib\onnxruntime.dll"
  if (!(Test-Path -LiteralPath $runtime)) { throw "NuGet missing native runtime for $arch" }
  $payload = Join-Path $cache "payload-$arch"
  New-Item -ItemType Directory -Path $payload -Force | Out-Null
  Copy-Item -LiteralPath $runtime -Destination (Join-Path $payload 'onnxruntime.dll') -Force
  Copy-Item -LiteralPath $model -Destination (Join-Path $payload 'model.onnx') -Force
  $payloadZip = Join-Path $cache "payload-$arch.zip"
  if (Test-Path $payloadZip) { Remove-Item $payloadZip -Force }
  Compress-Archive -LiteralPath (Join-Path $payload 'model.onnx'),(Join-Path $payload 'onnxruntime.dll') -DestinationPath $payloadZip -CompressionLevel Optimal
  $exe = Join-Path $out "PeopleOverlay_Win7to11_$arch.exe"
  $env:GOOS = 'windows'
  $env:GOARCH = $goarch
  $env:CGO_ENABLED = '0'
  Push-Location $sourceDir
  try { & go build -trimpath -ldflags '-s -w -H windowsgui' -o $exe .
        if ($LASTEXITCODE -ne 0) { throw "Go build failed for $arch" }
  } finally { Pop-Location }
  $f = [IO.File]::Open($exe,[IO.FileMode]::Append,[IO.FileAccess]::Write)
  $z = [IO.File]::OpenRead($payloadZip)
  try {
    $z.CopyTo($f)
    $magic = [Text.Encoding]::ASCII.GetBytes('PPLAI06!')
    $f.Write($magic,0,$magic.Length)
    $n = [BitConverter]::GetBytes([uint64]$z.Length)
    if (![BitConverter]::IsLittleEndian) { [Array]::Reverse($n) }
    $f.Write($n,0,$n.Length)
    $f.Flush()
  } finally { $z.Dispose(); $f.Dispose() }
  Write-Host "AI smoke-test with $arch bundled EXE ..."
  $p = Start-Process -FilePath $exe -ArgumentList '--selftest' -Wait -PassThru
  if ($p.ExitCode -ne 0) {
    $log = Join-Path $env:LOCALAPPDATA 'PeopleOverlay\PeopleOverlay.log'
    if (Test-Path $log) { Get-Content $log -Tail 35 | Write-Host }
    throw "Real model load + inference FAILED for $arch with code $($p.ExitCode); artifact NOT released"
  }
  Write-Host "PASS: $arch native EXE, embedded model and ONNX Runtime inference" -ForegroundColor Green
  Get-FileHash -Algorithm SHA256 $exe | ForEach-Object { Write-Host "$arch SHA256 $($_.Hash)" }
}
Package 'x64' 'amd64'
Package 'x86' '386'
Copy-Item (Join-Path $project 'README.md') -Destination $out
Copy-Item (Join-Path $project 'THIRD_PARTY_NOTICES.txt') -Destination $out
# Required to accompany redistributions of AGPL-3.0 licensed model.
$license = Join-Path $out 'LICENSE-AGPL-3.0.txt'
Copy-Item -LiteralPath (Join-Path $project 'LICENSE') -Destination $license -Force
Write-Host "Both offline builds passed the real AI self-test." -ForegroundColor Green
