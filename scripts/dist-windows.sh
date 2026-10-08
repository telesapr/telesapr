#!/usr/bin/env bash
# dist-windows.sh -- The Windows packages of a release: a zip of the two
# programs for x64 and for arm64, and an installer for each where Inno
# Setup is installed, into $OUT.
#
# Environment: VERSION (pkg/version.Version), OUT (where the packages
# go; make dist-windows sets both). The Windows build is pure Go, so the
# zips are cross-compiled from any OS; the installers need ISCC, which
# the Release workflow has on its Windows runner. On other systems the
# installers are skipped with a message.
#
# The icon, version information and manifest (DPI awareness, long
# paths) come from dist/windows/winres.json through go-winres, which
# writes the .syso files go build links in; they are not tracked.
# telesapr.exe is a windows subsystem program, so it opens no console:
# what it logs to stderr is lost, and telesapr-cli is the console tool.

set -euo pipefail
cd "$(dirname "$0")/.."

: "${VERSION:?VERSION is not set}"
: "${OUT:?OUT is not set}"
WINRES=github.com/tc-hib/go-winres@v0.3.3

mkdir -p "$OUT"
root=$PWD

echo "==> resources"
rm -f cmd/telesapr/rsrc_windows_*.syso
go run "$WINRES" make --in dist/windows/winres.json --arch amd64,arm64 \
    --file-version "$VERSION" --product-version "$VERSION" --out cmd/telesapr/rsrc

iscc=
if command -v ISCC >/dev/null; then
    iscc=ISCC
elif [[ -x "/c/Program Files (x86)/Inno Setup 6/ISCC.exe" ]]; then
    iscc="/c/Program Files (x86)/Inno Setup 6/ISCC.exe"
fi
# ISCC takes Windows paths; Git Bash would otherwise rewrite its /D
# options as paths.
winpath() { if command -v cygpath >/dev/null; then cygpath -w "$1"; else echo "$1"; fi; }

for arch in amd64 arm64; do
    name=telesapr-$VERSION-windows-$arch
    stage=$OUT/stage-$name
    rm -rf "$stage"
    mkdir -p "$stage"

    echo "==> build $arch"
    GOFLAGS="-trimpath -buildvcs=false" CGO_ENABLED=0 GOOS=windows GOARCH=$arch \
        go build -ldflags "-s -w -H windowsgui" -o "$stage/telesapr.exe" ./cmd/telesapr
    GOFLAGS="-trimpath -buildvcs=false" CGO_ENABLED=0 GOOS=windows GOARCH=$arch \
        go build -ldflags "-s -w" -o "$stage/telesapr-cli.exe" ./cmd/telesapr-cli
    cp LICENSE NOTICE README.md "$stage"

    echo "==> $name.zip"
    rm -f "$OUT/$name.zip"
    (cd "$OUT" && mv "stage-$name" "$name" && zip -qr "$name.zip" "$name" && mv "$name" "stage-$name")

    if [[ -n $iscc ]]; then
        echo "==> $name-setup.exe"
        MSYS_NO_PATHCONV=1 "$iscc" /Q "/DVERSION=$VERSION" "/DARCH=$arch" \
            "/DSRC=$(winpath "$stage")" "/DROOT=$(winpath "$root")" "/DOUT=$(winpath "$OUT")" \
            "$(winpath dist/windows/telesapr.iss)"
    else
        echo "==> $name-setup.exe skipped: Inno Setup (ISCC) is not installed"
    fi
    rm -rf "$stage"
done

rm -f cmd/telesapr/rsrc_windows_*.syso
ls -l "$OUT"
