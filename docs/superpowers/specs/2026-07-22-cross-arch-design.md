# True Cross-Arch Support — Design

Date: 2026-07-22
Status: Approved, pending implementation plan
Branch: `cross-arch` (from `master`)

## Problem

luet has essentially no cross-arch support. Architecture is an *implicit*
property of whichever machine ran `luet build`.

There is exactly one arch-aware feature in the codebase — a client-side repo
enable/disable filter:

```go
// pkg/api/core/types/repository.go:55
func (r *LuetRepository) Enabled() bool {
	return r.Arch != "" && r.Arch == runtime.GOARCH && !r.Enable || r.Enable
}
```

That is not cross-arch support. It is a workaround for its absence: you publish
N separate repositories, one per arch, and each client enables the one that
matches. (Note also the precedence footgun — `&&` binds tighter than `||`, so
`Enable: true` wins unconditionally.)

Everything else is arch-blind. The root cause is that one string is used as
identity everywhere:

```go
// pkg/api/core/types/package.go:301
// FIXME: this needs to be unique, now just name is generalized
func (p *Package) GetFingerPrint() string {
	return fmt.Sprintf("%s-%s-%s", p.Name, p.Category, p.Version)
}
```

`GetFingerPrint()` feeds package DB keys, `ImageID()` and therefore every docker
tag, artifact filenames (`<fp>.package.tar`), `GetMetadataFilePath()`, the
build-cache hash tree, and solver equality via `Matches()`. Consequently **two
builds of the same package for different arches are indistinguishable at every
layer, and collide.**

### Known-broken today

| Location | Defect |
|---|---|
| `pkg/api/core/types/artifact/artifact.go:228` | `image.CreateTar(..., runtime.GOARCH, runtime.GOOS)` — final image config always claims the *builder's* arch |
| `pkg/installer/client/docker.go:106,177` | passes `platform: ""`; ggcr then defaults index resolution to `linux/amd64`, so an arm64 host silently receives amd64 content |
| `pkg/api/core/types/compilerspec.go:136` | `Signature` omits arch — a build-cache image from one arch is considered valid for another |
| `pkg/compiler/backend/common.go:69` | `genBuildCommand` never emits `--platform` |
| `pkg/helpers/docker/docker.go:146` | `ExtractDockerImage`'s `else` branch redeclares `err` with `:=`, shadowing the variable the trailing error check reads; a failed `remote.Image` therefore falls through with a nil image and panics in `img.Manifest()` |
| `pkg/installer/finalizer.go` | executes target-arch binaries on the host CPU with no arch check |

`pkg/solver/`, `pkg/tree/`, `pkg/database/`, and `pkg/api/core/types/config.go`
contain no arch references whatsoever.

## Scope

Both halves of "cross-arch":

- **Distribute & install** foreign-arch packages (repository + client work)
- **Build** foreign-arch packages on a native host (compiler + backend work)

Explicitly out of scope: multi-arch co-installation into a single rootfs
(Debian-multiarch-style i386-on-amd64). luet targets OS images, where a rootfs
is one arch. This exclusion is what makes the design below possible.

## Design

### 1. Model — per-arch universes

Arch is **not** added to `Package`. `pkg/solver`, `pkg/database`, and `pkg/tree`
are untouched.

This is the load-bearing constraint; every other decision defers to it. The
solver is the highest-risk surface in the codebase and was recently stabilised
(commits `4d346152`, `d99b70fd`, `5e6544d3`). Putting arch into package identity
would add a dimension the solver must constrain on every `Requires`/`Conflicts`
edge, reopening exactly that surface.

Instead, platform appears in three places:

- `PackageArtifact.Platform` — what a built artifact *is*
- **Repository index partition** — a repo holds N arch-homogeneous universes
- **Build run target** — what one `luet build` invocation produces

Within any single partition or build run everything is arch-homogeneous, so
`name-category-version` remains a sufficient key and `GetFingerPrint()` is
unchanged. This mirrors how Debian and Alpine work (per-arch `Packages` files).

The pre-existing `FIXME` on `GetFingerPrint()` is deliberately **not** resolved
here. Fixing it is orthogonal and would be a breaking change on its own.

### 2. Platform type and storage naming

One canonical type with two renderings, converted only at the storage boundary.

| Form | Used in | Example |
|---|---|---|
| OCI | YAML, CLI flags, image config, `v1.ParsePlatform` | `linux/arm/v7` |
| Sanitized | filenames, docker tags | `linux-arm-v7` |

