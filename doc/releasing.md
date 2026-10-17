# Releasing

A release of TeleSAPR is a version `X.Y.Z` in `pkg/version/version.go`, the tag `vX.Y.Z` on
`master`, and a GitHub release of that tag with the packages the Release workflow builds. The
version is bumped by hand, in a commit of its own; the rest is `make release`.

## The bump

1. Set `Version` in `pkg/version/version.go`.
2. Commit the bump as "Version X.Y.Z". No golden holds the version.

## The release

```
make release-notes    # what the release will say, from the commits since the previous tag
make release
```

`make release` (`scripts/release.sh`), in order:

1. Checks: on `master`, nothing uncommitted, not behind `origin/master`, `gh` logged in, and the
   tag does not exist here or on origin.
2. Pushes `master`.
3. Waits for the CI workflow on that commit, and stops if it fails (`SKIP_CI=1` skips the wait;
   only for a CI failure that is not the code's).
4. Writes the notes: one line per commit since the previous tag, the compare link, and the names
   of the packages.
5. Makes the annotated tag with the notes as its message, and pushes it.
6. Creates the GitHub release with the same notes.
7. Waits for the Release workflow, which the tag push started, to attach the packages.

Stopping it after step 6 loses nothing. A pushed tag is permanent: a bad release is followed by
a fixed one with the next version, and the tag is never moved.

## The packages

The Release workflow (`.github/workflows/release.yml`) runs one job per OS, each of which runs the
script `make dist-<os>` runs, and a last job that attaches what they built to the release with
`SHA256SUMS`. Run by hand from the Actions page (workflow_dispatch) it builds without a release,
and the packages are the run's artifacts: that is how to try a change to the packaging.

| Package | Made by | On |
|---|---|---|
| `telesapr-X.Y.Z-linux-amd64.tar.gz`, `telesapr_X.Y.Z_amd64.deb`, `telesapr-X.Y.Z-1.x86_64.rpm` | `scripts/dist-linux.sh`: what `make install` puts in place, built with `-trimpath -ldflags "-s -w"`; the `.deb` and `.rpm` by nfpm from `dist/linux/nfpm.yaml` | a `debian:bookworm` container, for its glibc |
| `telesapr-X.Y.Z-windows-{amd64,arm64}.zip` and `-setup.exe` | `scripts/dist-windows.sh`: pure Go, cross-compiled, with the icon and manifest of `dist/windows/winres.json`; the installer by Inno Setup from `dist/windows/telesapr.iss` | `windows-latest` |
| `telesapr-X.Y.Z-macos-universal.dmg` | `scripts/dist-macos.sh`: both architectures joined by `lipo` into `TeleSAPR.app` (`dist/macos/Info.plist`), signed ad hoc | `macos-latest` |

`make dist` builds what the host can: on Linux the Linux packages and the Windows zips (the
installers need Inno Setup), on macOS the `.dmg`. The icons for Windows and macOS come from
`scripts/gen-icons.sh`, run when `dist/freedesktop/telesapr.svg` changes.

The binaries are built with `-buildvcs=false`: the container's checkout belongs to another user,
which git refuses to read, and the version is in the code.

## When a build job fails

The release page is there without its packages. Fix the packaging on a branch, try it with
workflow_dispatch, merge, and rerun the failed run from the Actions page (`gh run rerun <id>`):
it checks out the tag, so it builds the released commit with the fixed workflow only when the
fix is in the workflow file itself; a fix in a script needs the next version.

## What is not done

- The macOS app is signed ad hoc, not with a Developer ID, and not notarized: Gatekeeper asks for
  a Control-click > Open the first time. `scripts/dist-macos.sh` says where signing and
  `notarytool` go once there is a certificate.
- The Windows programs are not signed.
- Linux arm64: one more matrix entry with an arm64 runner, when the plan has one.
- The Windows and macOS builds are compiled by the workflow, not tested by it.
