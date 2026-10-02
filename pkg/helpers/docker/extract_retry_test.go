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

package docker

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mudler/luet/pkg/api/core/context"
)

// A registry that serves an empty blob for an image that is not empty reports
// no error anywhere in the pull: the only sign is that nothing was unpacked.
// These tests cover what extractWithRetry does with that, with no registry.
func TestExtractWithRetry(t *testing.T) {
	// The real backoff is seconds long and the delay adds nothing here.
	original := emptyImageBackoff
	emptyImageBackoff = 0
	defer func() { emptyImageBackoff = original }()

	ctx := context.NewContext()

	t.Run("returns the unpacked size without a second pull", func(t *testing.T) {
		calls := 0
		size, err := extractWithRetry(ctx, "repo:repository.yaml", "/dest", func() (int64, error) {
			calls++
			return 42, nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if size != 42 {
			t.Errorf("size = %d, want 42", size)
		}
		if calls != 1 {
			t.Errorf("pulled %d times, want 1", calls)
		}
	})

	t.Run("pulls again when the image unpacked to nothing", func(t *testing.T) {
		calls := 0
		size, err := extractWithRetry(ctx, "repo:repository.yaml", "/dest", func() (int64, error) {
			calls++
			if calls < emptyImageAttempts {
				return 0, nil
			}
			return 7, nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if size != 7 {
			t.Errorf("size = %d, want 7", size)
		}
		if calls != emptyImageAttempts {
			t.Errorf("pulled %d times, want %d", calls, emptyImageAttempts)
		}
	})

	t.Run("fails with ErrEmptyImage when every pull unpacked nothing", func(t *testing.T) {
		calls := 0
		size, err := extractWithRetry(ctx, "quay.io/kairos/packages:repository.yaml", "/tmp/tree", func() (int64, error) {
			calls++
			return 0, nil
		})

		if err == nil {
			t.Fatal("an image that never unpacked anything must not be reported as a successful pull")
		}
		if !errors.Is(err, ErrEmptyImage) {
			t.Errorf("error %q does not match ErrEmptyImage", err)
		}
		// The message has to name the image: the caller only sees that its
		// packages are missing.
		if !strings.Contains(err.Error(), "quay.io/kairos/packages:repository.yaml") {
			t.Errorf("error %q does not name the image", err)
		}
		if !strings.Contains(err.Error(), "/tmp/tree") {
			t.Errorf("error %q does not name the destination", err)
		}
		if size != 0 {
			t.Errorf("size = %d, want 0", size)
		}
		if calls != emptyImageAttempts {
			t.Errorf("pulled %d times, want %d", calls, emptyImageAttempts)
		}
	})

	t.Run("returns an extraction error on the first pull", func(t *testing.T) {
		boom := errors.New("layer is corrupt")
		calls := 0
		_, err := extractWithRetry(ctx, "repo:repository.yaml", "/dest", func() (int64, error) {
			calls++
			return 0, boom
		})

		if !errors.Is(err, boom) {
			t.Errorf("error = %v, want %v", err, boom)
		}
		// An unpack that failed is not an empty image, so retrying it here
		// would only multiply the same failure.
		if calls != 1 {
			t.Errorf("pulled %d times, want 1", calls)
		}
	})
}

// extractStream has to bring back the error go-containerregistry reports
// after the end-of-archive marker. These streams are built the way
// mutate.Extract builds them, so no registry is needed.
func TestExtractStream(t *testing.T) {
	ctx := context.NewContext()

	// ggcr closes the tar writer before it closes the pipe with the error, so
	// the end-of-archive marker arrives first and the unpack stops reading.
	endOfArchive := func(err error) io.ReadCloser {
		pr, pw := io.Pipe()
		go func() {
			tw := tar.NewWriter(pw)
			tw.Close()
			pw.CloseWithError(err)
		}()
		return pr
	}

	t.Run("reports the failure hidden behind the end of archive", func(t *testing.T) {
		dest := t.TempDir()
		forbidden := errors.New("reading layer contents: GET https://blobs.example/data: Forbidden")

		size, err := extractStream(ctx, endOfArchive(forbidden), dest)

		if err == nil {
			t.Fatal("a layer that could not be fetched must not be reported as a successful unpack")
		}
		if !strings.Contains(err.Error(), "Forbidden") {
			t.Errorf("error %q does not carry the download failure", err)
		}
		if size != 0 {
			t.Errorf("size = %d, want 0", size)
		}
	})

	t.Run("reports an image that is genuinely empty as empty", func(t *testing.T) {
		dest := t.TempDir()

		size, err := extractStream(ctx, endOfArchive(nil), dest)

		// Nothing failed, so there is nothing to report here: the caller
		// decides what an empty image means.
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if size != 0 {
			t.Errorf("size = %d, want 0", size)
		}
	})

	t.Run("unpacks a stream that carries files", func(t *testing.T) {
		dest := t.TempDir()

		pr, pw := io.Pipe()
		go func() {
			tw := tar.NewWriter(pw)
			content := []byte("Test Repo\n")
			tw.WriteHeader(&tar.Header{
				Name:     "repository.yaml",
				Mode:     0644,
				Size:     int64(len(content)),
				Typeflag: tar.TypeReg,
				// The unpack restores ownership, which an unprivileged
				// test run can only do for itself.
				Uid: os.Getuid(),
				Gid: os.Getgid(),
			})
			tw.Write(content)
			tw.Close()
			pw.Close()
		}()

		size, err := extractStream(ctx, pr, dest)

		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if size != 10 {
			t.Errorf("size = %d, want 10", size)
		}
		unpacked, err := os.ReadFile(filepath.Join(dest, "repository.yaml"))
		if err != nil {
			t.Fatalf("the file was not unpacked: %s", err)
		}
		if string(unpacked) != "Test Repo\n" {
			t.Errorf("unpacked %q, want %q", unpacked, "Test Repo\n")
		}
	})
}
