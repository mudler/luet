# Cross-Arch Group A (Platform Type + Platform Bug Fixes) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a canonical `Platform` type and fix three latent platform bugs that produce silently wrong results on non-amd64 hosts — without changing any on-disk or on-registry format.

**Architecture:** A new `types.Platform` value type (OS/Arch/Variant) with an OCI string form (`linux/arm/v7`) becomes the single representation of a platform. It is threaded through `image.CreateTar`, `PackageArtifact.GenerateFinalImage`, and the two `pkg/helpers/docker` extract helpers, replacing hardcoded `runtime.GOARCH`/`runtime.GOOS` and untyped `platform string` parameters.

**Tech Stack:** Go 1.25, go-containerregistry v0.20.6, Ginkgo v2 + Gomega, cobra.

## Global Constraints

- Module path is `github.com/mudler/luet`.
- Go 1.25.0 (`go.mod:1`).
- **No new dependencies.** `mutate`, `empty`, `remote`, `name`, `tarball` from `github.com/google/go-containerregistry` v0.20.6 are already direct dependencies.
- **`Package.GetFingerPrint()` must not change.** No field may be added to `types.Package`. `pkg/solver`, `pkg/database`, and `pkg/tree` must have zero diff. Treat any diff in those packages as a defect.
- **No on-disk or on-registry format changes in Group A.** Artifact filenames, image tags, and repository file names stay exactly as they are today. Group A is bug fixes plus the type that Groups B and C build on.
- OCI platform string form is `os/arch[/variant]`, e.g. `linux/amd64`, `linux/arm/v7`.
- The filename-safe sanitized form (`linux-arm-v7`) is **out of scope for Group A** — it is added in Group B, where the first caller appears. Do not add it here.
- Tests are Ginkgo v2 + Gomega. Run with `go run github.com/onsi/ginkgo/v2/ginkgo`.
- Existing licence header (GPL-2.0-or-later, `Copyright © <year> Ettore Di Giacinto <mudler@mocaccino.org>`) must be copied to every new file, matching neighbouring files.
- Branch: `cross-arch-group-a`, off `master`. **Group A ships as its own standalone PR** and is mergeable independently. Groups B and C stack separately.

---

## File Structure

| File | Responsibility |
|---|---|
| `pkg/api/core/types/platform.go` (create) | The `Platform` type: parse, render, host detection. No ggcr import — keeps `types` dependency-light. |
| `pkg/api/core/types/platform_test.go` (create) | Unit tests for parse/render round-trips. Joins the existing "Types Suite". |
| `pkg/api/core/image/create.go` (modify) | `CreateTar`/`imageFromTar` take a `types.Platform` and set `Variant` on the image config. |
| `pkg/api/core/types/artifact/artifact.go` (modify) | `PackageArtifact` gains a `Platform` field; `GenerateFinalImage` uses it instead of `runtime.GOARCH`/`runtime.GOOS`. |
| `pkg/helpers/docker/docker.go` (modify) | Both extract helpers take `types.Platform`; fixes swallowed errors in `ExtractDockerImage`. |
| `pkg/helpers/docker/docker_suite_test.go` (create) | Ginkgo suite bootstrap — this package currently has no tests at all. |
| `pkg/helpers/docker/docker_test.go` (create) | Proves platform selection is actually honoured. |
| `pkg/installer/client/docker.go` (modify) | Passes the host platform instead of `""`. |
| `cmd/util.go` (modify) | Call-site updates for the changed signatures. |

`Platform` lives in `pkg/api/core/types` because every consumer can already import it without a cycle: `pkg/api/core/image` imports `types` today (`extract.go:29`), `types/artifact` imports both, `pkg/helpers/docker` imports `types` for `types.Context`, and `types` itself imports none of them.

---

## Background: the three bugs

Read this before starting. Each task fixes one; none is theoretical.

**Bug 1 — final image configs always claim the builder's arch.**
`pkg/api/core/types/artifact/artifact.go:228` hardcodes `runtime.GOARCH, runtime.GOOS`. Every image luet generates therefore describes the machine that built it. When Group C assembles OCI indexes, every child manifest would declare the same platform and the index would silently resolve every platform to the same child. Invisible on a same-arch host.

**Bug 2 — `ExtractDockerImage` swallows registry errors and can nil-panic.**
In `pkg/helpers/docker/docker.go:146`, the `else` branch does `ref, err := name.ParseReference(local)`, which declares a *new* `err` shadowing the outer one. Every subsequent `img, err = remote.Image(...)` assigns to that inner `err`. The outer `if err != nil` check at the end therefore always sees `nil` in this branch, so a failed `remote.Image` yields `img == nil` and the following `img.Manifest()` dereferences nil.

