# Cut a release

Pushing a `v*` tag is the whole trigger. Everything else is automatic.

## Before you start

- You're on `main`, up to date, and CI is green.
- Nothing to bump by hand: the version is stamped from the tag at build time
  (`.goreleaser.yaml` → `-X …/internal/cli.Version={{ .Tag }}`). There is no
  version constant in the source.
- Pick a version. Users get it via `tuki update`, which compares semver, so
  `v0.3.0` must sort above the current tag.

```sh
git checkout main && git pull
make check                            # fmt, vet, test
gh run list --limit 3                 # last CI runs should be green
git describe --tags --abbrev=0        # what's current
```

## Release

```sh
git tag -a v0.3.0 -m "tuki v0.3.0"
git push origin v0.3.0
```

Then watch it:

```sh
gh run watch --repo rmpato/tuki       # or: gh run list --limit 3
```

`.github/workflows/release.yml` fires on the tag and runs GoReleaser, which
runs the tests again, cross-compiles, and publishes a GitHub release with a
changelog built from the commits since the last tag.

Takes about two minutes.

## Verify

```sh
# Six assets: five archives plus checksums.txt.
gh release view v0.3.0 --repo rmpato/tuki --json assets --jq '[.assets[].name]'
```

Expected, exactly — these names are load-bearing (see Gotchas):

```
checksums.txt
tuki_0.3.0_darwin_amd64.tar.gz
tuki_0.3.0_darwin_arm64.tar.gz
tuki_0.3.0_linux_amd64.tar.gz
tuki_0.3.0_linux_arm64.tar.gz
tuki_0.3.0_windows_amd64.zip
```

Then confirm the two paths users actually take, in a sandbox so you don't
disturb your own install:

```sh
# 1. The install script, into a throwaway HOME.
TMP=$(mktemp -d)
HOME=$TMP sh install.sh --no-modify-path --dir "$TMP/bin"
"$TMP/bin/tuki" --version          # should print v0.3.0 and the commit

# 2. Self-update, from a deliberately old build.
go build -ldflags "-X github.com/rmpato/tuki/internal/cli.Version=v0.0.1" -o "$TMP/old" .
"$TMP/old" update --check          # should offer v0.3.0
rm -rf "$TMP"
```

## If it goes wrong

The tag is the only state. Delete the release and the tag, fix, tag again:

```sh
gh release delete v0.3.0 --repo rmpato/tuki --yes --cleanup-tag
git tag -d v0.3.0
git fetch --prune --tags
```

`--cleanup-tag` removes the remote tag too. If the release was never created
(the workflow failed early), delete the tag directly:

```sh
git push --delete origin v0.3.0
git tag -d v0.3.0
```

Re-tagging the *same* version after anyone has installed it is a bad idea —
their `tuki update` won't offer a version it thinks it already has. If the
release is already public, ship `v0.3.1` instead.

## Gotchas

**Archive names are a contract.** Both `install.sh` and
`internal/update/update.go` (`AssetName`) *construct* the filename rather than
discovering it. If you change `name_template` in `.goreleaser.yaml`, change
both, or updates break for everyone. There's a test pinning the format:
`TestAssetNameMatchesGoReleaser`.

**`checksums.txt` has a fixed name** for the same reason — the default would
include the version. `tuki update` refuses to install anything it can't verify
against it.

**Pre-releases** work as expected: `v0.3.0-rc1` is marked pre-release
automatically (`prerelease: auto`), and `Compare` in `internal/update` sorts a
pre-release below its final version.

**`@latest` needs a tag.** `go install github.com/rmpato/tuki@latest` resolves
to the newest tag, not to `main`. Commits without a tag are invisible to it.

## Dry run

To check a GoReleaser change without tagging anything:

```sh
goreleaser release --snapshot --clean --skip=publish
ls dist/
```
