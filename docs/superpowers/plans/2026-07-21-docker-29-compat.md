# Docker 29 Compatibility Implementation Plan (Spec A)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bump luet's container dependency stack for Docker Engine 29 compatibility, remove Docker Content Trust, and prove the result with a Docker-version CI matrix.

**Architecture:** Tasks 1–4 each remove one dependency on an API that disappears in the newer stack, leaving the tree building at every step. Task 5 then performs the actual `go.mod` bump, which only becomes possible once no code references the removed APIs. Tasks 6–8 add verification and documentation.

**Tech Stack:** Go 1.25, ginkgo/gomega (unit), shunit2 (integration), GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-07-21-docker-29-compat-design.md`

## Global Constraints

- Target dependency versions, exactly: `docker/docker v28.5.2+incompatible`, `docker/cli v29.5.3+incompatible`, `go-containerregistry v0.20.6`, `moby/sys/user v0.4.0`, `moby/go-archive v0.2.0`.
- `containerd/containerd` stays at `v1.7.27`. Do not bump it.
- `github.com/moby/moby` and `github.com/theupdateframework/notary` must leave `go.mod` entirely.
- `github.com/docker/distribution` will remain as an **indirect** dependency of go-containerregistry. This is expected; do not try to remove it.
- `github.com/docker/cli` will become **indirect** after Task 3. This is expected.
- Do not migrate to `github.com/moby/moby/client`. Explicitly deferred by the spec.
- Do not touch `pkg/compiler/backend/simpleimg.go` or the `img` backend. That is Spec B.
- The unit suite requires root: run it as `sudo -E env "PATH=$PATH" ...`. Without root, container extraction fails with `failed to Lchown ... operation not permitted`. This is expected and is not a regression.

---

### Task 1: Replace the xattr helpers

`docker/docker/pkg/system` loses `Lgetxattr`/`Lsetxattr` in the target version. `Lsetxattr` maps directly onto `golang.org/x/sys/unix`, but `Lgetxattr` does not — the upstream version allocates internally and returns `[]byte`, while `unix.Lgetxattr` takes a caller-allocated buffer and returns a length. A naive swap silently breaks `copyXattr`, which relies on a nil return to skip absent attributes.

This task is independent of the dependency bump: `golang.org/x/sys` is already a direct dependency.

**Files:**
- Modify: `pkg/helpers/file/file.go:29` (import), `pkg/helpers/file/file.go:205-216` (`copyXattr`)
- Test: `pkg/helpers/file/xattr_test.go` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func lgetxattr(path, attr string) ([]byte, error)` — unexported, in package `file`. Returns `(nil, nil)` when the attribute is absent.

- [ ] **Step 1: Write the failing test**

`lgetxattr` is unexported, so this must be an internal test (`package file`, not `package file_test`). It uses `user.*` xattrs, which are settable without root.

Create `pkg/helpers/file/xattr_test.go`:

```go
package file

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLgetxattrReturnsValue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte("hello")
	if err := unix.Lsetxattr(p, "user.test", want, 0); err != nil {
		t.Skipf("xattrs unsupported on this filesystem: %v", err)
	}

	got, err := lgetxattr(p, "user.test")
	if err != nil {
		t.Fatalf("lgetxattr: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Absent attributes must come back as (nil, nil), not an error.
// copyXattr depends on this to skip attributes that are not set.
func TestLgetxattrAbsentReturnsNil(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := lgetxattr(p, "user.does.not.exist")
	if err != nil {
		t.Fatalf("expected nil error for absent attr, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil value for absent attr, got %q", got)
	}
}

// Values larger than the initial 128-byte buffer exercise the ERANGE
// resize path.
func TestLgetxattrLargeValue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte(strings.Repeat("a", 1024))
	if err := unix.Lsetxattr(p, "user.big", want, 0); err != nil {
		t.Skipf("xattrs unsupported on this filesystem: %v", err)
	}

	got, err := lgetxattr(p, "user.big")
	if err != nil {
		t.Fatalf("lgetxattr: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/helpers/file/ -run TestLgetxattr -v`

Expected: FAIL to compile, `undefined: lgetxattr`.