**Bug 3 — foreign-arch hosts silently receive amd64 content.**
`pkg/installer/client/docker.go:106` and `:177` pass `platform: ""`. `DownloadAndExtractDockerImage` then applies no `remote.WithPlatform`, and go-containerregistry falls back to its package default:

```go
// go-containerregistry@v0.20.6 pkg/v1/remote/options.go:59
var defaultPlatform = v1.Platform{
	Architecture: "amd64",
	OS:           "linux",
}
```

So on an arm64 host, pulling a multi-arch docker-type repository hands you the amd64 child.

---

### Task 1: The `Platform` type

**Files:**
- Create: `pkg/api/core/types/platform.go`
- Test: `pkg/api/core/types/platform_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Platform struct { OS, Arch, Variant string }`
  - `func ParsePlatform(s string) (Platform, error)`
  - `func HostPlatform() Platform`
  - `func (p Platform) String() string`
  - `func (p Platform) IsZero() bool`

Design notes for the implementer:

- `ParsePlatform` **errors** on empty and on 1- or 4+-component input. There is no "empty means host" magic — callers that want that behaviour ask for `HostPlatform()` explicitly. Ambiguous defaults are what caused Bug 3.
- The zero `Platform` is a meaningful value meaning *unspecified*; `IsZero()` reports it. Downstream code uses it to mean "don't constrain the registry request", preserving today's behaviour on paths we are not changing.
- `String()` on a zero `Platform` returns `""` so it round-trips with "unspecified".
- No `github.com/google/go-containerregistry` import here. Callers needing a `v1.Platform` do `v1.ParsePlatform(p.String())`. Keeping `types` free of ggcr avoids coupling the most widely imported package in the repo to a container library.

- [ ] **Step 1: Write the failing test**

Create `pkg/api/core/types/platform_test.go`:

```go
// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types_test

import (
	"runtime"

	. "github.com/mudler/luet/pkg/api/core/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Platform", func() {
	Context("ParsePlatform", func() {
		It("parses os/arch", func() {
			p, err := ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.OS).To(Equal("linux"))
			Expect(p.Arch).To(Equal("amd64"))
			Expect(p.Variant).To(Equal(""))
		})

		It("parses os/arch/variant", func() {
			p, err := ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.OS).To(Equal("linux"))
			Expect(p.Arch).To(Equal("arm"))
			Expect(p.Variant).To(Equal("v7"))
		})

		It("rejects the empty string", func() {
			_, err := ParsePlatform("")
			Expect(err).To(HaveOccurred())
		})

		It("rejects a single component", func() {
			_, err := ParsePlatform("amd64")
			Expect(err).To(HaveOccurred())
		})

		It("rejects too many components", func() {
			_, err := ParsePlatform("linux/arm/v7/extra")
			Expect(err).To(HaveOccurred())
		})

		It("rejects empty components", func() {
			_, err := ParsePlatform("linux//v7")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("String", func() {
		It("round-trips os/arch", func() {
			p, err := ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.String()).To(Equal("linux/amd64"))
		})

		It("round-trips os/arch/variant", func() {
			p, err := ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.String()).To(Equal("linux/arm/v7"))
		})

		It("renders the zero value as the empty string", func() {
			Expect(Platform{}.String()).To(Equal(""))
		})
	})


	Context("IsZero", func() {
		It("is true for the zero value", func() {
			Expect(Platform{}.IsZero()).To(BeTrue())
		})

		It("is false for a parsed platform", func() {
			p, err := ParsePlatform("linux/amd64")
			Expect(err).ToNot(HaveOccurred())
			Expect(p.IsZero()).To(BeFalse())
		})
	})

	Context("HostPlatform", func() {
		It("reports the running host", func() {
			p := HostPlatform()
			Expect(p.OS).To(Equal(runtime.GOOS))
			Expect(p.Arch).To(Equal(runtime.GOARCH))
			Expect(p.IsZero()).To(BeFalse())
		})
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="Platform" ./pkg/api/core/types/`
Expected: FAIL to compile — `undefined: ParsePlatform`, `undefined: Platform`, `undefined: HostPlatform`.

- [ ] **Step 3: Write minimal implementation**

Create `pkg/api/core/types/platform.go`:

