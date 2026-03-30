param(
  [Parameter(Mandatory = $true)]
  [string]$Server,
  [string]$Prefix = "$HOME\AppData\Local\serveray-mcp\bin"
)

if (-not (Test-Path ".\cmd\$Server")) {
  Write-Error "Unknown server: $Server"
  exit 1
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Error "Go is required on PATH for source installation."
  exit 1
}

New-Item -ItemType Directory -Force -Path $Prefix | Out-Null
go build -trimpath -o "$Prefix\$Server.exe" ".\cmd\$Server"
Write-Host "Installed $Server to $Prefix\$Server.exe"