- [ ] **Step 3: Write the implementation**

In `pkg/helpers/file/file.go`, change the import on line 29 from `"github.com/docker/docker/pkg/system"` to `"golang.org/x/sys/unix"`, then replace `copyXattr` (lines 205-216) with:

```go
// lgetxattr retrieves an extended attribute, returning a nil value when the
// attribute is absent. It mirrors the semantics of the former
// docker/docker/pkg/system.Lgetxattr, which was removed upstream: unlike
// unix.Lgetxattr it allocates its own buffer and reports a missing attribute
// as (nil, nil) rather than ENODATA.
func lgetxattr(path, attr string) ([]byte, error) {
	sysErr := func(err error) ([]byte, error) {
		if err == unix.ENODATA {
			return nil, nil
		}
		return nil, err
	}

	dest := make([]byte, 128)
	sz, err := unix.Lgetxattr(path, attr, dest)
	for err == unix.ERANGE {
		// Buffer too small: query the true size, then retry.
		sz, err = unix.Lgetxattr(path, attr, []byte{})
		if err != nil {
			return sysErr(err)
		}
		dest = make([]byte, sz)
		sz, err = unix.Lgetxattr(path, attr, dest)
	}
	if err != nil {
		return sysErr(err)
	}
	return dest[:sz], nil
}

func copyXattr(srcPath, dstPath, attr string) error {
	data, err := lgetxattr(srcPath, attr)
	if err != nil {
		return err
	}
	if data != nil {
		if err := unix.Lsetxattr(dstPath, attr, data, 0); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/helpers/file/ -run TestLgetxattr -v`

Expected: PASS — three tests.

- [ ] **Step 5: Verify the package still builds and the existing suite passes**

Run: `go build ./... && go test ./pkg/helpers/file/`

Expected: no build output, `ok github.com/mudler/luet/pkg/helpers/file`.

- [ ] **Step 6: Commit**

```bash
git add pkg/helpers/file/file.go pkg/helpers/file/xattr_test.go
git commit -m "refactor(file): replace docker pkg/system xattr calls with x/sys/unix

docker/docker/pkg/system loses Lgetxattr/Lsetxattr in v28. unix.Lgetxattr
has a different contract (caller-allocated buffer, ENODATA error), so add
a local lgetxattr preserving the previous allocate-and-return-nil
semantics that copyXattr depends on."
```

---

### Task 2: Replace `pools.Copy` with `io.Copy`

`docker/docker/pkg/pools` is a buffer-pool wrapper around `io.Copy`. luet uses it at exactly one call site. Dropping it removes a dependency on a package that is not part of docker's supported surface.

**Files:**
- Modify: `pkg/api/core/types/artifact/artifact.go:31` (import), `pkg/api/core/types/artifact/artifact.go:418`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing. Behavior-preserving.

- [ ] **Step 1: Confirm the call site**

Run: `grep -n '"io"\|pools' pkg/api/core/types/artifact/artifact.go`

Expected: three hits — the `io` import (already present, so no import needs adding), the `pools` import on line 31, and `pools.Copy` on line 418.

- [ ] **Step 2: Make the change**

Delete the line `"github.com/docker/docker/pkg/pools"` from the import block, then change line 418 from:

```go
				if _, err := pools.Copy(tarWriter, tarReader); err != nil {
```

to:

```go
				if _, err := io.Copy(tarWriter, tarReader); err != nil {
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`

Expected: no output.

- [ ] **Step 4: Run the artifact suite**

Run: `sudo -E env "PATH=$PATH" go test ./pkg/api/core/types/artifact/`

Expected: `ok github.com/mudler/luet/pkg/api/core/types/artifact`.

- [ ] **Step 5: Commit**

```bash
git add pkg/api/core/types/artifact/artifact.go
git commit -m "refactor(artifact): use io.Copy instead of docker pkg/pools

pools.Copy is a buffer-pool wrapper around io.Copy used at a single call
site. Dropping it removes a dependency on an unsupported docker package."
```

---

### Task 3: Remove Docker Content Trust