```go
// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package types

import (
	"fmt"
	"runtime"
	"strings"
)

// Platform identifies a target OS/architecture pair, optionally refined by a
// CPU variant (for example linux/arm/v7).
//
// String() produces the OCI form ("linux/arm/v7"). Use it in YAML, CLI flags,
// image configuration and anything handed to OCI tooling.
//
// The zero Platform means "unspecified" and is reported by IsZero.
type Platform struct {
	OS      string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch    string `json:"arch,omitempty" yaml:"arch,omitempty"`
	Variant string `json:"variant,omitempty" yaml:"variant,omitempty"`
}

// ParsePlatform parses an OCI platform string of the form os/arch or
// os/arch/variant.
//
// It deliberately rejects the empty string rather than defaulting to the host:
// callers that want the host must say so with HostPlatform. An implicit
// default is how luet ended up silently serving amd64 content to arm64 hosts.
func ParsePlatform(s string) (Platform, error) {
	if s == "" {
		return Platform{}, fmt.Errorf("empty platform: expected os/arch or os/arch/variant")
	}

	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return Platform{}, fmt.Errorf("invalid platform %q: expected os/arch or os/arch/variant", s)
	}
	for _, p := range parts {
		if p == "" {
			return Platform{}, fmt.Errorf("invalid platform %q: empty component", s)
		}
	}

	p := Platform{OS: parts[0], Arch: parts[1]}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

// HostPlatform returns the platform luet is currently running on.
//
// Variant is left empty: the Go runtime does not expose the ARM variant it was
// built for, and guessing it would be worse than omitting it.
func HostPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// String returns the OCI form, or "" for the zero Platform.
func (p Platform) String() string {
	if p.IsZero() {
		return ""
	}
	if p.Variant != "" {
		return fmt.Sprintf("%s/%s/%s", p.OS, p.Arch, p.Variant)
	}
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}


// IsZero reports whether the platform is unspecified.
func (p Platform) IsZero() bool {
	return p.OS == "" && p.Arch == "" && p.Variant == ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="Platform" ./pkg/api/core/types/`
Expected: PASS, 17 specs.

- [ ] **Step 5: Verify the forbidden packages are untouched**

Run: `git status --porcelain pkg/solver pkg/database pkg/tree`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add pkg/api/core/types/platform.go pkg/api/core/types/platform_test.go
git commit -m "feat(types): add Platform type for OS, arch and variant"
```

---

### Task 2: `CreateTar` takes a `Platform` and sets `Variant`

`imageFromTar` sets `cfg.Architecture` and `cfg.OS` but never `cfg.Variant`, so an
image built for `linux/arm/v7` is indistinguishable from one built for
`linux/arm/v6`. Switching to `Platform` fixes that and is the prerequisite for
Task 3.

**Files:**
- Modify: `pkg/api/core/image/create.go:31,48-49,69-70,78`
- Modify: `pkg/api/core/types/artifact/artifact.go:228` (call-site only; behaviour change lands in Task 3)
- Modify: `cmd/util.go:42,50,78` (call-site only)
- Test: `pkg/api/core/image/create_test.go`

**Interfaces:**
- Consumes: `types.Platform`, `types.ParsePlatform`, `types.HostPlatform` from Task 1.
- Produces:
  - `func CreateTar(srctar, dstimageTar, imagename string, platform types.Platform) error`
  - `func imageFromTar(imagename string, platform types.Platform, opener func() (io.ReadCloser, error)) (name.Reference, v1.Image, error)` (unexported)

- [ ] **Step 1: Write the failing test**

Append to `pkg/api/core/image/create_test.go`, inside the existing top-level
`Describe("Create", ...)` block, after the existing `Context`:

```go
	Context("Records the target platform in the image config", func() {
		It("sets os, architecture and variant", func() {
			ctx := context.NewContext()

			dst, err := ctx.TempFile("dst")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dst.Name())
			srcTar, err := ctx.TempFile("srcTar")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(srcTar.Name())

			dir, err := os.MkdirTemp("", "platform")
			Expect(err).ToNot(HaveOccurred())
			defer os.RemoveAll(dir)
			Expect(file.Touch(filepath.Join(dir, "test"))).ToNot(HaveOccurred())

			a := artifact.NewPackageArtifact(srcTar.Name())
			Expect(a.Compress(dir, 1)).ToNot(HaveOccurred())

			platform, err := types.ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())

			Expect(CreateTar(srcTar.Name(), dst.Name(), "testimage:v7", platform)).ToNot(HaveOccurred())

			// Read the written tarball back without involving the daemon.
			img, err := tarball.ImageFromPath(dst.Name(), nil)
			Expect(err).ToNot(HaveOccurred())

			cfg, err := img.ConfigFile()
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.OS).To(Equal("linux"))
			Expect(cfg.Architecture).To(Equal("arm"))
			Expect(cfg.Variant).To(Equal("v7"))
		})
	})
