# buildah Compiler Backend Implementation Plan (Spec B)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unmaintained `genuinetools/img` compiler backend with buildah, keeping `--backend img` as a deprecated alias.

**Architecture:** Task 1 empirically settles two assumptions the design could not verify, and is a hard gate — if either fails the design changes rather than the implementation working around it. Tasks 2-3 build the new backend, Task 4 wires it up and deletes the old one, Tasks 5-6 handle CI and docs.

**Tech Stack:** Go 1.25, buildah CLI, ginkgo/gomega (unit), shunit2 (integration), GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-07-21-buildah-backend-design.md`

## Global Constraints

- New backend constant is exactly `BuildahBackend = "buildah"`, added alongside the retained `ImgBackend = "img"` in `pkg/compiler/backend/common.go`.
- `--backend img` must keep working. It resolves to the buildah backend and emits a deprecation warning. Do NOT make it an error.
- Do NOT add luet flags or config keys for storage driver or isolation mode. Those come from the environment (`BUILDAH_ISOLATION`, `STORAGE_DRIVER`) and `--backend-args`, matching how luet already handles `DOCKER_HOST`/`DOCKER_BUILDKIT`.
- Do NOT modify `pkg/compiler/backend/simpledocker.go` or the docker backend's behavior.
- Do NOT touch `mudler/luet-k8s` — separate repo, Spec C.
- `genBuildCommand` in `common.go` is shared with the docker backend. buildah accepts the same `build -f X -t Y ctx` shape, so it must NOT be changed.
- The unit suite requires root: run as `sudo -E env "PATH=$PATH" ...`. Without root, container extraction fails with `failed to Lchown ... operation not permitted`. Pre-existing, not a regression.

---

### Task 1: Verify the two load-bearing assumptions

**This is a gate, not a formality.** buildah was not available when the design was written, so the entire command mapping is from documentation. Two assumptions carry the design. If either is false, STOP and report rather than improvising — `ImageReference` is on the delta hot path for every package built, so a wrong transport here corrupts package contents rather than failing loudly.

**Files:**
- Create: `pkg/compiler/backend/buildah_probe_test.go` (temporary; deleted in Step 6)

**Interfaces:**
- Consumes: nothing.
- Produces: a confirmed exact transport string for later tasks, of the form `docker-archive:<path>` or `docker-archive:<path>:<tag>`. Task 2 uses whichever this task proves.

- [ ] **Step 1: Install buildah**

Run: `sudo apt-get update && sudo apt-get install -y buildah`

Then: `buildah --version`

Expected: a version string. If the distro package is older than 1.23, note the version in your report — `buildah build` was named `buildah bud` before then, and the subcommand name matters.

- [ ] **Step 2: Verify `buildah build` accepts a multi-stage Dockerfile**

luet's `compilerspec.go:genDockerfile` emits `COPY --from=<image>` for `Copy` directives. Confirm buildah handles that shape.

```bash
cd "$(mktemp -d)"
cat > Dockerfile <<'EOF'
FROM alpine:latest AS builder
RUN echo builder-content > /built.txt

FROM alpine:latest
COPY --from=builder /built.txt /from-builder.txt
RUN echo runner-content > /runner.txt
EOF
buildah build -f Dockerfile -t luet-buildah-probe:1 .
```

Expected: build succeeds. Verify the copy actually worked:

```bash
buildah from --name probe-check luet-buildah-probe:1
buildah run probe-check -- cat /from-builder.txt
buildah rm probe-check
```

Expected: prints `builder-content`.

- [ ] **Step 3: Determine the exact working docker-archive transport string**

go-containerregistry's `tarball` reader requires docker-archive layout with a `manifest.json`. buildah's `oci-archive:` transport would NOT be accepted. Try both forms and record which works:

```bash
buildah push luet-buildah-probe:1 docker-archive:/tmp/probe-notag.tar
echo "no-tag exit: $?"
buildah push luet-buildah-probe:1 docker-archive:/tmp/probe-tag.tar:luet-buildah-probe:1
echo "tag exit: $?"
tar tf /tmp/probe-tag.tar | head
```

Expected: at least one succeeds and its tar contains `manifest.json`. Record which form worked and whether the tar layout is legacy (`<hash>/layer.tar`) or OCI (`blobs/sha256/...`) — go-containerregistry handles both, but the report should state which.

- [ ] **Step 4: Write the probe test proving `crane.Load` accepts buildah's output**

This is the assertion that actually matters. Create `pkg/compiler/backend/buildah_probe_test.go`:

```go
package backend_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
)

