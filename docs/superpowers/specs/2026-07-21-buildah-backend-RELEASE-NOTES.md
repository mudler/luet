# Release notes: the img backend is replaced by buildah

> These notes must be pasted into the GitHub release body manually.
> `.goreleaser.yml` excludes `^docs:` commits from the generated changelog.

## Breaking: `img` is no longer used

The `genuinetools/img` backend has been replaced by
[buildah](https://buildah.io). img has been unmaintained since 2018 and the
version CI pinned was released in 2020; it cannot be expected to work against
current buildkit.

**What changed:** `--backend buildah` is the daemonless backend.
`--backend img` still works, now resolving to buildah, and prints a
deprecation warning. It will be removed in a future major version.

**What to do:** install `buildah` on hosts that ran rootless luet builds, and
switch `--backend img` to `--backend buildah`. The `img` binary is no longer
needed.

## Improvements

- `create-repo --type docker` now works on the daemonless backend. The img
  backend returned "Not supported" for image loading, so this was previously
  impossible without Docker.
- Local image existence checks no longer produce false positives. The img
  backend substring-matched an image listing, so any image whose name
  contained the queried name as a substring reported as present. The buildah
  backend uses `buildah inspect --type image` instead.

## Rootless limitations

Rootless buildah needs `BUILDAH_ISOLATION=chroot`, `STORAGE_DRIVER=vfs`, and
the `SETUID`/`SETGID` capabilities. Two well-known properties of rootless
container builds follow from that:

- `vfs` is expected to be slower than `overlay`, especially for large images.
  Exposing `/dev/fuse` lets buildah use `fuse-overlayfs` instead.
- `mknod` is blocked for unprivileged users regardless of capabilities. This
  is a kernel restriction on user namespaces. Packages whose build creates
  device nodes cannot be built rootless.

## Test coverage

The CI migration keeps the buildah job's test surface identical to the old img
job's. The integration tests that skip on the daemonless backend still skip;
switching to buildah does not by itself widen coverage of that path.

## Kubernetes

[luet-k8s](https://github.com/mudler/luet-k8s) still requests the `img`
backend, which continues to work through the deprecation alias. Updating it to
buildah is tracked separately.