```

Add these imports to `pkg/api/core/image/create_test.go`:

```go
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
```

This test needs no docker daemon: it writes a tarball and reads it back with
go-containerregistry. It is the regression guard for Bug 1 and is meaningful on
an amd64 host, which the hardcoded-`runtime.GOARCH` version could never be.

- [ ] **Step 2: Run test to verify it fails**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="Records the target platform" ./pkg/api/core/image/`
Expected: FAIL to compile — `too many arguments in call to CreateTar` / cannot use `platform` (type `types.Platform`) as `string`.

- [ ] **Step 3: Change the signature in `pkg/api/core/image/create.go`**

Replace the `imageFromTar` signature and config assignment:

```go
func imageFromTar(imagename string, platform types.Platform, opener func() (io.ReadCloser, error)) (name.Reference, v1.Image, error) {
```

Replace lines 48-49:

```go
	cfg.Architecture = platform.Arch
	cfg.OS = platform.OS
	cfg.Variant = platform.Variant
```

Replace the `CreateTar` signature and its inner call:

```go
// CreateTar a imagetarball from a standard tarball
func CreateTar(srctar, dstimageTar, imagename string, platform types.Platform) error {
```

```go
	newRef, img, err := imageFromTar(imagename, platform, func() (io.ReadCloser, error) {
```

Add the import:

```go
	"github.com/mudler/luet/pkg/api/core/types"
```

- [ ] **Step 4: Update the three call sites**

`pkg/api/core/types/artifact/artifact.go:228` — mechanical for now; Task 3 replaces `types.HostPlatform()` with the artifact's own platform:

```go
	if err := image.CreateTar(a.Path, tempimage.Name(), imageName, types.HostPlatform()); err != nil {
```

`cmd/util.go` — change `pack` to take a `types.Platform`:

```go
func pack(ctx *context.Context, p, dst, imageName string, platform types.Platform) error {

	tempimage, err := ctx.TempFile("tempimage")
	if err != nil {
		return errors.Wrap(err, "error met while creating tempdir for "+p)
	}
	defer os.RemoveAll(tempimage.Name()) // clean up

	if err := image.CreateTar(p, tempimage.Name(), imageName, platform); err != nil {
		return errors.Wrap(err, "could not create image from tar")
	}

	return fileHelper.CopyFile(tempimage.Name(), dst)
}
```

and its caller in `NewPackCommand`, replacing the `arch`/`os` flag read:

```go
			arch, _ := cmd.Flags().GetString("arch")
			osFlag, _ := cmd.Flags().GetString("os")

			platform, err := types.ParsePlatform(osFlag + "/" + arch)
			if err != nil {
				util.DefaultContext.Fatal(err.Error())
			}

			err = pack(util.DefaultContext, src, dst, image, platform)
			if err != nil {
				util.DefaultContext.Fatal(err.Error())
			}
```

Note the local variable is renamed from `os` to `osFlag` — the original shadowed the
`os` package, which is imported in this file.

Add to `cmd/util.go` imports:

```go
	"github.com/mudler/luet/pkg/api/core/types"
```

`pkg/api/core/image/create_test.go:62` — the existing test:

```go
			err = CreateTar(srcTar.Name(), dst.Name(), "testimage", types.HostPlatform())
```

Then remove the now-unused `"runtime"` import from `create_test.go` if nothing else uses it.

- [ ] **Step 5: Run the full image and types suites**

Run: `go build ./... && go run github.com/onsi/ginkgo/v2/ginkgo ./pkg/api/core/image/ ./pkg/api/core/types/`
Expected: PASS. The new platform spec passes; the pre-existing "creates an image which is loadable" spec still passes.

- [ ] **Step 6: Verify the CLI still works**

Run:
```bash
go run . util pack --os linux --arch arm64 testimage:arm64 go.mod /tmp/packed-arm64.tar 2>&1 | tail -2
```
Expected: `Image packed as testimage:arm64` (the source need not be a real tarball for the signature check; if `pack` errors on the input format, that is acceptable — what matters is that the flags parse and no panic occurs).

- [ ] **Step 7: Commit**

```bash
git add pkg/api/core/image/create.go pkg/api/core/image/create_test.go pkg/api/core/types/artifact/artifact.go cmd/util.go
git commit -m "fix(image): carry OS, arch and variant into generated image configs

CreateTar took bare architecture and OS strings and never set the config
Variant, so linux/arm/v7 and linux/arm/v6 images were indistinguishable.
Take a types.Platform instead and set all three fields."
```

---

### Task 3: `PackageArtifact` carries its platform

This is the structural half of Bug 1. Today `GenerateFinalImage` stamps the
*builder's* arch onto every image it produces. After this task the artifact
carries its own platform and the image reflects it. Group B populates the field
from `luet build --platform`; until then it is set to the host, so **behaviour is
unchanged** — this task exists so that Group B is a one-line change rather than
a re-plumbing.

