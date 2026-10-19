#!/bin/sh
# Build the Linux architecture matrix used by official AdGuard Home releases.
# Usage: GO=/path/to/go sh scripts/build-linux.sh [arm64 armv7 ...]
set -eu
cd "$(dirname "$0")/.."
GO="${GO:-go}"
if [ "$#" -eq 0 ]; then
    set -- amd64 386 arm64 armv5 armv6 armv7 mips_softfloat mipsle_softfloat mips64_softfloat mips64le_softfloat ppc64le riscv64
fi
for agh_target do
    case "$agh_target" in
        amd64|386|arm64|armv5|armv6|armv7|mips_softfloat|mipsle_softfloat|mips64_softfloat|mips64le_softfloat|ppc64le|riscv64) ;;
        *) echo "Unsupported Linux target: $agh_target" >&2; exit 2 ;;
    esac
done
export GOWORK="$(pwd)/go.work"
if [ "${SKIP_FRONTEND:-0}" = 1 ]; then
    test -f AdGuardHome/build/static/index.html || {
        echo 'Build the frontend before using SKIP_FRONTEND=1.' >&2; exit 1;
    }
else
    if [ ! -d AdGuardHome/client/node_modules ]; then
        (cd AdGuardHome/client && npm ci --no-audit --no-fund)
    fi
    (cd AdGuardHome/client && npm run build-prod)
fi
mkdir -p artifacts
agh_version=$(cat VERSION)
for agh_target do
    (
        unset GOARM GOMIPS GOMIPS64
        export CGO_ENABLED=0 GOOS=linux
        agh_version_pkg=github.com/AdguardTeam/AdGuardHome/internal/version
        agh_flags="-s -w -X ${agh_version_pkg}.version=${agh_version} -X ${agh_version_pkg}.channel=development"
        case "$agh_target" in
            armv[567])
                export GOARCH=arm GOARM="${agh_target#armv}"
                agh_flags="$agh_flags -X ${agh_version_pkg}.goarm=${GOARM}"
                ;;
            mips_softfloat|mipsle_softfloat)
                export GOARCH="${agh_target%_softfloat}" GOMIPS=softfloat
                agh_flags="$agh_flags -X ${agh_version_pkg}.gomips=softfloat"
                ;;
            mips64_softfloat|mips64le_softfloat)
                export GOARCH="${agh_target%_softfloat}" GOMIPS64=softfloat
                agh_flags="$agh_flags -X ${agh_version_pkg}.gomips=softfloat"
                ;;
            *) export GOARCH="$agh_target" ;;
        esac
        "$GO" build -trimpath -buildvcs=false -ldflags "$agh_flags" \
            -o "artifacts/AdGuardHome-linux-$agh_target" ./AdGuardHome
    )
done
(
    cd artifacts
    for agh_target do
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum "AdGuardHome-linux-$agh_target"
        else
            shasum -a 256 "AdGuardHome-linux-$agh_target"
        fi
    done
) > artifacts/SHA256SUMS-linux