// TestBuildahDockerArchiveRoundtrip proves that a buildah-produced
// docker-archive is readable by go-containerregistry, which is what
// SimpleBuildah.ImageReference and ExportImage depend on.
func TestBuildahDockerArchiveRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("buildah"); err != nil {
		t.Skip("buildah not installed")
	}

	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile")
	content := "FROM alpine:latest AS builder\n" +
		"RUN echo builder-content > /built.txt\n" +
		"FROM alpine:latest\n" +
		"COPY --from=builder /built.txt /from-builder.txt\n"
	if err := os.WriteFile(df, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("buildah", "build", "-f", df, "-t", "luet-probe:1", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("buildah build: %v\n%s", err, out)
	}
	defer exec.Command("buildah", "rmi", "luet-probe:1").Run()

	tarPath := filepath.Join(dir, "probe.tar")
	out, err = exec.Command("buildah", "push", "luet-probe:1", "docker-archive:"+tarPath).CombinedOutput()
	if err != nil {
		t.Fatalf("buildah push docker-archive: %v\n%s", err, out)
	}

	img, err := crane.Load(tarPath)
	if err != nil {
		t.Fatalf("crane.Load rejected buildah output: %v", err)
	}
	layers, err := img.Layers()
	if err != nil {
		t.Fatalf("Layers: %v", err)
	}
	if len(layers) == 0 {
		t.Fatal("expected at least one layer")
	}
	cf, err := img.ConfigFile()
	if err != nil {
		t.Fatalf("ConfigFile: %v", err)
	}
	t.Logf("OK: %d layers, arch=%s os=%s", len(layers), cf.Architecture, cf.OS)
}
```

- [ ] **Step 5: Run the probe test**

Run: `go test ./pkg/compiler/backend/ -run TestBuildahDockerArchiveRoundtrip -v`

Expected: PASS, logging the layer count.

**If `crane.Load` fails:** STOP. Do not proceed to Task 2 and do not try `oci-archive:` as a silent substitute — go-containerregistry's `tarball` reader does not accept OCI archives, so a passing workaround would mean something other than what it appears. Report the exact error, the tar layout from Step 3, and the buildah version.

**If the transport needed the `:<tag>` suffix**, record the exact working string; Task 2 must use that form.

- [ ] **Step 6: Delete the probe test and record findings**

The probe has served its purpose; Task 2 adds real tests. Delete it:

Run: `rm pkg/compiler/backend/buildah_probe_test.go`

Write your findings into the task report: buildah version, which transport string worked, tar layout, and whether multi-stage `COPY --from` succeeded. Task 2 depends on all four.

- [ ] **Step 7: Commit**

Nothing to commit if the probe was deleted and no source changed. Confirm with `git status --short` that the tree is clean, and record the findings in the report only.

---

### Task 2: SimpleBuildah — build, export, and image reference

The three methods that depend on Task 1's findings. Kept separate from the simpler wrappers because these carry the risk.

**Files:**
- Create: `pkg/compiler/backend/simplebuildah.go`
- Create: `pkg/compiler/backend/simplebuildah_test.go`

**Interfaces:**
- Consumes: Task 1's confirmed docker-archive transport string.
- Produces:
  - `func NewSimpleBuildahBackend(ctx types.Context) *SimpleBuildah`
  - `func (s *SimpleBuildah) BuildImage(opts Options) error`
  - `func (s *SimpleBuildah) ExportImage(opts Options) error`
  - `func (s *SimpleBuildah) ImageReference(a string, ondisk bool) (v1.Image, error)`
  Task 3 adds the remaining nine methods to the same `SimpleBuildah` type.

- [ ] **Step 1: Write the failing test**

Create `pkg/compiler/backend/simplebuildah_test.go`. It skips when buildah is absent, so it is safe on machines without it.

```go
package backend_test

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/compiler/backend"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Buildah backend", func() {
	ctx := context.NewContext()

	BeforeEach(func() {
		if _, err := exec.LookPath("buildah"); err != nil {
			Skip("buildah not installed")
		}
	})

	Context("Builds, exports and references images", func() {
		It("builds an image, exports it, and reads it back", func() {
			b := NewSimpleBuildahBackend(ctx)

			tmpdir, err := os.MkdirTemp("", "buildah-test")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir)

			dockerfile := filepath.Join(tmpdir, "Dockerfile")
			Expect(os.WriteFile(dockerfile,
				[]byte("FROM alpine:latest\nRUN echo hello > /hello.txt\n"), 0o600)).ToNot(HaveOccurred())

			opts := Options{
				ImageName:      "luet/buildah-test:1",
				SourcePath:     tmpdir,
				DockerFileName: dockerfile,
				Destination:    filepath.Join(tmpdir, "out.tar"),
			}

			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			defer b.RemoveImage(opts)

			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(fileExists(opts.Destination)).To(BeTrue())

			img, err := b.ImageReference(opts.ImageName, true)
			Expect(err).ToNot(HaveOccurred())
			layers, err := img.Layers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(layers)).To(BeNumerically(">", 0))
		})
	})
})

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/compiler/backend/ -run TestBackend -v 2>&1 | head -20`

Expected: compile failure, `undefined: NewSimpleBuildahBackend`.

- [ ] **Step 3: Write the implementation**

Create `pkg/compiler/backend/simplebuildah.go`. This mirrors `simpledocker.go`'s structure deliberately — same logging, same event bus calls, same error wrapping.

**Use whichever docker-archive transport string Task 1 proved.** The code below uses the no-tag form; if Task 1 found the `:<tag>` suffix was required, use that instead and note it in the commit message.

```go
// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package backend

