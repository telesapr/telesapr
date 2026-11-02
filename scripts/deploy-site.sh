#!/usr/bin/env bash
# deploy-site.sh -- Copies the web site in site/, with the user's guide
# make guide-site built into site/guide, to the TeleSAPR project's web
# space on SourceForge (https://telesapr.sourceforge.io), over rsync and
# SSH:
#
#     SF_USER=<sourceforge user> ./scripts/deploy-site.sh [--dry-run]
#
# The browser version, which make wasm builds into bin/web (WEB_BUILD to
# take it from elsewhere), is copied into site/app first; site/app is not
# tracked. The landing page links to it through site/try.html.
#
# The SSH key must be registered with that SourceForge account. What is
# on the server and not in site/ is deleted, so htdocs holds the site and
# nothing else; --dry-run lists what would be copied and deleted, and
# changes nothing.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SITE="$REPO_ROOT/site"
WEB_BUILD="${WEB_BUILD:-$REPO_ROOT/bin/web}"

PROJECT="${SF_PROJECT:-telesapr}"
SF_USER="${SF_USER:-}"
DEST="web.sourceforge.net:/home/project-web/$PROJECT/htdocs/"

dry_run=()
case "${1:-}" in
  "") ;;
  -n|--dry-run) dry_run=(--dry-run) ;;
  *)
    echo "usage: SF_USER=<user> $0 [--dry-run]" >&2
    exit 2
    ;;
esac

if [ -z "$SF_USER" ]; then
  echo "error: set SF_USER to your SourceForge user name" >&2
  exit 2
fi
if [ ! -f "$SITE/index.html" ]; then
  echo "error: $SITE/index.html not found" >&2
  exit 2
fi
# The landing page links to the guide, which make guide-site builds.
if [ ! -f "$SITE/guide/index.html" ]; then
  echo "error: $SITE/guide/index.html not found: run make guide-site, or make deploy-site" >&2
  exit 2
fi

# site/try.html links to the browser version in site/app.
web_files=(index.html memfs.js wasm_exec.js telesapr-web.wasm)
for f in "${web_files[@]}"; do
  if [ ! -f "$WEB_BUILD/$f" ]; then
    echo "error: $WEB_BUILD/$f not found: run make wasm" >&2
    exit 2
  fi
done
echo "==> $WEB_BUILD to site/app"
rm -rf "$SITE/app"
mkdir -p "$SITE/app"
for f in "${web_files[@]}"; do
  cp "$WEB_BUILD/$f" "$SITE/app/"
done

echo "==> site/ to $SF_USER@$DEST${dry_run:+ (dry run)}"
rsync -rlvz --delete --chmod=D755,F644 "${dry_run[@]}" \
  -e ssh "$SITE/" "$SF_USER@$DEST"

if [ ${#dry_run[@]} -eq 0 ]; then
  echo "==> https://$PROJECT.sourceforge.io/"
fi
