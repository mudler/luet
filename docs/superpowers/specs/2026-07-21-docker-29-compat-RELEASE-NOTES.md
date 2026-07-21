# Release notes: Docker Engine 29 compatibility

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
downgrade. This is the supported way to keep failing closed if you cannot
accept unverified pulls. Note that it makes *all* warnings fatal, not only
this one.

### Digest-pinned references now actually work

The removed `verifyImage` helper returned an empty string with a nil error for
references that were already digest-pinned, and the caller assigned that result
back to the image name, blanking it. `verify: true` combined with a
digest-pinned image was therefore already broken before this change. Removing
the code path fixes it, which is what makes the "pin by digest instead" advice
above usable.

## Docker Engine 29 support

luet now builds against docker v28.5.2 / docker-cli v29.5.3 and
go-containerregistry v0.20.6, and the integration suite is configured to run
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
