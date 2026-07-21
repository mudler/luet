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
			// RemoveImage lands in a later task; clean up directly for now.
			defer exec.Command("buildah", "rmi", opts.ImageName).Run()

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
