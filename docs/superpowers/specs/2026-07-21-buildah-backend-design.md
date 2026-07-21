# buildah Compiler Backend — Design (Spec B)

Date: 2026-07-21
Status: Approved, pending implementation plan
Branch: `buildah-backend`

## Problem

luet's second compiler backend shells out to
[`genuinetools/img`](https://github.com/genuinetools/img). CI pins v0.5.11, released
in 2020. The project has been effectively unmaintained since 2018, is built
against a buildkit from that era, and cannot be expected to work against modern
buildkit or containerd.

The backend is not decorative. `docs/content/en/docs/Concepts/Overview/build_packages.md`
documents it as the way to build "without any docker daemon running in the
host", and it is what
[luet-k8s](https://github.com/mudler/luet-k8s) uses to build packages inside
Kubernetes pods as an unprivileged user. Losing it would remove luet's entire
daemonless and rootless build story.

## Why buildah

buildah preserves img's value proposition where the alternatives do not:

- **buildah** is a standalone rootless binary with no daemon, actively
  maintained by the containers project. Structurally it is the same model as
  img, which matters because `pkg/compiler/backend/simpleimg.go` is a CLI
  shell-out wrapper — the port is near-mechanical.
- **buildctl** is img's actual lineage (img wrapped buildkit), but requires a
  running `buildkitd`. luet would have to manage a daemon lifecycle it does not
  have today.
- **kaniko** is purpose-built for daemonless Kubernetes builds but must run
  *inside* a container; it mutates the root filesystem and can destroy a host if
  executed directly. luet's backend model is "exec a binary on the host", which
  kaniko fundamentally violates. It is also largely unmaintained upstream now.

Rootless operation in an unprivileged pod is confirmed supported with
`BUILDAH_ISOLATION=chroot`, `STORAGE_DRIVER=vfs`, and the `SETUID`/`SETGID`
capabilities, running as `runAsUser: 1000` / `runAsNonRoot: true`. That matches
luet-k8s's existing model exactly.

## Scope

In scope: replacing the img backend in the `luet` repository.

Out of scope: adapting `mudler/luet-k8s`. That is Spec C, sequenced immediately
after this lands and a luet release exists. The deprecated `img` alias below is
what makes that sequencing safe rather than a forced two-repo release.

## Backend selection

`buildah` becomes a real backend and `simpleimg.go` is deleted. `--backend img`
continues to resolve, emitting a deprecation warning and returning the buildah
backend:

```go
case backend.BuildahBackend:
    compilerBackend = backend.NewSimpleBuildahBackend(ctx)
case backend.ImgBackend:
    ctx.Warning("--backend img is deprecated and now uses buildah; " +
        "switch to --backend buildah")
    compilerBackend = backend.NewSimpleBuildahBackend(ctx)
```

This keeps luet-k8s and every existing rootless user working without a
coordinated release, while retiring the dead code immediately. The alias should
be removed in a later major version.

`ImgBackend = "img"` is retained as a constant for the alias; a new
`BuildahBackend = "buildah"` is added alongside it in
`pkg/compiler/backend/common.go`.

## Command mapping

All twelve `CompilerBackend` methods map onto buildah.

| Method | img (today) | buildah |
|---|---|---|
| `BuildImage` | `img build -f X -t Y ctx` | `buildah build -f X -t Y ctx` |
| `ExportImage` | `img save -o path name` | `buildah push name docker-archive:path` |
| `LoadImage` | returns `"Not supported"` | `buildah pull docker-archive:path` |
| `RemoveImage` | `img rm name` | `buildah rmi name` |
| `CopyImage` | `img tag src dst` | `buildah tag src dst` |
| `DownloadImage` | `img pull name` | `buildah pull name` |
| `Push` | `img push name` | `buildah push name` |
| `ImageReference` | `img save -o tmp` then `crane.Load` | `buildah push name docker-archive:tmp` then `crane.Load` |
| `ImageExists` | `img ls` + `strings.Contains` | `buildah inspect --type image name` |
| `ImageAvailable` | shared `image.Available()` | unchanged |
| `ImageDefinitionToTar` | build + export + remove | unchanged composition |

`genBuildCommand` in `pkg/compiler/backend/common.go` is already shared between
the docker and img backends and emits `build -f <dockerfile> -t <name> <context>`
with `opts.BackendArgs` prepended. buildah accepts that same shape, so the
builder needs no change.

### Two deliberate improvements

These are not transliteration, and are called out so they are not mistaken for
scope creep:

**`LoadImage` stops being a stub.** `SimpleImg.LoadImage` currently returns
`errors.New("Not supported")`. Because `artifact.GenerateFinalImage` calls it,
`create-repo --type docker` cannot work on the rootless path at all today.
buildah can pull a docker-archive, so this capability exists for the first time.

**`ImageExists` stops being a substring match.** `img ls` piped through
`strings.Contains` returns a false positive for any image whose name contains
the queried name as a substring. `buildah inspect --type image` communicates
through its exit code, matching how `SimpleDocker.ImageExists` already works.

## Rootless configuration

luet passes its environment through to the build engine and does not manage
engine configuration itself. `build_packages.md:49` documents this for
`DOCKER_HOST` and `DOCKER_BUILDKIT`. buildah follows the same pattern:
`BUILDAH_ISOLATION` and `STORAGE_DRIVER` are set by the pod spec or the invoking
shell, and `--backend-args` remains available for per-invocation flags.

No new luet flags or configuration keys are added for storage driver or
isolation mode. Spec C documents the required pod environment for luet-k8s.

## Assumptions requiring empirical verification

buildah is not installed on the development machine used for this design, so the
command mapping above is derived from documentation rather than observation. Two
assumptions are load-bearing and must be settled as the first step of
implementation, before the port is written:

1. **That `buildah push <name> docker-archive:<path>` produces a tarball
   `crane.Load` accepts.** go-containerregistry's `tarball` reader expects
   docker-archive layout with a `manifest.json`. buildah also offers an
   `oci-archive:` transport, which it would *not* accept. The exact transport
   string may additionally require a `:<tag>` suffix. `ImageReference` and
   `ExportImage` both depend on this, and `ImageReference` is on the delta hot
   path for every package built.
2. **That `buildah build` accepts luet's generated Dockerfiles unchanged**,
   specifically the `COPY --from=<image>` multi-stage instructions emitted by
   `compilerspec.go:genDockerfile`.

If either fails, the design changes rather than the implementation working
around it, which is why they are verified first.

## Known regressions

Two things get worse under buildah than under img. Both should be documented for
users rather than discovered by them:

- **vfs is markedly slower than overlay**, particularly for large images. img
  used a fuse-overlayfs snapshotter. Where a cluster permits exposing
  `/dev/fuse`, buildah can use fuse-overlayfs and recover most of the
  difference.
- **`mknod` is blocked for rootless users regardless of granted capabilities.**
  This is a kernel restriction on unprivileged user namespaces, not a
  configuration mistake. Any package whose build creates device nodes will fail
  under rootless buildah.

## Testing

The `tests-integration-img` CI job becomes `tests-integration-buildah`, running
the existing integration suite with `LUET_BACKEND=buildah`. buildah installs
from apt, replacing the current `curl` of a pinned img release binary. The job
exists in three workflows (`pr.yml`, `tests.yml`, `push.yml`); all three change
together.

`tests/integration/30_registry_roundtrip.sh` currently skips itself when
`LUET_BACKEND=img`, because `SimpleImg.Push` has no insecure-registry support
for the plain-HTTP local registry. Whether buildah can run it should be
evaluated during implementation: buildah supports `--tls-verify=false`, and
combined with the newly working `LoadImage` this would be the first coverage of
the docker-repository path on the rootless backend. If it works, the skip guard
becomes buildah-specific rather than blanket. If it does not, the guard stays
and the reason is recorded.

## Risk

The port itself is low-risk and mechanical. The real risk is behavioural drift
between buildkit (img) and buildah on identical Dockerfiles: build caching
semantics, layer boundaries, and produced layer content all differ between the
two engines. luet's delta computation
(`pkg/api/core/image/delta.go`, `extract.go`) works by flattening the builder
and runner images and diffing file paths, so a difference in how buildah lays
out layers surfaces as wrong package contents rather than a loud failure.

The full integration suite under `LUET_BACKEND=buildah` is what catches this,
which is why the CI job matters more here than any unit test.

## Sequencing

1. Spec B (this document) — buildah backend in `luet`, with the `img` alias.
2. Release luet.
3. Spec C — `mudler/luet-k8s`: swap `ImgBackend` for the buildah backend
   constant, replace `container/img` with buildah in its `Dockerfile`, and set
   the rootless pod environment (`BUILDAH_ISOLATION=chroot`,
   `STORAGE_DRIVER=vfs`, `SETUID`/`SETGID` capabilities).
