package imgworker

// FROM Slightly adapted from genuinetools/img worker

import (
	"errors"
	"fmt"
	"github.com/mudler/luet/pkg/bus"
	"os"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/archive"
	"github.com/sirupsen/logrus"
)

// TODO: this requires root permissions to mount/unmount layers, althrought it shouldn't be required.
// See how backends are unpacking images without asking for root permissions.

// UnpackEventData is the data structure to pass for the bus events
type UnpackEventData struct {
	Image string
	Dest  string
}

// Unpack exports an image to a rootfs destination directory.
func (c *Client) Unpack(image, dest string) error {

	ctx := c.ctx
	if len(dest) < 1 {
		return errors.New("destination directory for rootfs cannot be empty")
	}

	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("destination directory already exists: %s", dest)
	}

	// Parse the image name and tag.
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return fmt.Errorf("parsing image name %q failed: %v", image, err)
	}
	// Add the latest lag if they did not provide one.
	named = reference.TagNameOnly(named)
	image = named.String()

	// Create the worker opts.
	opt, err := c.createWorkerOpt()
	if err != nil {
		return fmt.Errorf("creating worker opt failed: %v", err)
	}

	if opt.ImageStore == nil {
		return errors.New("image store is nil")
	}

	img, err := opt.ImageStore.Get(ctx, image)
	if err != nil {
		return fmt.Errorf("getting image %s from image store failed: %v", image, err)
	}

	manifest, err := images.Manifest(ctx, opt.ContentStore, img.Target, platforms.Default())
	if err != nil {
		return fmt.Errorf("getting image manifest failed: %v", err)
	}

	_,_ = bus.Manager.Publish(bus.EventImagePreUnPack, UnpackEventData{Image: image, Dest: dest})

	for _, desc := range manifest.Layers {
		logrus.Debugf("Unpacking layer %s", desc.Digest.String())

		// Read the blob from the content store.
		layer, err := opt.ContentStore.ReaderAt(ctx, desc)
		if err != nil {
			return fmt.Errorf("getting reader for digest %s failed: %v", desc.Digest.String(), err)
		}

		// Unpack the tarfile to the rootfs path.
		// FROM: https://godoc.org/github.com/moby/moby/pkg/archive#TarOptions
		if err := archive.Untar(content.NewReader(layer), dest, &archive.TarOptions{
			NoLchown:        false,
			ExcludePatterns: []string{"dev/"}, // prevent 'operation not permitted'
		}); err != nil {
			return fmt.Errorf("extracting tar for %s to directory %s failed: %v", desc.Digest.String(), dest, err)
		}
	}

	_, _ = bus.Manager.Publish(bus.EventImagePostUnPack, UnpackEventData{Image: image, Dest: dest})

	return nil
}