This is the blocker for the whole bump: `github.com/docker/cli/cli/trust` does not exist in docker/cli v29, because Content Trust was removed upstream.

Removing it also eliminates luet's direct use of `docker/distribution/reference`, `docker/docker/registry`, and `theupdateframework/notary`.

Per the spec, `verify` continues to parse in both repository YAML and the `--verify` flag, but now warns and proceeds. **This is a deliberate, security-relevant behavior change**: a user who asked for signature verification no longer receives it. It is documented in Task 8.

**Files:**
- Modify: `pkg/helpers/docker/docker.go` — delete `verifyImage` (line 54) and `trustedResolveDigest` (line 83); replace the `verify` branch in `DownloadAndExtractDockerImage` (lines 140-146)

**Interfaces:**
- Consumes: nothing.
- Produces: `DownloadAndExtractDockerImage` keeps its existing signature, unchanged:
  `func DownloadAndExtractDockerImage(ctx luettypes.Context, image, dest string, auth *registrytypes.AuthConfig, verify bool, platform string) (*images.Image, error)`
  The `verify` parameter is retained deliberately so the three existing call sites (`cmd/util.go:136`, `pkg/installer/client/docker.go:100,160`) need no change.

- [ ] **Step 1: Delete the two content-trust functions**

In `pkg/helpers/docker/docker.go`, delete everything from the comment line:

```go
// See also https://github.com/docker/cli/blob/88c6089300a82d3373892adf6845a4fed1a4ba8d/cli/command/image/trust.go#L171
```

up to (but not including):

```go
type staticAuth struct {
```

That removes both `verifyImage` and `trustedResolveDigest`.

- [ ] **Step 2: Remove the now-unused imports**

From the import block, delete these six lines:

```go
	"context"
	"encoding/hex"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/cli/cli/trust"
	"github.com/theupdateframework/notary/tuf/data"
```

Keep `registrytypes "github.com/docker/docker/api/types/registry"` — it is still used by `staticAuth` and the function signature.

- [ ] **Step 3: Replace the verify branch with a warning**

In `DownloadAndExtractDockerImage`, replace:

```go
	if verify {
		img, err := verifyImage(image, auth)
		if err != nil {
			return nil, errors.Wrapf(err, "failed verifying image")
		}
		image = img
	}
```

with:

```go
	if verify {
		// Docker Content Trust was removed from docker/cli in v29, and the
		// notary project behind it is no longer maintained. The flag and the
		// repository `verify:` field are still accepted so existing
		// configurations keep working, but no verification is performed.
		ctx.Warning("image verification is no longer supported and was skipped: " +
			"Docker Content Trust was removed upstream. Pin images by digest instead.")
	}
```

`ctx` is `luettypes.Context`, which embeds a logger exposing `Warning(...interface{})` (`pkg/api/core/types/logger.go:22`).

- [ ] **Step 4: Verify it builds and the import block is clean**

Run: `gofmt -l pkg/helpers/docker/docker.go && go build ./...`

Expected: no output from either. If `gofmt` lists the file, run `gofmt -w pkg/helpers/docker/docker.go`.

- [ ] **Step 5: Confirm no references to trust remain**

Run: `grep -rn "cli/trust\|notary\|verifyImage\|trustedResolveDigest" --include="*.go" pkg/ cmd/`

Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add pkg/helpers/docker/docker.go
git commit -m "feat!: remove Docker Content Trust support

docker/cli v29 removed the cli/trust package and the notary project is
unmaintained. The --verify flag and the repository verify: field still
parse, so existing configs keep working, but they now warn and proceed
without verification.

BREAKING: users who set verify: true no longer get signature
verification. Pin by digest instead."
```

---

### Task 4: Swap `moby/moby/pkg/archive` for `moby/go-archive`

`moby/moby` v26's `pkg/archive` calls `system.StatT`/`Mknod`/`Mkdev`/`FromStatT`, all removed from `docker/docker` v28's `pkg/system`. Keeping both modules makes the bump impossible. `moby/go-archive` is the extracted upstream of the same code.

This is not a bare import swap: go-archive moved the compression constants into a subpackage.

**Files:**
- Modify: `pkg/helpers/archive.go:22` (import), `pkg/helpers/archive.go:32`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nothing.
- Produces: nothing. `helpers.Tar(src, dest string) error` keeps its signature.

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/moby/go-archive@v0.2.0`

