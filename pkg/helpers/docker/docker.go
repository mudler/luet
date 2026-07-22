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
	"net/http"
	"os"
	"strings"

	"github.com/containerd/containerd/images"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
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
)

type staticAuth struct {
	auth *registrytypes.AuthConfig
}

func (s staticAuth) Authorization() (*authn.AuthConfig, error) {
	if s.auth == nil {
		return nil, nil
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

// DownloadAndExtractDockerImage extracts a container image natively. It supports privileged/unprivileged mode
func DownloadAndExtractDockerImage(ctx luettypes.Context, image, dest string, auth *registrytypes.AuthConfig, verify bool, platform string) (*images.Image, error) {
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
	if platform != "" {
		p, err := v1.ParsePlatform(platform)
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

func ExtractDockerImage(ctx luettypes.Context, local, dest, platform string) (*images.Image, error) {
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