**Files:**
- Modify: `pkg/api/core/types/artifact/artifact.go:54-65` (struct), `:220-232` (`GenerateFinalImage`)
- Test: `pkg/api/core/types/artifact/artifact_test.go`

**Interfaces:**
- Consumes: `types.Platform`, `types.HostPlatform` from Task 1; `image.CreateTar` from Task 2.
- Produces:
  - `PackageArtifact.Platform types.Platform` — JSON key `platform`, `omitzero`
  - `func (a *PackageArtifact) TargetPlatform() types.Platform` — returns `a.Platform`, or `types.HostPlatform()` when zero

**Use `omitzero`, not `omitempty`.** This is load-bearing for the "no format
changes" constraint and is easy to get wrong:

- `encoding/json`'s `omitempty` has **no effect on struct-typed fields**. Tagging
  the field `omitempty` emits `"platform":{}` on every artifact — a format change
  to every existing repository index, which this group forbids.
- `omitzero` (Go 1.24+, and this module is on Go 1.25) does omit it. When the
  type has an `IsZero() bool` method, `omitzero` uses it — `Platform` has one
  from Task 1, so the two fit together directly.

**Both tags are load-bearing, on different paths.** `PackageArtifact` is written
to `.metadata.yaml` with `gopkg.in/yaml.v3` (`artifact.go:47`), which honours the
`yaml:` tag — and unlike `encoding/json`, yaml.v3's `omitempty` *does* omit zero
structs, so `yaml:"platform,omitempty"` is correct there. Separately, the
repository index is written with `github.com/ghodss/yaml`
(`pkg/installer/repository.go:41`), which routes through `encoding/json` and so
honours the `json:` tag — that is where `omitzero` is required. Verified
empirically for all three marshallers.

The on-the-wire shape of a *populated* platform (currently a nested
`os`/`arch`/`variant` map) is deliberately left undecided here. No Group A
artifact ever has a non-zero platform, so nothing is written and no format is
committed to. Group B, which first populates the field, may add
`MarshalJSON`/`UnmarshalJSON` to render it as the compact `linux/arm/v7` string
without breaking anything.

- [ ] **Step 1: Write the failing test**

Append to `pkg/api/core/types/artifact/artifact_test.go`, inside the existing
top-level `Describe` block:

```go
	Context("TargetPlatform", func() {
		It("falls back to the host when unset", func() {
			a := NewPackageArtifact("/tmp/foo.tar")
			Expect(a.TargetPlatform()).To(Equal(types.HostPlatform()))
		})

		It("returns the artifact platform when set", func() {
			p, err := types.ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())

			a := NewPackageArtifact("/tmp/foo.tar")
			a.Platform = p
			Expect(a.TargetPlatform()).To(Equal(p))
		})

		It("is omitted from JSON when unset", func() {
			a := NewPackageArtifact("/tmp/foo.tar")
			b, err := json.Marshal(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(b)).ToNot(ContainSubstring("platform"))
		})

		It("round-trips through JSON when set", func() {
			p, err := types.ParsePlatform("linux/arm/v7")
			Expect(err).ToNot(HaveOccurred())

			a := NewPackageArtifact("/tmp/foo.tar")
			a.Platform = p

			b, err := json.Marshal(a)
			Expect(err).ToNot(HaveOccurred())

			var decoded PackageArtifact
			Expect(json.Unmarshal(b, &decoded)).ToNot(HaveOccurred())
			Expect(decoded.Platform).To(Equal(p))
		})
	})
```

Ensure `artifact_test.go` imports `"encoding/json"` and
`"github.com/mudler/luet/pkg/api/core/types"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="TargetPlatform" ./pkg/api/core/types/artifact/`
Expected: FAIL to compile — `a.Platform undefined`, `a.TargetPlatform undefined`.

- [ ] **Step 3: Add the field and accessor**

In `pkg/api/core/types/artifact/artifact.go`, add to the `PackageArtifact` struct
after `Runtime`:

```go
	// Platform is the target platform this artifact was built for.
	// The zero value means unspecified, in which case the host platform is
	// assumed.
	//
	// omitzero, not omitempty: encoding/json's omitempty does not omit
	// struct-typed fields, so omitempty here would add "platform":{} to every
	// artifact ever serialized. omitzero uses Platform's IsZero method.
	Platform types.Platform `json:"platform,omitzero" yaml:"platform,omitempty"`
```

Add the accessor next to `GenerateFinalImage`:

```go
// TargetPlatform returns the platform this artifact targets, defaulting to the
// host platform when the artifact does not declare one.
func (a *PackageArtifact) TargetPlatform() types.Platform {
	if a.Platform.IsZero() {
		return types.HostPlatform()
	}
	return a.Platform
}
```