OCI platform strings are chosen because luet's build backends and its docker
repo client already sit on OCI tooling that speaks this format —
`pkg/helpers/docker/docker.go:93` already calls `v1.ParsePlatform`. Anything
narrower (bare `GOARCH`) collapses `arm/v6` and `arm/v7`.

Sanitized form is required because `/` is illegal in filenames and awkward in
tags. Affected names:

- `<fp>-<plat>.package.tar`
- `<fp>-<plat>.metadata.yaml`
- `<fp>-<plat>.image.tar`, `<fp>-<plat>-builder.image.tar`
- all pushed image tags
- `repository-<plat>.yaml`, `tree-<plat>.tar`, `compilertree-<plat>.tar`,
  `repository-<plat>.meta.yaml`

The suffix is appended by the artifact/image naming layer, **not** baked into
`GetFingerPrint()`.

### 3. Build path

- `luet build --platform linux/arm64`, defaulting to the host platform. One run
  targets one platform; N arches is N runs (parallel in CI).
- New `CompilerOptions.TargetPlatform`, with a matching `WithPlatform`
  functional option in `pkg/compiler/options.go`.
- **`Signature` gains the target platform.** Non-optional — without it,
  build-cache images cross-contaminate between arches. This is a correctness
  fix, not a feature.
- `backend.Options` gains `Platform`; `genBuildCommand` emits `--platform`.
  Shared by both the docker and buildah backends.
- `artifact.go:228` — the hardcoded `runtime.GOARCH, runtime.GOOS` becomes the
  build's target platform. **Highest-severity latent bug**: without this fix
  every child manifest in a published index declares the builder's arch, so the
  index silently resolves every platform to the same child.
- `build.yaml` gains an optional `platforms:` list:

  ```yaml
  platforms:            # absent = builds for any target
    - linux/amd64
    - linux/arm64
  ```

  A build run targeting a platform not in the list **skips** that package with a
  log line rather than failing, so a whole-tree `--platform linux/arm/v7` run
  does the right thing instead of exploding on one unbuildable package.

  This lives in `build.yaml`, not `definition.yaml`, because it is a statement
  about buildability. Keeping `definition.yaml` untouched is what preserves the
  promise that `Package`, the solver, and the DB do not change.
- Target platform injected into the template context as `{{.Values.platform}}`,
  with `{{.Values.arch}}` / `{{.Values.os}}` conveniences, so specs stop needing
  parallel per-arch values files.
- Seed-image preflight: inspect the seed image's index; if it has no variant for
  the target, fail early with `seed image X has no linux/arm64 variant` rather
  than letting docker produce a confusing downstream error.

**Emulation is docker's responsibility, not luet's.** luet emits `--platform`
and lets buildx/binfmt handle execution. luet never registers binfmt handlers
and never manages qemu; that is host setup, documented as a prerequisite.

### 4. Distribution — two-phase publishing

An OCI index can only be assembled after all per-arch builds exist, and those
are separate runs on separate machines. luet therefore **cannot** create an
index during a build run. This is the same constraint that makes
`docker buildx imagetools create` and `podman manifest push` separate commands.

**Phase 1** — each build run pushes arch-qualified tags only. No indexes.

**Phase 2** — a new `luet manifest create` command reads the N arch-qualified
tags and pushes an index referencing them, via `empty.Index` +
`mutate.AppendManifests` + `remote.WriteIndex`. luet already imports `mutate`,
`empty`, `remote`, and `name` from go-containerregistry v0.20.6, so **no new
dependency**.

Its interface is the destination tag plus the platforms to gather:

```
luet manifest create myrepo/foo:1.0 --platform linux/amd64 --platform linux/arm64
```

Source tags are derived by sanitized-suffix convention (`myrepo/foo:1.0-linux-amd64`),
not passed explicitly, so the command stays symmetric with what phase 1 pushed.
A missing source tag is a hard error naming the absent platform — never a
silently smaller index.

Treatment differs by image kind:

| Image kind | Treatment | Rationale |
|---|---|---|
| Build-cache images | arch-qualified tags, never an index | cache is per-build-run, never consumed by a foreign-arch client; an index adds a round-trip and buys nothing |
| Final package images | **index** | one tag that a plain `docker pull` resolves correctly — the entire point |
| Docker-type repo files | arch-qualified tags + partition listing at the stable tag | see below |

Repo metadata is deliberately **not** published as an OCI index keyed by
platform. It is tempting (clients would get their partition free via
`WithPlatform`) but it abuses platform semantics for non-image content and
breaks `crane ls` discoverability. Explicit arch-qualified tags are debuggable.

### 5. Client / install path

- `repository.yaml` at the stable location becomes a thin partition listing
  ("I have linux/amd64, linux/arm64"); the client fetches
  `repository-linux-arm64.yaml`.
