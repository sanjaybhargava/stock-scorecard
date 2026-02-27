#!/usr/bin/env bash
set -euo pipefail

DIST="./dist"
PKG="./cmd/scorecard"

rm -rf "$DIST"
mkdir -p "$DIST"

echo "Building Mac Apple Silicon (darwin/arm64)..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o "$DIST/stock-scorecard-mac-m" "$PKG"

echo "Building Mac Intel (darwin/amd64)..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o "$DIST/stock-scorecard-mac-intel" "$PKG"

echo "Building Windows (windows/amd64)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o "$DIST/stock-scorecard-windows.exe" "$PKG"

echo ""
echo "Done. Binaries in $DIST/:"
ls -lh "$DIST/"