- [ ] **Step 4: Use it in `GenerateFinalImage`**

Replace the `image.CreateTar` call introduced in Task 2:

```go
	if err := image.CreateTar(a.Path, tempimage.Name(), imageName, a.TargetPlatform()); err != nil {
```

Then remove the `"runtime"` import from `artifact.go` if nothing else in the file
uses it.

- [ ] **Step 5: Run test to verify it passes**

Run: `go build ./... && go run github.com/onsi/ginkgo/v2/ginkgo --focus="TargetPlatform" ./pkg/api/core/types/artifact/`
Expected: PASS, 4 specs.

- [ ] **Step 6: Confirm no serialization drift**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo ./pkg/api/core/types/... ./pkg/installer/...`
Expected: PASS. Any failure here means the field is not being omitted and an
existing repository format has changed — that violates a global constraint and
must be fixed, not accepted.

- [ ] **Step 7: Commit**

```bash
git add pkg/api/core/types/artifact/artifact.go pkg/api/core/types/artifact/artifact_test.go
git commit -m "fix(artifact): stamp the artifact's own platform into final images

GenerateFinalImage hardcoded runtime.GOARCH/GOOS, so every generated image
claimed the arch of the machine that built it. Carry the platform on the
artifact instead. Serialized with omitzero, so artifacts that do not declare
a platform round-trip byte-identically."```

---

### Task 4: Stop `ExtractDockerImage` swallowing registry errors

This is Bug 2, and it is a live nil-dereference. It is fixed in its own task
because it is a genuine defect independent of cross-arch work, and a reviewer
should be able to accept or reject it on its own merits.

**Files:**
- Modify: `pkg/helpers/docker/docker.go:146-180`
- Create: `pkg/helpers/docker/docker_suite_test.go`
- Create: `pkg/helpers/docker/docker_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: no signature change. `ExtractDockerImage` keeps
  `func ExtractDockerImage(ctx luettypes.Context, local, dest, platform string) (*images.Image, error)`
  until Task 5 changes it.

The defect, precisely: in the `else` branch, `ref, err := name.ParseReference(local)`
declares a **new** `err` that shadows the function-scope one. Both
`img, err = remote.Image(...)` assignments target a shadowed `err`. The
function-scope `if err != nil` check after the branch therefore always observes
`nil` here, so a failed registry call falls through with `img == nil` straight
into `img.Manifest()`.

- [ ] **Step 1: Write the failing test**

Create `pkg/helpers/docker/docker_suite_test.go`:

```go
// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package docker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDockerHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker Helpers Suite")
}
```

Create `pkg/helpers/docker/docker_test.go`:

```go
// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package docker_test

