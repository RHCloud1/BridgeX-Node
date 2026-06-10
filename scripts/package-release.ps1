param(
  [string]$Version = "dev"
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$out = Join-Path $root "dist"
$go = "go"
if (Test-Path "D:\Project\Tools\go\bin\go.exe") {
  $go = "D:\Project\Tools\go\bin\go.exe"
}

New-Item -ItemType Directory -Force -Path $out | Out-Null

$targets = @(
  @{ GOOS = "linux"; GOARCH = "amd64"; Name = "nodebridge-linux-amd64" },
  @{ GOOS = "linux"; GOARCH = "arm64"; Name = "nodebridge-linux-arm64" },
  @{ GOOS = "windows"; GOARCH = "amd64"; Name = "nodebridge-windows-amd64" }
)

foreach ($target in $targets) {
  $stage = Join-Path $out $target.Name
  if (Test-Path $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
  New-Item -ItemType Directory -Force -Path $stage, (Join-Path $stage "configs"), (Join-Path $stage "deploy") | Out-Null

  $env:GOOS = $target.GOOS
  $env:GOARCH = $target.GOARCH
  $binary = "nodebridged"
  if ($target.GOOS -eq "windows") { $binary = "nodebridged.exe" }
  & $go build -ldflags "-X main.version=$Version" -o (Join-Path $stage $binary) ./cmd/nodebridged

  Copy-Item -LiteralPath (Join-Path $root "configs\nodebridge.example.json") -Destination (Join-Path $stage "configs\nodebridge.example.json")
  Copy-Item -LiteralPath (Join-Path $root "deploy\nodebridge.sh") -Destination (Join-Path $stage "deploy\nodebridge.sh")
  Copy-Item -LiteralPath (Join-Path $root "deploy\install.sh") -Destination (Join-Path $stage "deploy\install.sh")
  Copy-Item -LiteralPath (Join-Path $root "README.md") -Destination (Join-Path $stage "README.md")

  $archive = Join-Path $out ($target.Name + ".tar.gz")
  if (Test-Path $archive) { Remove-Item -LiteralPath $archive -Force }
  tar -czf $archive -C $stage .
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

