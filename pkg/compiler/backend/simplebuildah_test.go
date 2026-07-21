package backend_test

import (
	"archive/tar"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/compiler/backend"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"

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
			// RemoveImage lands in a later task; clean up directly for now.
			defer exec.Command("buildah", "rmi", opts.ImageName).Run()

			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())
			Expect(fileExists(opts.Destination)).To(BeTrue())

			// fileExists is true for a zero-byte file: prove the exported
			// archive is actually a loadable image, not just a path.
			exported, err := crane.Load(opts.Destination)
			Expect(err).ToNot(HaveOccurred())
			exportedLayers, err := exported.Layers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(exportedLayers)).To(BeNumerically(">", 0))

			// ExportImage must truncate an already-populated destination the
			// way docker save -o does, rather than failing with
			// "docker-archive doesn't support modifying existing images".
			Expect(b.ExportImage(opts)).ToNot(HaveOccurred())

			img, err := b.ImageReference(opts.ImageName, true)
			Expect(err).ToNot(HaveOccurred())
			layers, err := img.Layers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(layers)).To(BeNumerically(">", 0))

			// A layer count alone cannot tell the built image apart from its
			// base: assert on the file the Dockerfile's RUN created.
			Expect(flattenedContains(img, "/hello.txt")).To(BeTrue())
		})
	})
})

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// flattenedContains reports whether path is present in the flattened
// filesystem of img. Tar entry names carry no leading slash and may be
// "./"-prefixed, so both ends are normalized before comparing.
func flattenedContains(img v1.Image, path string) bool {
	rc := mutate.Extract(img)
	defer rc.Close()

	want := normalizeTarPath(path)
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false
		}
		if err != nil {
			return false
		}
		if normalizeTarPath(hdr.Name) == want {
			return true
		}
	}
}

func normalizeTarPath(p string) string {
	return strings.TrimPrefix(strings.TrimPrefix(p, "./"), "/")
}