import (
	"os/exec"

	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/mudler/luet/pkg/api/core/bus"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// dockerArchive builds the containers/image transport string used to move
// images between buildah and go-containerregistry. It must be docker-archive
// rather than oci-archive: go-containerregistry's tarball reader only accepts
// the former.
func dockerArchive(path string) string {
	return "docker-archive:" + path
}

type SimpleBuildah struct {
	ctx types.Context
}

func NewSimpleBuildahBackend(ctx types.Context) *SimpleBuildah {
	return &SimpleBuildah{ctx: ctx}
}

func (s *SimpleBuildah) BuildImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePreBuild, opts)

	buildarg := genBuildCommand(opts)
	s.ctx.Info(":tea: Building image " + name)

	cmd := exec.Command("buildah", buildarg...)
	cmd.Dir = opts.SourcePath
	if err := runCommand(s.ctx, cmd); err != nil {
		return err
	}

	s.ctx.Success(":tea: Building image " + name + " done")
	bus.Manager.Publish(bus.EventImagePostBuild, opts)

	return nil
}

func (s *SimpleBuildah) ExportImage(opts Options) error {
	name := opts.ImageName
	path := opts.Destination

	s.ctx.Debug(":tea: Saving image " + name)
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "push", name, dockerArchive(path)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed exporting image: "+string(out))
	}

	s.ctx.Success(":tea: Image " + name + " saved")
	return nil
}

// ImageReference returns a go-containerregistry handle on the image. buildah
// has no daemon to query, so ondisk is ignored and the image is always routed
// through a docker-archive on disk.
func (s *SimpleBuildah) ImageReference(a string, ondisk bool) (v1.Image, error) {
	f, err := s.ctx.TempFile("snapshot")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "push", a, dockerArchive(f.Name())).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "Failed saving image: "+string(out))
	}

	img, err := crane.Load(f.Name())
	if err != nil {
		return nil, err
	}

	return img, nil
}
```

Note the import block deliberately does NOT include `pkg/api/core/image`. Nothing in this task uses it; `ImageAvailable` in Task 3 is its only consumer, and Go fails the build on an unused import.

Note also that `SimpleImg.ImageReference` leaked its temp file; this does not. That is a deliberate small fix, not scope creep, and should be mentioned in the commit message.

- [ ] **Step 4: Run the test to verify it passes**

Run: `sudo -E env "PATH=$PATH" go test ./pkg/compiler/backend/ -v 2>&1 | tail -20`

Expected: the Buildah backend spec passes. If buildah is not installed the spec skips, which is not a pass — confirm it actually ran.

- [ ] **Step 5: Verify the build is clean**

Run: `go build ./... && gofmt -l pkg/compiler/backend/`

Expected: no output from either.

- [ ] **Step 6: Commit**

```bash
git add pkg/compiler/backend/simplebuildah.go pkg/compiler/backend/simplebuildah_test.go
git commit -m "feat(backend): add buildah build, export and image reference

Implements the three CompilerBackend methods that move images between
buildah and go-containerregistry via a docker-archive. Mirrors the
structure of simpledocker.go.