Expected: `go: added github.com/moby/go-archive v0.2.0`.

- [ ] **Step 2: Update the imports**

In `pkg/helpers/archive.go`, replace:

```go
	"github.com/moby/moby/pkg/archive"
```

with:

```go
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
```

- [ ] **Step 3: Update the call site**

Change line 32 from:

```go
	fs, err := archive.Tar(src, archive.Uncompressed)
```

to:

```go
	fs, err := archive.Tar(src, compression.None)
```

The signature is `func Tar(path string, comp compression.Compression) (io.ReadCloser, error)`.

- [ ] **Step 4: Verify it builds**

Run: `gofmt -w pkg/helpers/archive.go && go build ./...`

Expected: no output.

- [ ] **Step 5: Run the helpers suite**

Run: `sudo -E env "PATH=$PATH" go test ./pkg/helpers/...`

Expected: all `ok`. `helpers.Tar` is on the artifact packing path, so a failure here matters.

- [ ] **Step 6: Commit**

```bash
git add pkg/helpers/archive.go go.mod go.sum
git commit -m "refactor(helpers): use moby/go-archive instead of moby/moby

moby/moby v26 pkg/archive depends on symbols removed from docker/docker
v28 pkg/system. go-archive is the extracted upstream of the same code;
compression constants moved to a subpackage."
```

---

### Task 5: Bump the dependency stack

With Tasks 1–4 done, no code references the removed APIs, so the bump can land.

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `pkg/compiler/backend/simpledocker.go` — delete `ManifestEntry` (line 250)

**Interfaces:**
- Consumes: Tasks 1–4 must be complete. Attempting this first will fail with `no required module provides package github.com/docker/cli/cli/trust`.
- Produces: nothing.

- [ ] **Step 1: Delete the dead `ManifestEntry` type**

In `pkg/compiler/backend/simpledocker.go`, delete the `ManifestEntry` struct at line 250. It is a leftover from an earlier implementation that parsed `docker save` output directly; luet now delegates that to go-containerregistry.

Confirm it is unreferenced first:

Run: `grep -rn "ManifestEntry" --include="*.go" .`

Expected: exactly one hit, the declaration itself. If there are more, stop and report.

- [ ] **Step 2: Bump the modules**

Run:

```bash
go get github.com/docker/docker@v28.5.2+incompatible \
       github.com/docker/cli@v29.5.3+incompatible \
       github.com/google/go-containerregistry@v0.20.6 \
       github.com/moby/sys/user@v0.4.0
```

Expected: `go: upgraded` lines for each.

- [ ] **Step 3: Tidy**

Run: `go mod tidy`

This pulls in `moby/sys/atomicwriter` and `containerd/errdefs/pkg/errhttp` transitively and drops `moby/moby` and `notary`.

- [ ] **Step 4: Verify the build**

Run: `go build ./...`

Expected: no output. If this fails with errors originating in `docker/docker` or `go-containerregistry` internals, a transitive dependency is missing — `go get` the package named in the error and re-tidy.

- [ ] **Step 5: Verify the module graph matches the spec**

Run:

```bash
grep -E "moby/moby |theupdateframework/notary" go.mod
```

Expected: **no output**. Both must be gone.

Run:

```bash
grep -E "docker/cli|docker/docker |go-containerregistry|moby/go-archive|containerd/containerd |moby/sys/user" go.mod
```

Expected, with `docker/cli` and `moby/sys/user` marked `// indirect`:

```
github.com/containerd/containerd v1.7.27
github.com/docker/docker v28.5.2+incompatible
github.com/google/go-containerregistry v0.20.6
github.com/moby/go-archive v0.2.0
github.com/docker/cli v29.5.3+incompatible // indirect
github.com/moby/sys/user v0.4.0 // indirect
```

`github.com/docker/distribution` remaining as `// indirect` is expected — go-containerregistry pulls it in via `remote/transport`.

