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

package image

import (
	"archive/tar"
	"context"

	containerdarchive "github.com/containerd/containerd/archive"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

// Extract dir:
// -> First extract delta considering the dir
// Afterward create artifact pointing to the dir

// ExtractDeltaFiles returns an handler to extract files in a list
func ExtractDeltaFiles(
	ctx *types.Context,
	d ImageDiff,
	includes []string, excludes []string,
) func(h *tar.Header) (bool, error) {

	includeRegexp := compileRegexes(includes)
	excludeRegexp := compileRegexes(excludes)

	return func(h *tar.Header) (bool, error) {
		switch {
		case len(includes) == 0 && len(excludes) != 0:
			for _, a := range d.Additions {
				if h.Name == a.Name {
					for _, i := range excludeRegexp {
						if i.MatchString(a.Name) && h.Name == a.Name {
							return false, nil
						}
					}
					ctx.Info("Adding name", h.Name)

					return true, nil
				}
			}
			return false, nil
		case len(includes) > 0 && len(excludes) == 0:
			for _, a := range d.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) && h.Name == a.Name {
						ctx.Info("Adding name", h.Name)

						return true, nil
					}
				}
			}
			return false, nil
		case len(includes) != 0 && len(excludes) != 0:
			for _, a := range d.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) && h.Name == a.Name {
						for _, e := range excludeRegexp {
							if e.MatchString(a.Name) {
								return false, nil
							}
						}
						ctx.Info("Adding name", h.Name)

						return true, nil
					}
				}
			}
			return false, nil
		default:
			for _, a := range d.Additions {
				if h.Name == a.Name {
					ctx.Info("Adding name", h.Name)

					return true, nil
				}
			}
			return false, nil
		}

	}
}

func Extract(ctx *types.Context, img v1.Image, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (string, error) {
	src := mutate.Extract(img)
	defer src.Close()

	tmpdiffs, err := ctx.Config.GetSystem().TempDir("extraction")
	if err != nil {
		return "", errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}

	if filter != nil {
		opts = append(opts, containerdarchive.WithFilter(filter))
	}

	_, err = containerdarchive.Apply(context.Background(), tmpdiffs, src, opts...)
	if err != nil {
		return "", err
	}

	return tmpdiffs, nil
}
