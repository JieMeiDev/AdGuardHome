#!/bin/sh
set -eu
cd "$(dirname "$0")"
GO="${GO:-go}"
export GOWORK="$(pwd)/go.work"
if [ ! -d AdGuardHome/client/node_modules ]; then
    (cd AdGuardHome/client && npm ci --no-audit --no-fund)
fi
(cd AdGuardHome/client && npm run build-prod)
mkdir -p artifacts
version_pkg=github.com/AdguardTeam/AdGuardHome/internal/version
runtime_version="v0.107.79-ech.2"
build_flags="-s -w -X ${version_pkg}.version=${runtime_version} -X ${version_pkg}.channel=development"
for platform in darwin linux; do
    CGO_ENABLED=0 GOOS="$platform" GOARCH=amd64 "$GO" build -trimpath -ldflags "$build_flags" -o "artifacts/AdGuardHome-$platform-amd64" ./AdGuardHome
done
(cd artifacts && shasum -a 256 AdGuardHome-darwin-amd64 AdGuardHome-linux-amd64 > SHA256SUMS)
