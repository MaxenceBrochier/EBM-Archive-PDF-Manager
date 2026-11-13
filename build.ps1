# Build des deux exécutables Windows (binaires statiques, sans CGO).
# Prérequis : Go >= 1.24.  Premier build : go mod tidy
$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "0"; $env:GOOS = "windows"; $env:GOARCH = "amd64"
go mod tidy
go vet ./...
go test ./...
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags "-s -w" -o dist\ ./cmd/indexer ./cmd/finder
Copy-Item fields.json dist\ -Force
Write-Host "OK -> dist\indexer.exe, dist\finder.exe, dist\fields.json"
