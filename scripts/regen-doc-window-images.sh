#!/usr/bin/env bash
# Regenerate the guide's pictures of the real window, the menus, popup
# menus and dialogs the headless scenarios can't draw
# (TestDocWindowShots in pkg/ui/docwindow_test.go). They are read back
# from an Xvfb of this script's own, so that the screen is the same on
# every run. The window draws its text through Xft, so the fonts are
# fontconfig's: the script gives the session a fontconfig of its own that
# sees only the Liberation fonts (the Liberation package: fonts-liberation
# on Debian, ttf-liberation on Arch, liberation-fonts on Fedora; Sans,
# Serif and Mono) and maps sans-serif, serif and monospace to them, so two
# machines with the same Liberation version give the same pictures. Look
# at every picture that changed before committing, and commit them on
# their own.
#
# Usage:
#     ./scripts/regen-doc-window-images.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

command -v Xvfb >/dev/null || { echo "Xvfb is needed" >&2; exit 2; }
command -v fc-list >/dev/null || { echo "fc-list (fontconfig) is needed" >&2; exit 2; }

# The fonts: copies of the Liberation files in a directory of their own,
# the only one the fontconfig below reads.
work="$(mktemp -d)"
xvfb=""
cleanup() {
    [ -z "$xvfb" ] || kill "$xvfb" 2>/dev/null || true
    rm -rf "$work"
}
trap cleanup EXIT
mkdir -p "$work/fonts" "$work/cache"
fc-list : file family | grep -E ': Liberation (Sans|Serif|Mono)(:|,|$)' | sed 's/: .*//' | sort -u |
    while read -r f; do cp "$f" "$work/fonts/"; done
if [ -z "$(ls "$work/fonts")" ]; then
    echo "the Liberation fonts (fonts-liberation, ttf-liberation or liberation-fonts) are needed" >&2
    exit 2
fi
cat >"$work/fonts.conf" <<EOF
<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
  <dir>$work/fonts</dir>
  <cachedir>$work/cache</cachedir>
  <alias binding="same"><family>sans-serif</family><prefer><family>Liberation Sans</family></prefer></alias>
  <alias binding="same"><family>serif</family><prefer><family>Liberation Serif</family></prefer></alias>
  <alias binding="same"><family>monospace</family><prefer><family>Liberation Mono</family></prefer></alias>
</fontconfig>
EOF
export FONTCONFIG_FILE="$work/fonts.conf"
echo "fonts: $(fc-list : family | sort -u | tr '\n' ';')"

# A display no other server holds, apart from make test's and test-gui's.
display=190
while [ -e "/tmp/.X${display}-lock" ] || [ -e "/tmp/.X11-unix/X${display}" ]; do
    display=$((display + 1))
done
Xvfb ":$display" -screen 0 1280x1024x24 -noreset -nolisten tcp >/dev/null 2>&1 &
xvfb=$!
for _ in $(seq 50); do
    [ -e "/tmp/.X11-unix/X${display}" ] && break
    sleep 0.1
done

mkdir -p doc/guide/images
DISPLAY=":$display" TELESAPR_DOC_WINDOW_SHOTS="$REPO_ROOT/doc/guide/images" \
    go test ./pkg/ui -run '^TestDocWindowShots$' -count=1
git status --short doc/guide/images
