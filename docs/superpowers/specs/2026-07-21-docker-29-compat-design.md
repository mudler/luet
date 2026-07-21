# Docker 29 Compatibility — Design (Spec A)

Date: 2026-07-21
Status: Approved, pending implementation plan
Branch: `bump-docker-deps`

## Problem

luet pins a container dependency stack from 2023–2024 (`docker/cli` v25.0.3,
`docker/docker` v26.1.5, `moby/moby` v26.1.0, `go-containerregistry` v0.14.0).
Docker Engine 29 has since shipped three changes that matter:

- the containerd image store is the default for fresh installs
- the daemon requires API v1.44 or later
- Docker Content Trust was removed from the CLI
- `github.com/docker/docker` is deprecated in favour of `moby/moby/client` + `/api`

### What is actually broken

The distinction matters, because the initial assumption was a runtime break and
that turned out to be wrong.

**Runtime on Docker 29 works today.** Verified on Docker 29.1.2 (API 1.52,
containerd image store, buildkit with attestations):

| Path | Result |
|---|---|
| `go build ./...` at HEAD | passes |
| `pkg/compiler/backend` suite (real `docker build` / `save` / delta) | passes |
| `crane.Load` on buildkit `docker save` output (`ondisk=true`) | passes, 2 layers |
| `daemon.Image` (`ondisk=false`) | passes |

`docker save` did change to OCI layout (`blobs/sha256/…`, `index.json`,
`oci-layout`), but luet never parses that itself — it hands the tar to
go-containerregistry, which copes.

A first run of the compiler suite showed 28/34 failures, but every one was
`failed to Lchown ... operation not permitted` from running as an unprivileged
user. CI runs the suite under `sudo`. This was not a Docker issue.

**The dependency stack is what breaks.** Attempting the bump surfaces a
cascade, in order:

1. `github.com/docker/cli/cli/trust` no longer exists in v29 (Content Trust
   removed upstream). `pkg/helpers/docker/docker.go` depends on it.
2. `go-containerregistry` v0.14.0 does not build against `docker/docker` v28 —
   `types.ImageLoadResponse` undefined.
3. `moby/moby` v26.1.0's `pkg/archive` calls `system.StatT` / `Mknod` / `Mkdev`
   / `FromStatT`, all removed from `docker/docker` v28's `pkg/system`.
4. `docker/distribution` v2 is deprecated in favour of `distribution/reference`.

## Scope

In scope: dependency bump, content trust removal, e2e verification harness.

