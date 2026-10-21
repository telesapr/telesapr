#!/usr/bin/env bash
# dist-macos.sh -- The macOS package of a release: TeleSAPR.app, a
# universal binary (Apple silicon and Intel) with telesapr-cli beside it
# in Contents/MacOS, in a .dmg, into $OUT.
#
# Environment: VERSION (pkg/version.Version), OUT (where the package
# goes; make dist-macos sets both). Runs on macOS only: the window
# library's Cocoa backend is cgo, and lipo, iconutil, codesign and
# hdiutil are Apple's. Either architecture builds on the other, since
# go passes -arch to clang.
#
# The app is signed ad hoc, which lets it run on Apple silicon; it is
# not notarized, so Gatekeeper asks for a right-click > Open the first
# time (README.md, Download). Signing with a Developer ID and
# notarizing (xcrun notarytool submit --wait, then xcrun stapler) go
# where the codesign line is, once there is a certificate.

set -euo pipefail
cd "$(dirname "$0")/.."

: "${VERSION:?VERSION is not set}"
: "${OUT:?OUT is not set}"
[[ $(uname -s) == Darwin ]] || { echo "dist-macos.sh: macOS only" >&2; exit 1; }

name=telesapr-$VERSION-macos-universal
stage=$OUT/stage-macos
app=$stage/TeleSAPR.app
rm -rf "$stage"
mkdir -p "$OUT" "$app/Contents/MacOS" "$app/Contents/Resources" "$stage/arch"

for cmd in telesapr telesapr-cli; do
    for arch in arm64 amd64; do
        echo "==> build $cmd $arch"
        GOFLAGS="-trimpath -buildvcs=false" CGO_ENABLED=1 GOARCH=$arch \
            go build -ldflags "-s -w" -o "$stage/arch/$cmd-$arch" "./cmd/$cmd"
    done
    lipo -create -output "$app/Contents/MacOS/$cmd" "$stage/arch/$cmd-arm64" "$stage/arch/$cmd-amd64"
done

echo "==> bundle"
sed "s/@VERSION@/$VERSION/g" dist/macos/Info.plist > "$app/Contents/Info.plist"
printf 'APPL????' > "$app/Contents/PkgInfo"
iconutil -c icns -o "$app/Contents/Resources/telesapr.icns" dist/macos/telesapr.iconset
cp LICENSE NOTICE README.md "$stage"
codesign --force --deep --sign - "$app"
codesign --verify --deep --strict "$app"
ln -s /Applications "$stage/Applications"
rm -rf "$stage/arch"

echo "==> $name.dmg"
rm -f "$OUT/$name.dmg"
# hdiutil fails now and then on the hosted runners ("Resource busy" while
# the volume is detached, or a helper that dies), and -quiet hides why, so
# it prints its errors and is tried again.
made=
for attempt in 1 2 3 4 5; do
    rm -f "$OUT/$name.dmg"
    if hdiutil create -volname "TeleSAPR $VERSION" -srcfolder "$stage" -ov -format UDZO "$OUT/$name.dmg" &&
        hdiutil verify -quiet "$OUT/$name.dmg"; then
        made=1
        break
    fi
    echo "hdiutil failed (attempt $attempt of 5)" >&2
    sleep $((attempt * 5))
done
[[ -n $made ]] || { echo "dist-macos.sh: could not make $name.dmg" >&2; exit 1; }

rm -rf "$stage"
ls -l "$OUT"
