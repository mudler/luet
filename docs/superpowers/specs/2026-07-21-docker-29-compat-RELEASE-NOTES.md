# Release notes: Docker Engine 29 compatibility

## Publishing note: paste this into the release body manually

This file is not picked up by the automated release notes. `.goreleaser.yml`
excludes `^docs:` commits from the generated changelog, and the commit carrying
this file is a `docs:` commit. Without a manual paste, the breaking change below
appears in the published GitHub release only as the `feat!:` subject line of the
trust-removal commit. Copy the contents of this file into the GitHub release
body when cutting the release.

## Breaking: image signature verification removed

Docker Content Trust has been removed. `docker/cli` deleted the `cli/trust`
package in v29, and the notary project behind it is no longer maintained.

**What changed:** the `--verify` flag and the `verify:` field in repository
configuration are still accepted, so existing configurations continue to load
and luet does not fail on them. They no longer verify anything. When either is
set, luet logs a warning and proceeds.

**Who is affected:** anyone with `verify: true` in a repository definition, or
who passes `--verify`. You are no longer getting signature verification.

**What to do instead:** pin images by digest rather than tag, or verify out of
band with cosign before invoking luet.

### Failing closed instead of downgrading silently

The skipped-verification notice is emitted through `Context.Warning`. luet
panics on any warning when `fatal_warnings: true` is set under `general:` in
the configuration. Combining the two:

```yaml
general:
  fatal_warnings: true
```

turns every attempted verification into a hard failure rather than a silent
downgrade. This is the available way to keep failing closed if you cannot
accept unverified pulls, but understand what it costs. It makes *all* warnings
fatal, not only this one, and the mechanism is a literal
`panic("panic on warning")` in `Context.Warning`
(`pkg/api/core/context/context.go:138`). The process aborts with a Go stack
trace rather than exiting cleanly with an error message, so do not adopt it
expecting graceful failure handling.

### Digest-pinned references now actually work

The removed `verifyImage` helper returned an empty string with a nil error for
references that were already digest-pinned, and the caller assigned that result
back to the image name, blanking it. `verify: true` combined with a
digest-pinned image was therefore already broken before this change. Removing
the code path fixes it, which is what makes the "pin by digest instead" advice
above usable.

## Docker Engine 29 support

luet now builds against `github.com/docker/docker` v28.5.2 and
`github.com/google/go-containerregistry` v0.20.6. `github.com/docker/cli` is no
longer compiled against at all: removing Docker Content Trust deleted luet's
only direct import of it, and it is now recorded in `go.mod` as
`github.com/docker/cli v29.5.3+incompatible // indirect`, pulled in only through
the dependency graph. The integration suite is configured to run
against Docker 26, 28, and 29 with the containerd image store both enabled and
disabled. The dependency bump is exercised by the CLI-driven compiler path.
The `docker/docker` client path is expected to be unaffected but is not covered
by the pinned matrix, since the unit test suite still runs against whatever
Docker version is present on the runner. The CI matrix is newly added and has
not yet been observed passing.

The containerd image store changes `docker save` output to OCI layout. luet
delegates that parsing to go-containerregistry, so no change was required.
`tests/integration/30_registry_roundtrip.sh` was added as a regression guard
for the push/pull behavior reported in moby/moby#51532, where a single-platform
push may be published as an OCI index rather than a plain manifest. That shape
was not reproduced here: on Docker 29.1.2 the pushed manifest was observed as a
plain `application/vnd.oci.image.manifest.v1+json`. The test logs the observed
media type rather than asserting a particular one, because the shape varies by
Docker version and image store configuration.

### Known limitation: non-amd64 hosts

`pkg/installer/client/docker.go` passes an empty platform string when pulling
repository images (both call sites of `DownloadAndExtractDockerImage`), so
go-containerregistry resolves with its default platform, `linux/amd64`. If the
OCI-index push shape from moby/moby#51532 does appear in practice, a non-amd64
host would fail to resolve a matching child manifest from that index. This is a
latent risk rather than an observed failure: the index shape did not reproduce
here. The CI matrix is amd64-only, so it cannot surface this either way.

## Possible artifact digest shift on rebuild

`klauspost/compress` moved from 1.17.4 to 1.18.2, pulled in by `moby/go-archive`
under minimal version selection. luet compresses zstd artifacts with
`zstd.NewWriter(dst)` and no encoder options, so the compressed bytes come from
the library's default encoder settings. A minor version bump of the encoder can
change its output for identical input.

Nothing breaks as a result. Decompression is fully backward compatible, luet
computes artifact checksums at build time, and there are no hardcoded digests
or fixture checksums in the tree. But **artifact digests may shift when
packages are rebuilt**, even with unchanged sources. If you pin or compare
published artifact digests across builds, expect to re-record them after
upgrading.