Unlike SimpleImg.ImageReference, the temporary archive is cleaned up."
```

---

### Task 3: SimpleBuildah — remaining nine methods

**Files:**
- Modify: `pkg/compiler/backend/simplebuildah.go`
- Modify: `pkg/compiler/backend/simplebuildah_test.go`

**Interfaces:**
- Consumes: `SimpleBuildah` and `NewSimpleBuildahBackend(ctx types.Context) *SimpleBuildah` from Task 2.
- Produces: `*SimpleBuildah` satisfying the full `compiler.CompilerBackend` interface — `LoadImage(string) error`, `RemoveImage(Options) error`, `CopyImage(string, string) error`, `DownloadImage(Options) error`, `Push(Options) error`, `ImageAvailable(string) bool`, `ImageExists(string) bool`, `ImageDefinitionToTar(Options) error`.

- [ ] **Step 1: Write the failing test**

Two behaviors are worth asserting beyond "it does not error". Append to the `Describe("Buildah backend", ...)` block in `pkg/compiler/backend/simplebuildah_test.go`:

```go
	Context("Reports image existence accurately", func() {
		It("does not false-positive on substring matches", func() {
			b := NewSimpleBuildahBackend(ctx)

			tmpdir, err := os.MkdirTemp("", "buildah-exists")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir)

			dockerfile := filepath.Join(tmpdir, "Dockerfile")
			Expect(os.WriteFile(dockerfile,
				[]byte("FROM alpine:latest\n"), 0o600)).ToNot(HaveOccurred())

			opts := Options{
				ImageName:      "luet/exists-test:1",
				SourcePath:     tmpdir,
				DockerFileName: dockerfile,
			}
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			defer b.RemoveImage(opts)

			Expect(b.ImageExists("luet/exists-test:1")).To(BeTrue())
			// "exists-test" is a substring of the real image name. The old
			// img backend matched with strings.Contains and would return
			// true here, which is wrong.
			Expect(b.ImageExists("exists-test")).To(BeFalse())
			Expect(b.ImageExists("luet/definitely-absent:1")).To(BeFalse())
		})
	})

	Context("Round-trips an image through a docker archive", func() {
		It("exports and loads an image back", func() {
			b := NewSimpleBuildahBackend(ctx)

			tmpdir, err := os.MkdirTemp("", "buildah-load")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(tmpdir)

			dockerfile := filepath.Join(tmpdir, "Dockerfile")
			Expect(os.WriteFile(dockerfile,
				[]byte("FROM alpine:latest\nRUN echo loadme > /loadme.txt\n"), 0o600)).ToNot(HaveOccurred())

			opts := Options{
				ImageName:      "luet/load-test:1",
				SourcePath:     tmpdir,
				DockerFileName: dockerfile,
				Destination:    filepath.Join(tmpdir, "load.tar"),
			}
			Expect(b.BuildImage(opts)).ToNot(HaveOccurred())
			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(b.RemoveImage(opts)).ToNot(HaveOccurred())
			Expect(b.ImageExists(opts.ImageName)).To(BeFalse())

			// LoadImage returned "Not supported" on the img backend, so this
			// capability is new.
			Expect(b.LoadImage(opts.Destination)).ToNot(HaveOccurred())
			defer b.RemoveImage(opts)
			Expect(b.ImageExists(opts.ImageName)).To(BeTrue())
		})
	})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/compiler/backend/ 2>&1 | head -20`

Expected: compile failure, `b.ImageExists undefined` and `b.LoadImage undefined`.

- [ ] **Step 3: Write the implementation**

First add the one import this task needs and Task 2 deliberately omitted. In the import block of `pkg/compiler/backend/simplebuildah.go`, add:

```go
	"github.com/mudler/luet/pkg/api/core/image"
```

It is used only by `ImageAvailable` below.

Then append to `pkg/compiler/backend/simplebuildah.go`:

```go
// LoadImage imports a docker-archive produced by ExportImage. The img backend
// could not do this at all, which is why create-repo --type docker did not
// work on the daemonless path.
func (s *SimpleBuildah) LoadImage(path string) error {
	s.ctx.Debug(":tea: Loading image:", path)

	out, err := exec.Command("buildah", "pull", dockerArchive(path)).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed loading image: "+string(out))
	}

	s.ctx.Success(":tea: Loaded image:", path)
	return nil
}

func (s *SimpleBuildah) RemoveImage(opts Options) error {
	name := opts.ImageName

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "rmi", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed removing image: "+string(out))
	}

	s.ctx.Success(":tea: Removed image:", name)
	return nil
}

