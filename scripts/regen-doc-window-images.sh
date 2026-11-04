#!/usr/bin/env bash
# Regenerate the guide's pictures of the real window, the menus, popup
# menus and dialogs the headless scenarios can't draw
# (TestDocWindowShots in pkg/ui/docwindow_test.go). They are read back
# from an Xvfb of this script's own, so that the screen is the same on
# every run; the fonts are the machine's, so regenerate on the machine
# the pictures were last made on, or all of them change. Look at every
# picture that changed before committing, and commit them on their own.
#
# Usage:
#     ./scripts/regen-doc-window-images.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

command -v Xvfb >/dev/null || { echo "Xvfb is needed" >&2; exit 2; }

# A display no other server holds, apart from make test's and test-gui's.
display=190
while [ -e "/tmp/.X${display}-lock" ] || [ -e "/tmp/.X11-unix/X${display}" ]; do
    display=$((display + 1))
done
Xvfb ":$display" -screen 0 1280x1024x24 -noreset -nolisten tcp >/dev/null 2>&1 &
xvfb=$!
trap 'kill "$xvfb" 2>/dev/null || true' EXIT
for _ in $(seq 50); do
    [ -e "/tmp/.X11-unix/X${display}" ] && break
    sleep 0.1
done

mkdir -p doc/guide/images
DISPLAY=":$display" TELESAPR_DOC_WINDOW_SHOTS="$REPO_ROOT/doc/guide/images" \
    go test ./pkg/ui -run '^TestDocWindowShots$' -count=1
git status --short doc/guide/images
