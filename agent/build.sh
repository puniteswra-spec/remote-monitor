#!/bin/bash
# Build SystemHelper.exe with version
# Usage: ./build.sh [version]

VERSION="${1:-1.0.0}"

cd "$(dirname "$0")"

# Update version in source
sed -i '' "s/const Version = \".*\"/const Version = \"$VERSION\"/" main.go

# Build
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o "SystemHelper_v${VERSION}.exe" main.go

echo "✅ Built: SystemHelper_v${VERSION}.exe ($(ls -lh "SystemHelper_v${VERSION}.exe" | awk '{print $5}'))"