- [ ] **Step 6: Run the backend suite — the key regression gate**

Run: `sudo -E env "PATH=$PATH" go test ./pkg/compiler/backend/`

Expected: `ok github.com/mudler/luet/pkg/compiler/backend`.

This drives real `docker build`, `docker save`, and delta extraction through go-containerregistry v0.20.6, and is the single most valuable check that the six-year library jump is safe.

- [ ] **Step 7: Run the full unit suite**

Run: `sudo -E env "PATH=$PATH" go test ./...`

Expected: all packages `ok`.

If `pkg/compiler` fails with `failed to Lchown ... operation not permitted`, the command was not run as root. Re-run with `sudo`.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum pkg/compiler/backend/simpledocker.go
git commit -m "deps: bump docker stack for Docker Engine 29 compatibility

docker/docker v26.1.5 -> v28.5.2, docker/cli v25.0.3 -> v29.5.3 (now
indirect), go-containerregistry v0.14.0 -> v0.20.6, moby/sys/user v0.4.0.
Drops moby/moby and theupdateframework/notary entirely.

go-containerregistry v0.14 does not build against docker v28
(types.ImageLoadResponse), so the two must move together.

Also removes the dead ManifestEntry type, a leftover from parsing
docker save output before that was delegated to go-containerregistry."
```

---

### Task 6: Add the registry roundtrip integration test

The existing suite never exercises a real registry push/pull roundtrip — the docker-repository tests are gated behind `TEST_DOCKER_IMAGE`, which is unset without credentials, so they skip. That leaves the one Docker 29 change the investigation could not rule out untested: v29 pushes a single-platform **OCI index** rather than a plain image ([moby#51532](https://github.com/moby/moby/issues/51532)).

Running against a local `registry:2` means this test always runs, with no credentials.

**Files:**
- Create: `tests/integration/30_registry_roundtrip.sh`

**Interfaces:**
- Consumes: Task 5 complete.
- Produces: nothing. Discovered automatically by `tests/integration/run.sh`, which globs `^[0-9]*_.*.sh`.

- [ ] **Step 1: Write the test**

Follow the shunit2 conventions of the existing suite (`oneTimeSetUp`, `test*` functions, `assertEquals`, sourcing shunit2 last). Model the luet invocations on `tests/integration/01_simple_docker.sh`, reusing the `docker_repo` fixture.

Create `tests/integration/30_registry_roundtrip.sh`:

```bash
#!/bin/bash

export LUET_NOLOCK=true

# Exercises a full build -> create-repo -> push -> pull -> install cycle
# against a local registry. Docker 29 pushes single-platform images as OCI
# indexes rather than plain manifests (moby/moby#51532); this test is what
# catches that, since the quay-backed docker tests skip without credentials.

REGISTRY_PORT=5000
REGISTRY_NAME=luet-test-registry
LOCAL_IMAGE="localhost:${REGISTRY_PORT}/luet-roundtrip"

oneTimeSetUp() {
    export tmpdir="$(mktemp -d)"
    docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1
    docker run -d --name "$REGISTRY_NAME" \
        -p "${REGISTRY_PORT}:5000" registry:2 >/dev/null

    # Wait for the registry to accept connections.
    for _ in $(seq 1 30); do
        if curl -sf "http://localhost:${REGISTRY_PORT}/v2/" >/dev/null; then
            break
        fi
        sleep 1
    done
}

oneTimeTearDown() {
    docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1
    rm -rf "$tmpdir"
}

testRegistryUp() {
    curl -sf "http://localhost:${REGISTRY_PORT}/v2/" >/dev/null
    assertEquals 'local registry is reachable' "0" "$?"
}

testBuild() {
    mkdir -p "$tmpdir/testbuild"
    cat <<EOF > "$tmpdir/default.yaml"
extra: "bar"
foo: "baz"
EOF
    luet build --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
               --destination "$tmpdir/testbuild" --concurrency 1 \
               --image-repository "${LOCAL_IMAGE}-cache" --push \
               --compression zstd --values "$tmpdir/default.yaml" \
               test/c@1.0 test/z test/interpolated
    assertEquals 'builds and pushes cache images successfully' "0" "$?"
    assertTrue 'created package c' "[ -e '$tmpdir/testbuild/c-test-1.0.package.tar.zst' ]"
}