Out of scope: the `genuinetools/img` backend replacement. That is Spec B — a
buildah backend, chosen because buildah preserves img's value proposition
(standalone, rootless, daemonless host binary) and `simpleimg.go` is a CLI
shell-out wrapper, making it a near-mechanical port. Adapting
[luet-k8s](https://github.com/mudler/luet-k8s) is a follow-up in that separate
repo and is not covered by either spec.

## Dependency changes

All verified by trial bump in a throwaway worktree.

| Module | From | To | Why |
|---|---|---|---|
| `docker/cli` | v25.0.3 (direct) | v29.5.3 (**indirect**) | unblocked once trust is gone |
| `docker/docker` | v26.1.5 | v28.5.2 | Docker 29 daemon compat |
| `go-containerregistry` | v0.14.0 | v0.20.6 | required — v0.14 fails against docker v28 |
| `moby/sys/user` | v0.3.0 | v0.4.0 | required — `idtools` needs `MkdirAllAndChown` |
| `moby/go-archive` | — | v0.2.0 | replaces `moby/moby/pkg/archive` |
| `theupdateframework/notary` | v0.7.0 | removed | content trust |
| `moby/moby` | v26.1.0 | removed | `pkg/archive` breaks against docker v28 |
| `docker/distribution` | v2.8.2 (direct) | v2.8.3 (**indirect**) | see below |

Transitive additions: `moby/sys/atomicwriter`, `containerd/errdefs/pkg/errhttp`.

`containerd/containerd` stays at v1.7.27; nothing in the cascade requires moving it.

Two notes on the resulting graph, both confirmed by building the full change set:

- **`docker/cli` becomes indirect.** Content trust was luet's only direct import
  of it, so removing that drops `docker/cli` out of the direct requirements
  entirely. The version bump still matters for the module graph, but luet no
  longer compiles against its API.
- **`docker/distribution` cannot be fully removed.** It survives as an indirect
  dependency of go-containerregistry itself
  (`crane` → `remote/transport` → `registry/client/auth/challenge`). Removing
  luet's direct use is all that is in scope.

### Deliberately deferred

`github.com/docker/docker` is deprecated upstream in favour of
`github.com/moby/moby/client` + `github.com/moby/moby/api`. luet only consumes
leaf packages (`api/types/registry`), so this design stays on `docker/docker`
and records the migration as future work rather than widening the blast radius
of an already dependency-heavy change.

`moby/sys/xattr` has no tagged releases, so the xattr call sites move to
`golang.org/x/sys/unix` (already a dependency) rather than to that module.

## Code changes

Five sites.

### 1. `pkg/helpers/docker/docker.go` — remove content trust

Delete `verifyImage` (line 54) and `trustedResolveDigest` (line 83). This alone
removes four imports: `cli/trust`, `notary/tuf/data`, `distribution/reference`,
and `docker/docker/registry`.

Retain the `verify bool` parameter on `DownloadAndExtractDockerImage`; it
becomes the warning choke point (see below).

### 2. `pkg/helpers/archive.go:32`

`moby/moby/pkg/archive` → `moby/go-archive`.

Not a bare import swap: go-archive moved the compression constants into a
`compression` subpackage. The call becomes
`archive.Tar(src, compression.None)`, with an added import of
`github.com/moby/go-archive/compression`.

### 3. `pkg/helpers/file/file.go:206,211`

`system.Lgetxattr` / `system.Lsetxattr` → `golang.org/x/sys/unix`.

`Lsetxattr` is a direct swap. `Lgetxattr` is **not** — the signatures differ:

```go
system.Lgetxattr(path, attr string) ([]byte, error)   // allocates internally
unix.Lgetxattr(path, attr string, dest []byte) (int, error)  // caller-allocated
```

A local `lgetxattr` helper is required to preserve existing semantics: start
with a 128-byte buffer, retry on `ERANGE` by querying the true size, and return
a nil value (not an error) on `ENODATA`, since `copyXattr` relies on the nil
check to skip absent attributes.

### 4. `pkg/api/core/types/artifact/artifact.go:418`

`pools.Copy` → `io.Copy`. Single call.

### 5. `pkg/compiler/backend/simpledocker.go:250`

Delete `ManifestEntry`. It is dead code — a leftover from an earlier
implementation that parsed `docker save` output directly, with zero references
in the codebase. luet now delegates that parsing to go-containerregistry.

## The `verify` flag

`verify` is public API in two forms: the `verify:` field in repository YAML
(`pkg/api/core/types/repository.go:37`) and the `--verify` CLI flag
(`cmd/util.go:160`). Three call sites pass it: `cmd/util.go:116` and
`pkg/installer/client/docker.go:100,160`.

**Decision: warn and continue.** The field and flag continue to parse, so no
existing configuration breaks. When `verify` is true, luet emits a warning and
proceeds without verification.

Rather than warn at three call sites, warn once at the single choke point inside
`DownloadAndExtractDockerImage`.

The warning is emitted at `Warning` level, not debug, so it is visible in normal
output.

**This is a security-relevant behavior change and must be called out in the
release notes.** A user who explicitly requested signature verification will
stop receiving it. The alternative considered was failing closed with an
explicit error, which never silently downgrades a security control; warn-and-
continue was chosen instead to avoid hard-failing existing `verify: true`
configurations. The tradeoff is accepted knowingly: upstream Docker Content
Trust and notary are themselves effectively dead, so the guarantee being lost
was already thin. Users wanting verification should pin by digest or adopt
cosign.

## E2E verification

### Matrix

Run the existing `tests/integration` suite across:

```
docker:           [26, 28, 29]
containerd-store: [on, off]
```

Today CI exercises exactly one Docker version, so the compatibility claim is
asserted rather than tested. The matrix is what converts it into a verified
one, and it is the gate on merging.

### New test: `tests/integration/30_registry_roundtrip.sh`

build → `create-repo` → push → pull → install, against a local `registry:2`.

This covers the one v29 change that could not be ruled out during
investigation: [moby#51532](https://github.com/moby/moby/issues/51532), where
v29 pushes a single-platform **OCI index** rather than a plain image. That path
runs through luet's docker-repository publish code
(`pkg/installer/repository_docker.go`) and its daemonless pull side
(`pkg/helpers/docker/docker.go`), neither of which the current suite exercises
against a real registry.

If the roundtrip fails, the known workaround is an explicit `--platform` on push.

## Risk

The principal risk is **go-containerregistry 0.14 → 0.20**, not the Docker bump.
luet uses `crane`, `remote`, `tarball`, `mutate`, `daemon`, and `empty` across
the compiler, installer, and image packages. It is a six-year jump that touches
the delta and extract hot path (`pkg/api/core/image/delta.go`,
`extract.go`, `pkg/compiler/compiler.go`).

This risk is now partly retired: the `pkg/compiler/backend` suite passes against
v0.20.6 on Docker 29 (see verification status below). The residual exposure is
the compiler and installer paths that suite does not reach, which is what the
root-privileged run and the integration matrix are there to cover.

Secondary risk: removing `moby/moby` changes the tar implementation behind
`helpers.Tar`. `moby/go-archive` is the extracted upstream of the same code, so
behavior should be identical, but artifact packing is on the critical path.

### Verification status at design time

A prototype of this entire change set — all five code changes plus the full
dependency bump — was built and tested in a throwaway worktree.

Proven:

- `go build ./...` succeeds with the complete target set, including
  `docker/cli` v29.5.3. No docker-related compile errors remain; the only
  failures encountered were the two API-shape issues now documented above.
- `pkg/compiler/backend` suite passes on Docker 29.1.2 **with
  go-containerregistry v0.20.6**. This substantially retires the primary risk:
  the suite drives real `docker build`, `docker save`, and delta extraction
  through the upgraded library.
- `crane.Load` and `daemon.Image` handle buildkit/OCI-layout `docker save`
  output.
- `moby/moby` and `theupdateframework/notary` leave the module graph cleanly.

Not yet proven — these remain the merge gates:

- the full compiler suite as root (requires a sudo password unavailable during
  investigation)
- the `tests/integration` suite on any Docker version under the new deps
- the registry push/pull roundtrip on v29

## Sequencing

1. Spec A (this document) — deps, trust removal, e2e harness. Ships first so the
   compatibility fix is not gated behind new backend work.
2. Spec B — buildah backend, reusing this harness to prove parity against the
   docker backend.
3. Follow-up — adapt luet-k8s to buildah, in that repository.