func (s *SimpleBuildah) CopyImage(src, dst string) error {
	s.ctx.Debug(":tea: Tagging image:", src, "->", dst)

	out, err := exec.Command("buildah", "tag", src, dst).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed tagging image: "+string(out))
	}

	s.ctx.Success(":tea: Tagged image:", src, "->", dst)
	return nil
}

func (s *SimpleBuildah) DownloadImage(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePull, opts)

	s.ctx.Debug(":tea: Downloading image " + name)
	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "pull", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pulling image: "+string(out))
	}

	s.ctx.Success(":tea: Downloaded image:", name)
	bus.Manager.Publish(bus.EventImagePostPull, opts)

	return nil
}

func (s *SimpleBuildah) Push(opts Options) error {
	name := opts.ImageName
	bus.Manager.Publish(bus.EventImagePrePush, opts)

	s.ctx.Spinner()
	defer s.ctx.SpinnerStop()

	out, err := exec.Command("buildah", "push", name).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "Failed pushing image: "+string(out))
	}

	s.ctx.Success(":tea: Pushed image:", name)
	bus.Manager.Publish(bus.EventImagePostPush, opts)

	return nil
}

func (*SimpleBuildah) ImageAvailable(imagename string) bool {
	return image.Available(imagename)
}

// ImageExists reports whether the image is present locally. It uses buildah
// inspect rather than matching against the output of buildah images: the img
// backend used strings.Contains over `img ls`, which false-positives on any
// name containing the queried name as a substring.
func (s *SimpleBuildah) ImageExists(imagename string) bool {
	s.ctx.Debug(":tea: Checking existence of image: " + imagename)

	cmd := exec.Command("buildah", "inspect", "--type", "image", imagename)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.ctx.Debug("Image not present")
		s.ctx.Debug(string(out))
		return false
	}
	return true
}