testCreateRepoAndPush() {
    luet create-repo --tree "$ROOT_DIR/tests/fixtures/docker_repo" \
        --output "${LOCAL_IMAGE}" \
        --packages "$tmpdir/testbuild" \
        --name "test" \
        --descr "Test Repo" \
        --urls "$tmpdir/testrootfs" \
        --tree-compression zstd \
        --tree-filename foo.tar \
        --meta-filename repository.meta.tar \
        --meta-compression zstd \
        --type docker --push-images --force-push
    assertEquals 'pushes repository to local registry' "0" "$?"
}

# The pull side is daemonless (go-containerregistry remote), so this is
# where an unexpected OCI index shape surfaces.
testInstallFromRegistry() {
    mkdir -p "$tmpdir/testrootfs"
    cat <<EOF > "$tmpdir/luet.yaml"
general:
  debug: true
system:
  rootfs: $tmpdir/testrootfs
  database_path: "/"
  database_engine: "boltdb"
config_from_host: true
repositories:
   - name: "main"
     type: "docker"
     enable: true
     urls:
       - "${LOCAL_IMAGE}"
EOF
    luet install -y --config "$tmpdir/luet.yaml" test/c@1.0 test/z
    assertEquals 'installs from the local registry' "0" "$?"
    assertTrue 'package c installed' "[ -e '$tmpdir/testrootfs/c' ]"
    assertTrue 'package z installed' "[ -e '$tmpdir/testrootfs/z' ]"
}

. "$ROOT_DIR/tests/integration/shunit2"/shunit2
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x tests/integration/30_registry_roundtrip.sh`

- [ ] **Step 3: Run it in isolation**

Run: `sudo -E env "PATH=$PATH" env SINGLE_TEST=30_registry_roundtrip.sh make test-integration`

Expected: all four tests pass, ending in `Ran 4 tests.` and `OK`.

If `testInstallFromRegistry` fails while push succeeded, that is the moby#51532 OCI-index behavior. Record the exact error and report it before continuing — per the spec, the known workaround is an explicit `--platform` on push, but confirm the diagnosis first rather than applying the workaround blind.

- [ ] **Step 4: Run the full integration suite**

Run: `sudo -E env "PATH=$PATH" make test-integration`

Expected: every script passes. This is the first time the whole suite runs under the new dependency set.

- [ ] **Step 5: Commit**

```bash
git add tests/integration/30_registry_roundtrip.sh
git commit -m "test: add registry push/pull roundtrip integration test

Runs against a local registry:2 so it always executes, unlike the
quay-backed docker tests which skip without credentials. Covers the
Docker 29 change where single-platform images push as OCI indexes
(moby/moby#51532), exercising both the publish and daemonless pull path."
```

---

### Task 7: Add the Docker version CI matrix

CI currently tests exactly one Docker version, installed by `docker-practice/actions-setup-docker@0.0.1`. That makes the compatibility claim an assertion rather than a result. The official `docker/setup-docker-action` supports pinning a version and supplying daemon config, which is what allows toggling the containerd image store.

**Files:**
- Modify: `.github/workflows/pr.yml` — the `tests-integration` job

**Interfaces:**
- Consumes: Tasks 5 and 6 complete.
- Produces: nothing.

- [ ] **Step 1: Replace the `tests-integration` job**

In `.github/workflows/pr.yml`, replace the whole `tests-integration:` job (from `tests-integration:` down to, but not including, `tests-unit:`) with:

```yaml
  tests-integration:
    strategy:
      fail-fast: false
      matrix:
        go-version: [1.25.x]
        platform: [ubuntu-latest]
        docker-version: ["26.1.4", "28.5.2", "29.1.2"]
        containerd-snapshotter: [true, false]
    runs-on: ${{ matrix.platform }}
    name: integration (docker ${{ matrix.docker-version }}, containerd-store ${{ matrix.containerd-snapshotter }})
    steps:
    - name: Install Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go-version }}
    - name: Checkout code
      uses: actions/checkout@v4
    - name: Free Disk Space
      uses: jlumbroso/free-disk-space@v1.3.1
      with:
        tool-cache: false
        android: true
        dotnet: true
        haskell: true
        large-packages: true
        docker-images: true
        swap-storage: true
    - name: setup-docker
      uses: docker/setup-docker-action@v4
      with:
        version: ${{ matrix.docker-version }}
        daemon-config: |
          {
            "features": {
              "containerd-snapshotter": ${{ matrix.containerd-snapshotter }}
            }
          }
    - name: Report docker version
      run: docker version && docker info | grep -i "storage driver"
    - name: Install deps
      run: |
            sudo apt-get update && sudo apt-get install -y upx && sudo -E env "PATH=$PATH" make deps
    - name: Tests
      run: sudo -E env "PATH=$PATH" make test-integration
