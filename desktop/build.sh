#!/bin/sh
# Build the Windows desktop tracker (needs Go 1.24+, nothing else; works on Linux, macOS or Windows/WSL).
#   desktop/build.sh            -> dist/SSLTracker.exe
set -e
cd "$(dirname "$0")"
python3 gen_data.py
go vet . && GOOS=windows go vet .
go test .
mkdir -p ../dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui" -o ../dist/SSLTracker.exe .
echo "built dist/SSLTracker.exe"
