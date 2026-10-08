#!/usr/bin/env bash
# release.sh -- Release the version pkg/version/version.go names.
#
# Usage:
#   release.sh            # the release
#   release.sh --notes    # only print what the release would say
#
# The release of version X.Y.Z is the tag vX.Y.Z on master and a GitHub
# release of that tag. In order:
#   1. master is checked out, clean and not behind origin, and the tag
#      is new here and on origin;
#   2. master is pushed;
#   3. the CI workflow run for HEAD is waited for (SKIP_CI=1 skips
#      this, for when CI is known to be broken for a reason that is
#      not the code);
#   4. the notes are made: the commits since the previous tag, the
#      compare link and the names of the packages (docs/releasing.md);
#   5. the annotated tag carries the notes and is pushed;
#   6. the GitHub release is created with the same notes;
#   7. the Release workflow, which builds and attaches the packages, is
#      watched until it is done.
# Stopping after step 6 loses nothing: the workflow runs on its own.
#
# A pushed tag is permanent. A bad release is fixed by the next version,
# never by moving the tag.

set -euo pipefail
cd "$(dirname "$0")/.."

die() { echo "release: $*" >&2; exit 1; }

version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' pkg/version/version.go)
repo=$(sed -n 's/^const Repository = "\(.*\)"$/\1/p' pkg/version/version.go)
[[ -n $version && -n $repo ]] || die "cannot read Version and Repository from pkg/version/version.go"
[[ $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$ ]] ||
    die "version $version is not X.Y.Z"
tag=v$version

# notes writes the release notes for the tag to stdout: the commits
# since the previous tag, newest last, the compare link, and the
# packages the Release workflow attaches.
notes() {
    local prev
    prev=$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || true)
    if [[ -n $prev ]]; then
        echo "## Changes since $prev"
        echo
        git log --no-merges --reverse --format='- %s' "$prev..HEAD"
        echo
        echo "Full diff: $repo/compare/$prev...$tag"
    else
        echo "The first release."
    fi
    cat <<EOF

## Downloads

| | |
|---|---|
| Linux x86_64 | \`telesapr-$version-linux-amd64.tar.gz\`, \`telesapr_${version}_amd64.deb\`, \`telesapr-$version-1.x86_64.rpm\` |
| Windows x64 | \`telesapr-$version-windows-amd64-setup.exe\`, or \`telesapr-$version-windows-amd64.zip\` |
| Windows arm64 | \`telesapr-$version-windows-arm64-setup.exe\`, or \`telesapr-$version-windows-arm64.zip\` |
| macOS (Apple silicon and Intel) | \`telesapr-$version-macos-universal.dmg\` |

The Release workflow attaches them a few minutes after this page appears, with their checksums
in \`SHA256SUMS\`. How to install each is in the [README]($repo/blob/$tag/README.md#download).
The Windows and macOS builds are compiled by the workflow and not tested by it.
EOF
}

if [[ ${1:-} == --notes ]]; then
    notes
    exit 0
fi
[[ $# -eq 0 ]] || die "usage: release.sh [--notes]"

# 1. Preconditions.
command -v gh >/dev/null || die "gh is not installed"
gh auth status >/dev/null 2>&1 || die "gh is not logged in"
[[ $(git rev-parse --abbrev-ref HEAD) == master ]] || die "not on master"
[[ -z $(git status --porcelain --untracked-files=no) ]] || die "working tree has uncommitted changes"
git fetch -q origin master --tags
[[ -z $(git rev-list HEAD..origin/master) ]] || die "master is behind origin/master"
git rev-parse -q --verify "refs/tags/$tag" >/dev/null && die "tag $tag exists; bump pkg/version/version.go first (docs/releasing.md)"
[[ -z $(git ls-remote --tags origin "refs/tags/$tag") ]] || die "tag $tag exists on origin"

sha=$(git rev-parse HEAD)
echo "Releasing $tag at ${sha:0:7}."

# 2. The commits.
git push origin master

# 3. CI on the commit.
if [[ ${SKIP_CI:-} != 1 ]]; then
    run=
    for _ in $(seq 1 24); do
        run=$(gh run list -w CI -c "$sha" --json databaseId -q '.[0].databaseId')
        [[ -n $run ]] && break
        sleep 5
    done
    [[ -n $run ]] || die "no CI run for ${sha:0:7} after two minutes"
    echo "Waiting for CI run $run."
    gh run watch "$run" --exit-status >/dev/null || die "CI failed for ${sha:0:7}; see gh run view $run"
    echo "CI passed."
fi

# 4. The notes.
file=$(mktemp)
trap 'rm -f "$file"' EXIT
notes >"$file"
echo
cat "$file"
echo

# 5. The tag.
{ echo "$tag"; echo; cat "$file"; } | git tag -a --cleanup=whitespace "$tag" -F -
git push origin "$tag"
echo "Tagged $tag."

# 6. The release.
url=$(gh release create "$tag" --verify-tag --title "TeleSAPR $version" --notes-file "$file")
echo "Release: $url"

# 7. The packages.
run=
for _ in $(seq 1 24); do
    run=$(gh run list -w Release -b "$tag" --json databaseId -q '.[0].databaseId')
    [[ -n $run ]] && break
    sleep 5
done
[[ -n $run ]] || die "no Release run for $tag after two minutes; see gh run list -w Release"
echo "Waiting for Release run $run to attach the packages (Ctrl-C leaves it running)."
gh run watch "$run" --exit-status >/dev/null || die "the Release run failed; see gh run view $run, and docs/releasing.md"
echo "Done: $url"