```

`fail-fast: false` matters here: one Docker version failing should not hide the results for the others.

- [ ] **Step 2: Validate the workflow YAML parses**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/pr.yml')); print('ok')"`

Expected: `ok`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/pr.yml
git commit -m "ci: test integration suite across a Docker version matrix

Runs the integration suite on Docker 26/28/29 with the containerd image
store both on and off, replacing the single unpinned version. Switches to
the official docker/setup-docker-action, which supports version pinning
and daemon config.

This is what turns Docker 29 compatibility from a claim into a result."
```

- [ ] **Step 4: Push and confirm the matrix is green**

```bash
git push -u origin bump-docker-deps
```

Then watch the run:

```bash
gh pr create --fill --title "Docker Engine 29 compatibility" --body "Implements docs/superpowers/specs/2026-07-21-docker-29-compat-design.md"
gh pr checks --watch
```

Expected: six `tests-integration` jobs, all passing.

**This is the merge gate.** If any matrix cell fails, stop and report which Docker version and snapshotter combination broke, with the failing test name — do not proceed to Task 8 with a red matrix.

---

### Task 8: Document the behavior change

The `verify` change is silent at runtime by design — a warning in a long build log is easy to miss. It needs to be discoverable in the docs and release notes.

**Files:**
- Modify: `docs/content/en/docs/Concepts/Overview/build_packages.md` (only if it documents `--verify`; check first)
- Create: `docs/superpowers/specs/2026-07-21-docker-29-compat-RELEASE-NOTES.md`

**Interfaces:**
- Consumes: Task 3 complete.
- Produces: nothing.

- [ ] **Step 1: Find any existing documentation of `--verify` or `verify:`**

Run: `grep -rn "verify" docs/content --include="*.md"`

If any hit documents the flag or the repository field as providing signature verification, update that prose to state it is a no-op retained for compatibility. If there are no hits, skip to Step 2.

- [ ] **Step 2: Write the release note**

Create `docs/superpowers/specs/2026-07-21-docker-29-compat-RELEASE-NOTES.md`:

```markdown
# Release notes — Docker Engine 29 compatibility

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

## Docker Engine 29 support

luet now builds against docker v28.5.2 / docker-cli v29.5.3 and
go-containerregistry v0.20.6, and the integration suite runs against Docker
26, 28, and 29 with the containerd image store both enabled and disabled.

The containerd image store changes `docker save` output to OCI layout. luet
delegates that parsing to go-containerregistry, so no change was required, but
the roundtrip is now covered by `tests/integration/30_registry_roundtrip.sh`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: note content trust removal and Docker 29 support"
```

---

## Completion checklist

Before requesting review, confirm each of these was actually run and observed — not assumed:

- [ ] `go build ./...` clean
- [ ] `sudo -E env "PATH=$PATH" go test ./...` all packages ok
- [ ] `sudo -E env "PATH=$PATH" make test-integration` passes locally
- [ ] `grep -E "moby/moby |theupdateframework/notary" go.mod` returns nothing
- [ ] CI matrix green across all six Docker/snapshotter combinations
- [ ] `pkg/compiler/backend/simpleimg.go` untouched (Spec B scope)
- [ ] `containerd/containerd` still at v1.7.27
