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
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	containerdarchive "github.com/containerd/containerd/archive"
	"github.com/docker/docker/pkg/system"
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
					ctx.Debug("Adding name", h.Name)

					return true, nil
				}
			}
			return false, nil
		case len(includes) > 0 && len(excludes) == 0:
			for _, a := range d.Additions {
				for _, i := range includeRegexp {
					if i.MatchString(a.Name) && h.Name == a.Name {
						ctx.Debug("Adding name", h.Name)

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
						ctx.Debug("Adding name", h.Name)

						return true, nil
					}
				}
			}
			return false, nil
		default:
			for _, a := range d.Additions {
				if h.Name == a.Name {
					ctx.Debug("Adding name", h.Name)

					return true, nil
				}
			}
			return false, nil
		}

	}
}

func ExtractFiles(
	ctx *types.Context,
	prefixPath string,
	includes []string, excludes []string,
) func(h *tar.Header) (bool, error) {
	includeRegexp := compileRegexes(includes)
	excludeRegexp := compileRegexes(excludes)

	return func(h *tar.Header) (bool, error) {

		fileName := filepath.Join(string(os.PathSeparator), h.Name)
		switch {
		case len(includes) == 0 && len(excludes) != 0:
			for _, i := range excludeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					return false, nil
				}
			}
			if prefixPath != "" {
				return strings.HasPrefix(fileName, prefixPath), nil
			}
			ctx.Debug("Adding name", fileName)
			return true, nil

		case len(includes) > 0 && len(excludes) == 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					if prefixPath != "" {
						return strings.HasPrefix(fileName, prefixPath), nil
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		case len(includes) != 0 && len(excludes) != 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(prefixPath, fileName)) {
					for _, e := range excludeRegexp {
						if e.MatchString(filepath.Join(prefixPath, fileName)) {
							return false, nil
						}
					}
					if prefixPath != "" {
						return strings.HasPrefix(fileName, prefixPath), nil
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		default:
			if prefixPath != "" {
				return strings.HasPrefix(fileName, prefixPath), nil
			}

			return true, nil
		}
	}
}

func ExtractReader(ctx *types.Context, reader io.ReadCloser, output string, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	defer reader.Close()

	perms := map[string][]int{}
	xattrs := map[string]map[string]string{}
	paxrecords := map[string]map[string]string{}

	f := func(h *tar.Header) (bool, error) {
		perms[h.Name] = []int{h.Gid, h.Uid}
		xattrs[h.Name] = h.Xattrs
		paxrecords[h.Name] = h.PAXRecords
		if filter != nil {
			return filter(h)
		}
		return true, nil
	}

	opts = append(opts, containerdarchive.WithFilter(f))

	c, err := containerdarchive.Apply(context.Background(), output, reader, opts...)
	if err != nil {
		return 0, "", err
	}

	for f, p := range perms {
		ff := filepath.Join(output, f)
		if _, err := os.Lstat(ff); err == nil {
			if err := os.Lchown(ff, p[1], p[0]); err != nil {
				ctx.Warning(err, "failed chowning file")
			}
		}
	}

	for _, m := range []map[string]map[string]string{xattrs, paxrecords} {
		for key, attrs := range m {
			ff := filepath.Join(output, key)
			for k, attr := range attrs {
				if err := system.Lsetxattr(ff, k, []byte(attr), 0); err != nil {
					if errors.Is(err, syscall.ENOTSUP) {
						ctx.Debug("ignored xattr %s in archive", key)
					}
				}
			}
		}
	}

	return c, output, nil
}

func Extract(ctx *types.Context, img v1.Image, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	tmpdiffs, err := ctx.Config.GetSystem().TempDir("extraction")
	if err != nil {
		return 0, "", errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	return ExtractReader(ctx, mutate.Extract(img), tmpdiffs, filter, opts...)
}

func ExtractTo(ctx *types.Context, img v1.Image, output string, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	return ExtractReader(ctx, mutate.Extract(img), output, filter, opts...)
}