import (
	"os"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/helpers/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExtractDockerImage", func() {
	It("returns an error instead of panicking when the reference cannot be resolved", func() {
		ctx := context.NewContext()

		dest, err := os.MkdirTemp("", "extract-err")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(dest)

		// This host does not resolve, so remote.Image must fail. Before the
		// shadowing fix the error was dropped and img.Manifest() nil-panicked.
		_, err = ExtractDockerImage(ctx, "luet-nonexistent.invalid/nope:latest", dest, "")
		Expect(err).To(HaveOccurred())
	})
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="ExtractDockerImage" ./pkg/helpers/docker/`
Expected: FAIL with a panic — `runtime error: invalid memory address or nil pointer dereference`, originating at the `img.Manifest()` call in `docker.go`. A panic, not an assertion failure, is the confirmation that the bug is real.

- [ ] **Step 3: Fix the shadowing**

In `pkg/helpers/docker/docker.go`, rewrite the `else` branch of
`ExtractDockerImage` so that every assignment targets the function-scope `err`:

```go
	var err error
	if strings.HasPrefix(local, filePrefix) {
		parts := strings.Split(local, fileImageSeparator)
		if len(parts) == 2 && parts[1] != "" {
			img, err = tarball.ImageFromPath(parts[1], nil)
		}
	} else {
		var ref name.Reference
		ref, err = name.ParseReference(local)
		if err != nil {
			return nil, err
		}

		opts := []remote.Option{}
		if platform != "" {
			var p *v1.Platform
			p, err = v1.ParsePlatform(platform)
			if err != nil {
				return nil, err
			}
			opts = append(opts, remote.WithPlatform(*p))
		}

		img, err = remote.Image(ref, opts...)
	}
	if err != nil {
		return nil, err
	}
```

Two changes carry the fix: `var ref name.Reference` + `ref, err = ...` instead of
`ref, err := ...`, and hoisting the platform option into a slice so the single
`remote.Image` call assigns to the outer `err`. The option-slice shape also
matches `DownloadAndExtractDockerImage`, which already builds `opts` this way.

- [ ] **Step 4: Guard against `img` being nil**

Immediately after the `if err != nil` block, add:

```go
	if img == nil {
		return nil, errors.Errorf("could not resolve image %s", local)
	}
```

This covers the remaining hole: if `local` starts with `file://` but does not
split into exactly two non-empty parts, neither `img` nor `err` is ever assigned
and the function still reaches `img.Manifest()` with a nil image.

- [ ] **Step 5: Run test to verify it passes**

Run: `go build ./... && go run github.com/onsi/ginkgo/v2/ginkgo --focus="ExtractDockerImage" ./pkg/helpers/docker/`
Expected: PASS, 1 spec. No panic.

- [ ] **Step 6: Commit**

```bash
git add pkg/helpers/docker/docker.go pkg/helpers/docker/docker_suite_test.go pkg/helpers/docker/docker_test.go
git commit -m "fix(docker): do not swallow registry errors in ExtractDockerImage

The else branch redeclared err with :=, shadowing the function-scope variable
that the trailing error check reads. A failed remote.Image therefore fell
through with a nil image and panicked in img.Manifest(). Assign to the outer
err and reject a nil image explicitly."
```

---

### Task 5: Pass a real platform from the installer client

This is Bug 3, the user-visible one: on an arm64 host, luet's docker-type
repository client silently receives amd64 content.

**Files:**
- Modify: `pkg/helpers/docker/docker.go:70,92-97,146,168-178`
- Modify: `pkg/installer/client/docker.go:106,177`
- Modify: `cmd/util.go:136,144`
- Test: `pkg/helpers/docker/docker_test.go`

**Interfaces:**
- Consumes: `types.Platform`, `types.ParsePlatform`, `types.HostPlatform` from Task 1; the fixed `else` branch from Task 4.
- Produces:
  - `func DownloadAndExtractDockerImage(ctx luettypes.Context, image, dest string, auth *registrytypes.AuthConfig, verify bool, platform luettypes.Platform) (*images.Image, error)`
  - `func ExtractDockerImage(ctx luettypes.Context, local, dest string, platform luettypes.Platform) (*images.Image, error)`

A zero `Platform` continues to mean "apply no `remote.WithPlatform`", preserving
today's behaviour on the `luet util unpack` path where the flag may be empty.
The installer client stops relying on that default and asks for the host
explicitly — that is the whole fix.

- [ ] **Step 1: Write the failing test**

Append to `pkg/helpers/docker/docker_test.go`:

```go
var _ = Describe("DownloadAndExtractDockerImage platform selection", func() {
	It("resolves different children of a multi-arch image", func() {
		ctx := context.NewContext()

		amdDest, err := os.MkdirTemp("", "plat-amd64")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(amdDest)
		armDest, err := os.MkdirTemp("", "plat-arm64")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(armDest)

		amd64, err := types.ParsePlatform("linux/amd64")
		Expect(err).ToNot(HaveOccurred())
		arm64, err := types.ParsePlatform("linux/arm64")
		Expect(err).ToNot(HaveOccurred())

		amdInfo, err := DownloadAndExtractDockerImage(ctx, "alpine:3.19", amdDest, nil, false, amd64)
		Expect(err).ToNot(HaveOccurred())

		armInfo, err := DownloadAndExtractDockerImage(ctx, "alpine:3.19", armDest, nil, false, arm64)
		Expect(err).ToNot(HaveOccurred())

		// alpine:3.19 is a multi-arch index; the two platforms must resolve to
		// different child manifests. Identical digests mean the platform
		// option was ignored and both requests fell back to one default.
		Expect(amdInfo.Target.Digest).ToNot(Equal(armInfo.Target.Digest))
	})
})
```

Add `"github.com/mudler/luet/pkg/api/core/types"` to the imports of
`docker_test.go`.

This test asserts *selection happened* rather than trying to interpret the
extracted rootfs, so it is meaningful on any host and needs no emulation. It
requires network access to Docker Hub, which the existing suites already assume
(`pkg/api/core/image/mutator_suite_test.go` pulls `alpine` and `golang:alpine`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go run github.com/onsi/ginkgo/v2/ginkgo --focus="platform selection" ./pkg/helpers/docker/`
Expected: FAIL to compile — cannot use `amd64` (type `types.Platform`) as type `string`.

- [ ] **Step 3: Change both helper signatures**

In `pkg/helpers/docker/docker.go`, change `DownloadAndExtractDockerImage`:

```go
func DownloadAndExtractDockerImage(ctx luettypes.Context, image, dest string, auth *registrytypes.AuthConfig, verify bool, platform luettypes.Platform) (*images.Image, error) {
```

and its option block:

```go
	opts := []remote.Option{remote.WithAuth(staticAuth{auth}), remote.WithTransport(http.DefaultTransport)}
	if !platform.IsZero() {
		p, err := v1.ParsePlatform(platform.String())
		if err != nil {
			return nil, err
		}
		opts = append(opts, remote.WithPlatform(*p))
	}
```

Change `ExtractDockerImage`:

```go
func ExtractDockerImage(ctx luettypes.Context, local, dest string, platform luettypes.Platform) (*images.Image, error) {
```

and, inside the `else` branch rewritten in Task 4:

```go
		opts := []remote.Option{}
		if !platform.IsZero() {
			var p *v1.Platform
			p, err = v1.ParsePlatform(platform.String())
			if err != nil {
				return nil, err
			}
			opts = append(opts, remote.WithPlatform(*p))
		}
```

- [ ] **Step 4: Fix the installer client — the actual bug**

In `pkg/installer/client/docker.go`, at **both** line 106 and line 177, replace
the `""` argument:

```go
		info, err := docker.DownloadAndExtractDockerImage(c.context, imageName, temp, c.auth, c.RepoData.Verify, types.HostPlatform())
```

Add the import, matching the alias already used elsewhere in the file if one
exists, otherwise:

```go
	"github.com/mudler/luet/pkg/api/core/types"
```

- [ ] **Step 5: Update the `luet util unpack` call sites**

In `cmd/util.go`, the `--platform` flag is a string that may legitimately be
empty, meaning "unspecified". Parse it only when set:

```go
			platformFlag, _ := cmd.Flags().GetString("platform")

			var platform types.Platform
			if platformFlag != "" {
				platform, err = types.ParsePlatform(platformFlag)
				if err != nil {
					util.DefaultContext.Fatal(err.Error())
				}
			}
```

Declare `var err error` ahead of this block if the surrounding scope does not
already have one, then pass `platform` unchanged to both
`docker.DownloadAndExtractDockerImage` and `docker.ExtractDockerImage`.

- [ ] **Step 6: Update the Task 4 test for the new signature**

Task 4's test passes a bare `""` for the platform, which no longer compiles.
In `pkg/helpers/docker/docker_test.go`, change that call to pass the zero
`Platform` — which still means "unspecified", so the test keeps asserting
exactly what it asserted before:

```go
		_, err = ExtractDockerImage(ctx, "luet-nonexistent.invalid/nope:latest", dest, types.Platform{})
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go build ./... && go run github.com/onsi/ginkgo/v2/ginkgo ./pkg/helpers/docker/`
Expected: PASS, 2 specs.

- [ ] **Step 8: Verify no `""` platform arguments remain**

Run: `grep -rn 'DownloadAndExtractDockerImage\|ExtractDockerImage' --include='*.go' pkg cmd | grep -v _test | grep ', ""'`
Expected: no output.

- [ ] **Step 9: Run the full suite**

Run: `make test`
Expected: PASS. This is the full Ginkgo run with `--flake-attempts=3`; it needs docker and network.

- [ ] **Step 10: Verify the forbidden packages are still untouched**

Run: `git diff --stat $(git merge-base origin/master HEAD) -- pkg/solver pkg/database pkg/tree`
Expected: no output.

- [ ] **Step 11: Commit**

```bash
git add pkg/helpers/docker/docker.go pkg/helpers/docker/docker_test.go pkg/installer/client/docker.go cmd/util.go
git commit -m "fix(installer): request the host platform when pulling repository images

client/docker.go passed an empty platform, so no remote.WithPlatform option was
applied and go-containerregistry fell back to its linux/amd64 default. On an
arm64 host, pulling a multi-arch docker-type repository silently returned amd64
content. Ask for the host platform explicitly and type the parameter as
types.Platform so an empty value can no longer be spelled by accident."
```

---

## Done criteria

- [ ] `make test` passes.
- [ ] `git diff --stat $(git merge-base origin/master HEAD) -- pkg/solver pkg/database pkg/tree` is empty.
      (NOT against local `master`, which may be stale relative to `origin/master` and will report
      inherited upstream solver work as though it were ours.)
- [ ] `types.Package` has no new field and `GetFingerPrint()` is unchanged.
- [ ] No artifact filename, image tag, or repository file name has changed.
- [ ] `grep -rn 'runtime.GOARCH\|runtime.GOOS' --include='*.go' pkg | grep -v _test` returns only `pkg/api/core/types/platform.go` (inside `HostPlatform`) and `pkg/api/core/types/repository.go` (the `Enabled()` filter, deprecated in Group C).
