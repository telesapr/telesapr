#!/usr/bin/env bash
# dist-linux.sh -- The Linux packages of a release: a tarball, a .deb
# and an .rpm of what make install puts in place, into $OUT.
#
# Environment: VERSION (pkg/version.Version), OUT (where the packages
# go; make dist-linux sets both). Needs the X11 and OpenGL development
# packages (README.md, Build). The .deb and .rpm are made by nfpm, run
# at a pinned version through go run, which leaves go.mod alone.
#
# The binaries are built with -trimpath and without symbols, and with
# -buildvcs=false: the checkout in the workflow's container belongs to
# another user, which git refuses to read, and the version is in the
# code anyway.

set -euo pipefail
cd "$(dirname "$0")/.."

: "${VERSION:?VERSION is not set}"
: "${OUT:?OUT is not set}"
NFPM=github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0

arch=amd64
name=telesapr-$VERSION-linux-$arch
# Where nfpm.yaml takes the files from: it expands no variables there.
stage=bin/stage-linux
rm -rf "$stage"
mkdir -p "$OUT" "$stage"

# What make install puts in place, under /usr, built for the release;
# the desktop entry's Exec then names /usr/bin/telesapr.
echo "==> build and stage"
GOFLAGS="-trimpath -buildvcs=false" CGO_ENABLED=1 GOARCH=$arch \
    make --no-print-directory install DESTDIR="$stage" PREFIX=/usr LDFLAGS="-s -w"
install -Dm644 LICENSE "$stage/usr/share/doc/telesapr/LICENSE"
install -Dm644 NOTICE "$stage/usr/share/doc/telesapr/NOTICE"
install -Dm644 README.md "$stage/usr/share/doc/telesapr/README.md"

echo "==> $name.tar.gz"
tar -C "$stage" --transform "s,^usr,$name," --owner=0 --group=0 --numeric-owner \
    -czf "$OUT/$name.tar.gz" usr

echo "==> .deb and .rpm"
export VERSION
go run "$NFPM" package -f dist/linux/nfpm.yaml -p deb -t "$OUT/telesapr_${VERSION}_$arch.deb"
go run "$NFPM" package -f dist/linux/nfpm.yaml -p rpm -t "$OUT/telesapr-$VERSION-1.x86_64.rpm"

rm -rf "$stage"
ls -l "$OUT"