- `pkg/installer/client/docker.go` passes the real target platform instead of
  `""`, fixing the arm64-receives-amd64 bug.
- `LuetSystemConfig` gains a target platform, so `--system-target` into a
  foreign rootfs is explicit rather than inferred from the host.
- Install refuses a partition that mismatches the system target, with a clear
  message.
- `LuetRepository.Arch` and its `Enabled()` filter are deprecated, then removed.
  One repo entry now serves all arches; the client self-selects. This retires
  the "publish N separate repos" workaround and the precedence footgun.

### 6. Finalizers on cross-arch installs

`pkg/installer/finalizer.go` runs finalizers either directly on the host (target
`/`) or via the `pkg/box` chroot. Both execute **target-arch binaries on the
host CPU**. With no binfmt registered, a cross-arch install with finalizers dies
on `Exec format error`.

**Decision: skip with a loud warning, and record as pending.** When the target
platform differs from the host, finalizers are not executed; they are recorded
as pending so they can run on first boot on real hardware.

Concretely: pending finalizers are written to a file under the *target rootfs*
(not the host system DB), so that the record travels with the image being built
— a rootfs cross-built on a CI machine must carry its own pending work. A new
`luet finalize` command drains that file and is what an OS image's first-boot
unit invokes. The exact on-disk location and format are an implementation
detail to settle in the plan; the requirement is that the record lives inside
the target rootfs and is drainable by a single command.

This is the established pattern for cross-built OS images and is what makes the
primary use case (building an arm64 rootfs on an amd64 machine) work at all.
Failing fast would block it entirely. Attempting execution via binfmt is
deferred to a possible later opt-in flag — it is the most capable option but the
most magic, and the hardest to debug when a handler is subtly misconfigured.

The warning must be unmissable and must name the skipped finalizers.

### 7. Compatibility and migration

- A repo with **no partition listing** is a legacy single-partition repo,
  consumed exactly as today.
- An artifact with **no platform suffix** resolves as it does today.
- New clients read old repos. Old clients read old repos.
- Old clients **cannot** read new multi-arch repos. This is accepted. A repo
  format version bump makes the failure a clear message rather than a crash.
- `GetFingerPrint()` is unchanged, so existing on-disk artifact names and
  on-registry tags for legacy single-arch repos keep resolving byte-identically.

## Testing

**Unit**
- `Platform` parse/sanitize round-trip, including `linux/arm/v7`
- `Signature` differs across target platforms (regression guard for cache
  cross-contamination)
- `platforms:` skip logic — matching, non-matching, and absent

**Integration** (`tests/`)
- Build one package for two platforms; assert distinct artifacts, distinct cache
  images, no filename or tag collision
- Compose an index from two arch tags; assert **each child's image config
  reports the correct arch** — the direct regression test for the
  `artifact.go:228` bug, which is invisible on a same-arch host
- Cross-arch install into a rootfs via `--system-target`, asserting correct
  partition selection and that finalizers are skipped-and-recorded rather than
  executed

## Risks

| Risk | Mitigation |
|---|---|
| `artifact.go:228` fix missed or regressed — index silently resolves wrong | dedicated integration test asserting per-child config arch |
| Old clients hitting new repos | repo format version bump → clear error |
| Scope creep into solver | design explicitly forbids touching `Package`; treat any solver diff as a red flag in review |
| binfmt/qemu misconfiguration blamed on luet | luet never manages emulation; document host prerequisites and surface docker's error verbatim |

## Sequencing

This spec is larger than one implementation plan. It decomposes into three
groups, each of which should get its own plan and its own review cycle:

- **Group A — bug fixes (steps 1–2).** Standalone value, no format changes,
  shippable independently and immediately.
- **Group B — build side (steps 3–5).** Produces correct per-arch artifacts.
- **Group C — distribution side (steps 6–9).** Multi-arch repos, indexes,
  finalizers, deprecation.

Group B depends on A; C depends on B. Ordered so each step is independently
useful and testable:

1. `Platform` type + tests (sanitized form deferred to group B, where its first caller appears)
2. `artifact.go:228` fix and `client/docker.go` platform threading — **these are
   bug fixes with standalone value and can ship first**
3. `Signature` + `CompilerOptions.TargetPlatform` + `backend.Options.Platform` +
   `--platform` flag
4. `build.yaml` `platforms:` + template injection + seed preflight
5. Arch-qualified storage naming across artifacts and repo files
6. Repository partitioning + client partition selection
7. `luet manifest create`
8. Finalizer skip-and-record
9. Deprecate `LuetRepository.Arch`
