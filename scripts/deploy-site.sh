#!/usr/bin/env bash
# deploy-site.sh -- Copies the web site in site/ to the TeleSAPR project's
# web space on SourceForge (https://telesapr.sourceforge.io), over rsync
# and SSH:
#
#     SF_USER=<sourceforge user> ./scripts/deploy-site.sh [--dry-run]
#
# The SSH key must be registered with that SourceForge account. What is
# on the server and not in site/ is deleted, so htdocs holds the site and
# nothing else; --dry-run lists what would be copied and deleted, and
# changes nothing.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SITE="$REPO_ROOT/site"

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

echo "==> site/ to $SF_USER@$DEST${dry_run:+ (dry run)}"
rsync -rlvz --delete --chmod=D755,F644 "${dry_run[@]}" \
  -e ssh "$SITE/" "$SF_USER@$DEST"

if [ ${#dry_run[@]} -eq 0 ]; then
  echo "==> https://$PROJECT.sourceforge.io/"
fi
