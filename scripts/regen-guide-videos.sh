#!/usr/bin/env bash
# Record doc/guide/videos: the videos the scenarios in
# doc/guide/scenarios record between `video start` and `video end`, on
# the headless platform, through ffmpeg. Watch every video that changed
# before committing, and commit them on their own. They are not
# goldens: ffmpeg's versions write different bytes for the same frames.
#
# Usage:
#     ./scripts/regen-guide-videos.sh [scenario names...]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

command -v ffmpeg >/dev/null || { echo "error: ffmpeg not found" >&2; exit 1; }
if [ $# -eq 0 ]; then
    # Every scenario that records one.
    mapfile -t names < <(grep -l '^video start ' doc/guide/scenarios/*.scenario | xargs -n1 basename | sed 's/\.scenario$//')
    set -- "${names[@]}"
fi
run="^TestDocScenarios\$/^($(IFS='|'; echo "$*"))\$"
mkdir -p doc/guide/videos
TELESAPR_DOC_VIDEOS="$REPO_ROOT/doc/guide/videos" go test ./pkg/ui -run "$run" -count=1
ls -l doc/guide/videos
git status --short doc/guide/videos
