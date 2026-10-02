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

package docker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/containerd/containerd/images"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/mudler/luet/pkg/api/core/bus"
	luetimages "github.com/mudler/luet/pkg/api/core/image"
	luettypes "github.com/mudler/luet/pkg/api/core/types"

	fileHelper "github.com/mudler/luet/pkg/helpers/file"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	filePrefix         = "file://"
	fileImageSeparator = ":/"

	// emptyImageAttempts is how many times an image that unpacked to nothing
	// is pulled again before the pull is reported as a failure.
	emptyImageAttempts = 3
)

// emptyImageBackoff is the delay before the second attempt at an image that
// unpacked to nothing. It doubles for every attempt after that. It is a
// variable so that tests do not have to wait for it.
var emptyImageBackoff = 2 * time.Second

// ErrEmptyImage reports an image that unpacked to zero bytes. A luet
// repository file or package image always carries content, so an empty one is
// a failed pull and not an empty package: the destination is left as it was
// and callers must not read it as populated.
var ErrEmptyImage = errors.New("image unpacked to 0 bytes")

type staticAuth struct {
	auth *registrytypes.AuthConfig
}

func (s staticAuth) Authorization() (*authn.AuthConfig, error) {
	// A nil AuthConfig means anonymous. go-containerregistry dereferences the
	// returned config unconditionally (see transport.bearerTransport.refresh),
	// so return an empty config rather than nil, matching authn.Anonymous.
	if s.auth == nil {
		return &authn.AuthConfig{}, nil
	}
	return &authn.AuthConfig{
		Username:      s.auth.Username,
		Password:      s.auth.Password,
		Auth:          s.auth.Auth,
		IdentityToken: s.auth.IdentityToken,
		RegistryToken: s.auth.RegistryToken,
	}, nil
}

// UnpackEventData is the data structure to pass for the bus events
type UnpackEventData struct {
	Image string
	Dest  string
}

// noClose keeps a reader open across an unpack that closes what it is given.
type noClose struct {
	io.Reader
}

func (noClose) Close() error { return nil }

// extractStream unpacks an image extraction stream into dest and returns the
// number of bytes it unpacked.
//
// go-containerregistry reports a layer it could not fetch by closing the
// extraction stream with that error, but it closes the tar writer first, so
// the stream opens with the two zero blocks that mean "end of archive". The
// tar reader inside the unpack stops there and reports a clean end of input,
// and the error is never read: a layer that failed to download arrives as an
// image with no files in it.
//
// So when nothing was unpacked, read what is left of the stream. That is
// where the download failure is, and naming it beats reporting an empty image.
func extractStream(ctx luettypes.Context, reader io.ReadCloser, dest string) (int64, error) {
	defer reader.Close()

	size, _, err := luetimages.ExtractReader(ctx, noClose{reader}, dest, nil)
	if err != nil || size > 0 {
		return size, err
	}

	if _, drainErr := io.Copy(io.Discard, reader); drainErr != nil {
		return 0, drainErr
	}

	return 0, nil
}

// extractWithRetry unpacks an image through extract, which returns the number
// of bytes it unpacked.
//
// A registry that answers a manifest request but serves an empty blob for the
// layers leaves luet with a destination that holds nothing. Nothing in the
// pull reports this: the manifest, the media type and the digest are all read
// without error, so the only sign is the unpacked size. The caller then reads
// an empty repository index and reports the packages as missing, which is a
// confusing way to describe a failed download.
//
// Every attempt fetches the layers again, so an intermittently empty blob is
// worth another pull. When no attempt unpacks anything the pull fails with
// ErrEmptyImage, naming the image, so the failure is read as what it is.
func extractWithRetry(ctx luettypes.Context, image, dest string, extract func() (int64, error)) (int64, error) {
	backoff := emptyImageBackoff

	for attempt := 1; ; attempt++ {
		size, err := extract()
		if err != nil {
			return 0, err
		}
		if size > 0 {
			return size, nil
		}

		if attempt >= emptyImageAttempts {
			return 0, fmt.Errorf("%w: %s unpacked no files into %s in %d attempts, the registry is serving it empty",
				ErrEmptyImage, image, dest, attempt)
		}

		ctx.Warning(fmt.Sprintf("%s unpacked no files into %s on attempt %d of %d, pulling it again",
			image, dest, attempt, emptyImageAttempts))
		time.Sleep(backoff)
		backoff *= 2
	}
}

// DownloadAndExtractDockerImage extracts a container image natively. It supports privileged/unprivileged mode
func DownloadAndExtractDockerImage(ctx luettypes.Context, image, dest string, auth *registrytypes.AuthConfig, verify bool, platform luettypes.Platform) (*images.Image, error) {
	if verify {
		// Docker Content Trust was removed from docker/cli in v29, and the
		// notary project behind it is no longer maintained. The flag and the
		// repository `verify:` field are still accepted so existing
		// configurations keep working, but no verification is performed.
		ctx.Warning("image verification is no longer supported and was skipped: " +
			"Docker Content Trust was removed upstream. Pin images by digest instead.")
	}

	if !fileHelper.Exists(dest) {
		if err := os.MkdirAll(dest, os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "cannot create destination directory")
		}
	}

	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, err
	}

	opts := []remote.Option{remote.WithAuth(staticAuth{auth}), remote.WithTransport(http.DefaultTransport)}
	if !platform.IsZero() {
		p, err := v1.ParsePlatform(platform.String())
		if err != nil {
			return nil, err
		}
		opts = append(opts, remote.WithPlatform(*p))
	}

	img, err := remote.Image(ref, opts...)
	if err != nil {
		return nil, err
	}

	m, err := img.Manifest()
	if err != nil {
		return nil, err
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, err
	}

	d, err := img.Digest()
	if err != nil {
		return nil, err
	}

	bus.Manager.Publish(bus.EventImagePreUnPack, UnpackEventData{Image: image, Dest: dest})

	c, err := extractWithRetry(ctx, image, dest, func() (int64, error) {
		return extractStream(ctx, mutate.Extract(img), dest)
	})
	if err != nil {
		return nil, err
	}

	bus.Manager.Publish(bus.EventImagePostUnPack, UnpackEventData{Image: image, Dest: dest})

	return &images.Image{
		Name:   image,
		Labels: m.Annotations,
		Target: specs.Descriptor{
			MediaType: string(mt),
			Digest:    digest.Digest(d.String()),
			Size:      c,
		},
	}, nil
}

func ExtractDockerImage(ctx luettypes.Context, local, dest string, platform luettypes.Platform) (*images.Image, error) {
	var img v1.Image
	if !fileHelper.Exists(dest) {
		if err := os.MkdirAll(dest, os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "cannot create destination directory")
		}
	}

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
		if !platform.IsZero() {
			var p *v1.Platform
			p, err = v1.ParsePlatform(platform.String())
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

	if img == nil {
		return nil, errors.Errorf("could not resolve image %s", local)
	}

	m, err := img.Manifest()
	if err != nil {
		return nil, err
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, err
	}

	d, err := img.Digest()
	if err != nil {
		return nil, err
	}

	var c int64
	c, _, err = luetimages.ExtractTo(
		ctx,
		img,
		dest,
		nil,
	)

	if err != nil {
		return nil, err
	}

	bus.Manager.Publish(bus.EventImagePostUnPack, UnpackEventData{Image: local, Dest: dest})

	return &images.Image{
		Name:   local,
		Labels: m.Annotations,
		Target: specs.Descriptor{
			MediaType: string(mt),
			Digest:    digest.Digest(d.String()),
			Size:      c,
		},
	}, nil
}