func (s *SimpleBuildah) ImageDefinitionToTar(opts Options) error {
	if err := s.BuildImage(opts); err != nil {
		return errors.Wrap(err, "Failed building image")
	}
	if err := s.ExportImage(opts); err != nil {
		return errors.Wrap(err, "Failed exporting image")
	}
	if err := s.RemoveImage(opts); err != nil {
		return errors.Wrap(err, "Failed removing image")
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `sudo -E env "PATH=$PATH" go test ./pkg/compiler/backend/ -v 2>&1 | tail -25`

Expected: all Buildah backend specs pass, including the substring and load round-trip cases.

If the substring assertion fails (i.e. `ImageExists("exists-test")` returns true), `buildah inspect` is resolving partial names — report it rather than relaxing the assertion, because the whole point of that method change is to stop false positives.

- [ ] **Step 5: Verify the interface is fully satisfied**

Add a compile-time assertion at the bottom of `pkg/compiler/backend/simplebuildah.go`. This is how the code proves it implements the interface, rather than waiting for the factory in Task 4 to fail:

```go
// Compile-time check that SimpleBuildah satisfies the backend contract.
// The interface lives in package compiler, so this is asserted there in
// backend.go's factory; this local assertion catches drift earlier.
var _ interface {
	BuildImage(Options) error
	ExportImage(Options) error
	LoadImage(string) error
	RemoveImage(Options) error
	ImageDefinitionToTar(Options) error
	CopyImage(string, string) error
	DownloadImage(Options) error
	Push(Options) error
	ImageAvailable(string) bool
	ImageReference(string, bool) (v1.Image, error)
	ImageExists(string) bool
} = (*SimpleBuildah)(nil)
```

Run: `go build ./...`

Expected: no output. A missing or misnamed method fails here with a clear message.

- [ ] **Step 6: Commit**

```bash
git add pkg/compiler/backend/simplebuildah.go pkg/compiler/backend/simplebuildah_test.go
git commit -m "feat(backend): complete the buildah CompilerBackend implementation

Adds the remaining nine methods. Two are improvements over the img
backend rather than transliterations:

- LoadImage works. SimpleImg returned \"Not supported\", which meant
  create-repo --type docker could not run on the daemonless path at all.
- ImageExists uses buildah inspect instead of substring-matching the
  output of an image listing, which false-positived on any name
  containing the queried name."
```

---

### Task 4: Wire up the factory, deprecate img, delete the old backend

**Files:**
- Modify: `pkg/compiler/backend/common.go:25-28` (constants)
- Modify: `pkg/compiler/backend.go:11-24` (factory)
- Delete: `pkg/compiler/backend/simpleimg.go`
- Modify: `cmd/build.go:337` and `cmd/create-repo.go:197` (flag help text)

**Interfaces:**
- Consumes: `NewSimpleBuildahBackend` from Tasks 2-3.
- Produces: `backend.BuildahBackend = "buildah"` constant.

- [ ] **Step 1: Add the constant**

In `pkg/compiler/backend/common.go`, change the const block at lines 25-28 from:

```go
const (
	ImgBackend    = "img"
	DockerBackend = "docker"
)
```

to:

```go
const (
	// ImgBackend is retained as a deprecated alias for BuildahBackend.
	// genuinetools/img is unmaintained; see the buildah backend design.
	ImgBackend     = "img"
	BuildahBackend = "buildah"
	DockerBackend  = "docker"
)
```

- [ ] **Step 2: Rewire the factory**

In `pkg/compiler/backend.go`, replace the `switch` in `NewBackend` (lines 14-21) with:

```go
	switch s {
	case backend.BuildahBackend:
		compilerBackend = backend.NewSimpleBuildahBackend(ctx)
	case backend.ImgBackend:
		ctx.Warning("--backend img is deprecated and now uses buildah; " +
			"switch to --backend buildah")
		compilerBackend = backend.NewSimpleBuildahBackend(ctx)
	case backend.DockerBackend:
		compilerBackend = backend.NewSimpleDockerBackend(ctx)
	default:
		return nil, errors.New("invalid backend. Unsupported")
	}
```

`ctx` is already a parameter of `NewBackend(ctx types.Context, s string)`, and `types.Context` exposes `Warning(...interface{})` via the logger it embeds (`pkg/api/core/types/logger.go:22`).

- [ ] **Step 3: Delete the img backend**

Run: `git rm pkg/compiler/backend/simpleimg.go`

Confirm nothing else referenced it:

Run: `grep -rn "SimpleImg\|NewSimpleImgBackend" --include="*.go" .`

Expected: no output. If there are hits, stop and report — something depends on it that this plan did not account for.

- [ ] **Step 4: Update the two flag help strings**

`cmd/build.go:337`:

```go
	buildCmd.Flags().String("backend", "docker", "backend used (docker,img)")
```

becomes:

```go
	buildCmd.Flags().String("backend", "docker", "backend used (docker,buildah)")
```

`cmd/create-repo.go:197`:

```go
	createrepoCmd.Flags().String("backend", "docker", "backend used (docker,img)")
```

becomes:

```go
	createrepoCmd.Flags().String("backend", "docker", "backend used (docker,buildah)")
```

The default stays `docker`. `img` is deliberately not advertised in the help text even though it still works, because it is deprecated.

- [ ] **Step 5: Verify the build and the full backend suite**

Run: `go build ./... && gofmt -l pkg/ cmd/`

Expected: no output from either.

Run: `sudo -E env "PATH=$PATH" go test ./pkg/compiler/...`

Expected: all packages `ok`.

- [ ] **Step 6: Verify the deprecation alias actually works**

This is the behavior luet-k8s depends on, so prove it rather than assuming:

```bash
go build -o /tmp/luet-alias-check . && /tmp/luet-alias-check build --backend img --help 2>&1 | head -5
```

Expected: the command runs (does not error with "invalid backend"). The deprecation warning appears when a build actually runs, not on `--help`; confirming the flag is accepted is sufficient here.

- [ ] **Step 7: Commit**

```bash
git add -A pkg/compiler cmd/build.go cmd/create-repo.go
git commit -m "feat!: replace the img backend with buildah

genuinetools/img is unmaintained since 2018 and CI pinned a 2020 release.
buildah keeps the same standalone, rootless, daemonless model.

--backend img still resolves, now to buildah, with a deprecation warning,
so luet-k8s and existing users keep working without a coordinated
release. The alias should be removed in a future major version.

BREAKING: the img binary is no longer used. Hosts running rootless builds
must install buildah."
```

---

### Task 5: Update CI

buildah replaces img in **two jobs per workflow**, across three workflow files. The img install appears in both the integration job and the unit-test job, and there is a `img login` step for quay that needs a buildah equivalent.

**Files:**
- Modify: `.github/workflows/pr.yml`
- Modify: `.github/workflows/tests.yml`
- Modify: `.github/workflows/push.yml`

**Interfaces:**
- Consumes: Task 4 complete.
- Produces: nothing.

- [ ] **Step 1: Find every img reference across the workflows**

Run: `grep -n "img" .github/workflows/pr.yml .github/workflows/tests.yml .github/workflows/push.yml`

Expected: references in `tests-integration-img` job names, `curl`-and-`chmod` install pairs, `img login` steps, and `LUET_BACKEND=img`. Work from this list so none is missed.

- [ ] **Step 2: Replace the install steps**

Everywhere this pair appears:

```yaml
            sudo curl -fSL "https://github.com/genuinetools/img/releases/download/v0.5.11/img-linux-amd64" -o "/usr/bin/img"
            sudo chmod a+x "/usr/bin/img"
```

replace both lines with:

```yaml
            sudo apt-get install -y buildah
```

Note it must land inside the existing `run: |` block that already runs `apt-get update`, so the package list is current.

- [ ] **Step 3: Replace the registry login steps**

Everywhere this appears (`tests.yml:29-30`, `push.yml:34-35`):

```yaml
      - name: Login to quay with img
        run: echo ${{ secrets.DOCKER_TESTING_PASSWORD }} | sudo img login -u ${{ secrets.DOCKER_TESTING_USERNAME }} --password-stdin quay.io
```

replace with:

```yaml
      - name: Login to quay with buildah
        run: echo ${{ secrets.DOCKER_TESTING_PASSWORD }} | sudo buildah login -u ${{ secrets.DOCKER_TESTING_USERNAME }} --password-stdin quay.io
```

- [ ] **Step 4: Rename the jobs and switch the backend variable**

Rename the job key `tests-integration-img` to `tests-integration-buildah`, its `name:` from `Integration tests with img` to `Integration tests with buildah` (and in `pr.yml`, the step name `Tests with Img backend` to `Tests with buildah backend`), and every `LUET_BACKEND=img` to `LUET_BACKEND=buildah`.

Note for the reviewer: this changes the reported check names. The repository currently has no branch protection or rulesets configured, so no required check breaks. If that changes before this merges, re-verify.

- [ ] **Step 5: Evaluate whether the registry roundtrip test can now run**

`tests/integration/30_registry_roundtrip.sh` skips itself under `LUET_BACKEND=img` because `SimpleImg.Push` had no insecure-registry support for the plain-HTTP local registry. buildah supports `--tls-verify=false`, and `LoadImage` now works, so this may be able to run and would be the first coverage of the docker-repository path on the daemonless backend.

Check what the guard currently keys on:

Run: `grep -n "LUET_BACKEND" tests/integration/30_registry_roundtrip.sh`

Expected: four `[ "$LUET_BACKEND" == "img" ] && startSkipping` lines.

**Do not change the guard in this task.** With Task 4 in place `LUET_BACKEND=img` resolves to buildah, so the guard string no longer matches the backend actually in use, and the test would start running unintentionally. Update all four to `[ "$LUET_BACKEND" == "buildah" ] && startSkipping` to preserve current behavior exactly, and record in your report whether an unguarded run looks feasible. Enabling it is a follow-up with its own testing, not a side effect of a CI rename.

- [ ] **Step 6: Validate all three workflows parse**

```bash
for f in pr tests push; do
  python3 -c "import yaml; yaml.safe_load(open('.github/workflows/$f.yml')); print('$f ok')"
done
```

Expected: `pr ok`, `tests ok`, `push ok`.

Run: `grep -rn "genuinetools\|LUET_BACKEND=img" .github/workflows/`

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/ tests/integration/30_registry_roundtrip.sh
git commit -m "ci: run the daemonless integration job with buildah

Replaces the pinned genuinetools/img v0.5.11 binary with the distro
buildah package across all three workflows, in both the integration and
unit-test jobs, including the quay login step.

The registry roundtrip test's skip guard is retargeted from img to
buildah so its behavior is unchanged: with the img alias resolving to
buildah, keying on the old string would have silently enabled it."
```

---

### Task 6: Documentation and release notes

**Files:**
- Modify: `docs/content/en/docs/Concepts/Overview/build_packages.md:13,21,25,205`
- Create: `docs/superpowers/specs/2026-07-21-buildah-backend-RELEASE-NOTES.md`

**Interfaces:**
- Consumes: Tasks 4-5 complete.
- Produces: nothing.

- [ ] **Step 1: Update the backend documentation**

`build_packages.md` documents img in four places. Replace each exactly.

Line 13:

> Luet currently supports [Docker](https://www.docker.com/) and [Img](https://github.com/genuinetools/img) as backends to build packages. Both of them can be used and switched in runtime with the ```--backend``` option, so either one of them must be present in the host system.

becomes:

> Luet currently supports [Docker](https://www.docker.com/) and [buildah](https://buildah.io) as backends to build packages. Both of them can be used and switched in runtime with the ```--backend``` option, so either one of them must be present in the host system.

Line 21:

> Luet supports [Img](https://github.com/genuinetools/img). To use it, simply install it in your system, and while running `luet build`, you can switch the backend by providing it as a parameter: `luet build --backend img`. For small packages it is particularly powerful, as it doesn't require any docker daemon running in the host.

becomes:

> Luet supports [buildah](https://buildah.io). To use it, simply install it in your system, and while running `luet build`, you can switch the backend by providing it as a parameter: `luet build --backend buildah`. It doesn't require any docker daemon running in the host, and can build as an unprivileged user. The older `--backend img` is deprecated and now resolves to buildah.

Line 25:

> Luet and img can be used together to orchestrate package builds also on kubernetes. There is available an experimental [Kubernetes CRD for Luet](https://github.com/mudler/luet-k8s) which allows to build packages seamelessly in Kubernetes and push package artifacts to an S3 Compatible object storage (e.g. Minio).

becomes:

> Luet and buildah can be used together to orchestrate package builds also on kubernetes. There is available an experimental [Kubernetes CRD for Luet](https://github.com/mudler/luet-k8s) which allows to build packages seamlessly in Kubernetes and push package artifacts to an S3 Compatible object storage (e.g. Minio). Updating that CRD to request the buildah backend is tracked separately; it continues to work through the deprecated `img` alias in the meantime.

Line 205:

> Luet doesn't handle login to registries, so that has to be handled separately with `docker login` or `img login` before the build process starts.

becomes:

> Luet doesn't handle login to registries, so that has to be handled separately with `docker login` or `buildah login` before the build process starts.

Also add a short subsection covering rootless operation, since this is where users will look:

```markdown
### Rootless builds with buildah

buildah does not need a daemon and can build as an unprivileged user. In a
container or Kubernetes pod, set:

    BUILDAH_ISOLATION=chroot
    STORAGE_DRIVER=vfs

and grant the `SETUID` and `SETGID` capabilities. luet passes its environment
through to the build engine, so no luet-side configuration is required.

Two limitations apply to rootless builds:

- The `vfs` storage driver is considerably slower than `overlay`, particularly
  for large images. Where the host or cluster allows exposing `/dev/fuse`,
  buildah can use `fuse-overlayfs` instead and recover most of the difference.
- `mknod` is blocked for unprivileged users regardless of granted
  capabilities. This is a kernel restriction on user namespaces, not a
  configuration problem. Packages whose build creates device nodes cannot be
  built rootless.
```

- [ ] **Step 2: Write the release note**

Create `docs/superpowers/specs/2026-07-21-buildah-backend-RELEASE-NOTES.md`:

```markdown
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
  contained the queried name as a substring reported as present.

## Rootless limitations

Rootless buildah needs `BUILDAH_ISOLATION=chroot`, `STORAGE_DRIVER=vfs`, and
the `SETUID`/`SETGID` capabilities. Two limitations follow:

- `vfs` is considerably slower than `overlay`, especially for large images.
  Exposing `/dev/fuse` lets buildah use `fuse-overlayfs` instead.
- `mknod` is blocked for unprivileged users regardless of capabilities. This
  is a kernel restriction. Packages whose build creates device nodes cannot be
  built rootless.

## Kubernetes

[luet-k8s](https://github.com/mudler/luet-k8s) still requests the `img`
backend, which continues to work through the deprecation alias. Updating it to
buildah is tracked separately.
```

- [ ] **Step 3: Verify no stale img references remain in user-facing docs**

Run: `grep -rn "genuinetools\|img login\|backend img" docs/content --include="*.md"`

Expected: no output, or only occurrences that deliberately describe the deprecated alias.

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs: document the buildah backend and rootless limitations"
```

---

## Completion checklist

Confirm each was run and observed, not assumed:

- [ ] Task 1's probe actually passed — `crane.Load` accepted buildah's docker-archive
- [ ] `go build ./...` clean
- [ ] `sudo -E env "PATH=$PATH" go test ./pkg/compiler/...` all ok, with the buildah specs running rather than skipping
- [ ] `grep -rn "SimpleImg\|genuinetools" --include="*.go" .` returns nothing
- [ ] `grep -rn "LUET_BACKEND=img" .github/workflows/` returns nothing
- [ ] `--backend img` still accepted and warns
- [ ] `pkg/compiler/backend/simpledocker.go` untouched
- [ ] `mudler/luet-k8s` untouched (Spec C)
