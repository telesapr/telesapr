#!/usr/bin/env bash
# gen-icons.sh -- The Windows and macOS icons from dist/freedesktop/telesapr.svg.
#
# Writes dist/windows/telesapr.ico (the installer's icon),
# dist/windows/telesapr-256.png (what go-winres makes the program's
# icon from: it reads no ImageMagick .ico) and the PNGs of
# dist/macos/telesapr.iconset, which iconutil turns into the .icns on
# the macOS runner (ImageMagick on Linux writes no ICNS). Needs
# rsvg-convert (librsvg) and magick (ImageMagick 7). Run when the SVG
# changes, and commit what it writes.

set -euo pipefail
cd "$(dirname "$0")/.."

svg=dist/freedesktop/telesapr.svg
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

render() { rsvg-convert -w "$1" -h "$1" "$svg" -o "$2"; }

echo "==> dist/windows/telesapr.ico"
for s in 16 24 32 48 64 128 256; do render $s "$tmp/$s.png"; done
magick "$tmp"/{16,24,32,48,64,128,256}.png dist/windows/telesapr.ico
cp "$tmp/256.png" dist/windows/telesapr-256.png

echo "==> dist/macos/telesapr.iconset"
set=dist/macos/telesapr.iconset
rm -rf "$set"
mkdir -p "$set"
for s in 16 32 128 256 512; do
    render $s "$set/icon_${s}x${s}.png"
    render $((s * 2)) "$set/icon_${s}x${s}@2x.png"
done
ls "$set"
