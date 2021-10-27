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
func ExtractDeltaAdditionsFiles(
	ctx *types.Context,
	srcimg v1.Image,
	includes []string, excludes []string,
) (func(h *tar.Header) (bool, error), error) {

	includeRegexp := compileRegexes(includes)
	excludeRegexp := compileRegexes(excludes)

	srcfilesd, err := ctx.Config.System.TempDir("srcfiles")
	if err != nil {
		return nil, err
	}
	filesSrc := NewCache(srcfilesd, 50*1024*1024, 10000)

	srcReader := mutate.Extract(srcimg)
	defer srcReader.Close()

	srcTar := tar.NewReader(srcReader)

	for {
		var hdr *tar.Header
		hdr, err := srcTar.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return nil, err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			filesSrc.Set(filepath.Dir(hdr.Name), "")
		default:
			filesSrc.Set(hdr.Name, "")
		}
	}

	return func(h *tar.Header) (bool, error) {

		fileName := filepath.Join(string(os.PathSeparator), h.Name)
		_, exists := filesSrc.Get(h.Name)
		if exists {
			return false, nil
		}

		switch {
		case len(includes) == 0 && len(excludes) != 0:
			for _, i := range excludeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) &&
					fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					return false, nil
				}
			}
			ctx.Debug("Adding name", fileName)

			return true, nil
		case len(includes) > 0 && len(excludes) == 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) && fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}
			return false, nil
		case len(includes) != 0 && len(excludes) != 0:
			for _, i := range includeRegexp {
				if i.MatchString(filepath.Join(string(os.PathSeparator), h.Name)) && fileName == filepath.Join(string(os.PathSeparator), h.Name) {
					for _, e := range excludeRegexp {
						if e.MatchString(fileName) {
							return false, nil
						}
					}
					ctx.Debug("Adding name", fileName)

					return true, nil
				}
			}

			return false, nil
		default:
			ctx.Debug("Adding name", fileName)
			return true, nil
		}

	}, nil
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

func ExtractReader(ctx *types.Context, reader io.ReadCloser, output string, keepPerms bool, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	defer reader.Close()

	// If no filter is specified, grab all.
	if filter == nil {
		filter = func(h *tar.Header) (bool, error) { return true, nil }
	}

	// Keep records of permissions as we walk the tar
	type permData struct {
		PAX, Xattrs map[string]string
		Uid, Gid    int
		Name        string
	}

	permstore, err := ctx.Config.System.TempDir("permstore")
	if err != nil {
		return 0, "", err
	}
	perms := NewCache(permstore, 50*1024*1024, 10000)

	f := func(h *tar.Header) (bool, error) {
		res, err := filter(h)
		if res {
			perms.SetValue(h.Name, permData{
				PAX: h.PAXRecords,
				Uid: h.Uid, Gid: h.Gid,
				Xattrs: h.Xattrs,
				Name:   h.Name,
			})
			//perms = append(perms, })
		}
		return res, err
	}

	opts = append(opts, containerdarchive.WithFilter(f))

	// Handle the extraction
	c, err := containerdarchive.Apply(context.Background(), output, reader, opts...)
	if err != nil {
		return 0, "", err
	}

	// Reconstruct permissions
	if keepPerms {
		ctx.Debug("Reconstructing permissions")
		perms.All(func(cr CacheResult) {
			p := &permData{}
			cr.Unmarshal(p)
			ff := filepath.Join(output, p.Name)
			if _, err := os.Lstat(ff); err == nil {
				if err := os.Lchown(ff, p.Uid, p.Gid); err != nil {
					ctx.Warning(err, "failed chowning file")
				}
			}
			for _, attrs := range []map[string]string{p.Xattrs, p.PAX} {
				for k, attr := range attrs {
					if err := system.Lsetxattr(ff, k, []byte(attr), 0); err != nil {
						if errors.Is(err, syscall.ENOTSUP) {
							ctx.Debug("ignored xattr %s in archive", ff)
						}
					}
				}
			}
		})
	}
	return c, output, nil
}

func Extract(ctx *types.Context, img v1.Image, keepPerms bool, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	tmpdiffs, err := ctx.Config.GetSystem().TempDir("extraction")
	if err != nil {
		return 0, "", errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	return ExtractReader(ctx, mutate.Extract(img), tmpdiffs, keepPerms, filter, opts...)
}

func ExtractTo(ctx *types.Context, img v1.Image, output string, keepPerms bool, filter func(h *tar.Header) (bool, error), opts ...containerdarchive.ApplyOpt) (int64, string, error) {
	return ExtractReader(ctx, mutate.Extract(img), output, keepPerms, filter, opts...)
}
