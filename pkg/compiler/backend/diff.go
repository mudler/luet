// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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

package backend

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/config"
	"github.com/pkg/errors"
)

// GenerateChanges generates changes between two images using a backend by leveraging export/extractrootfs methods
// example of json return: [
//   {
//     "Image1": "luet/base",
//     "Image2": "alpine",
//     "DiffType": "File",
//     "Diff": {
//       "Adds": null,
//       "Dels": [
//         {
//           "Name": "/luetbuild",
//           "Size": 5830706
//         },
//         {
//           "Name": "/luetbuild/Dockerfile",
//           "Size": 50
//         },
//         {
//           "Name": "/luetbuild/output1",
//           "Size": 5830656
//         }
//       ],
//       "Mods": null
//     }
//   }
// ]
func GenerateChanges(b compiler.CompilerBackend, fromImage, toImage compiler.CompilerBackendOptions) ([]compiler.ArtifactLayer, error) {

	res := compiler.ArtifactLayer{FromImage: fromImage.ImageName, ToImage: toImage.ImageName}

	tmpdiffs, err := config.LuetCfg.GetSystem().TempDir("extraction")
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(tmpdiffs) // clean up

	srcRootFS, err := ioutil.TempDir(tmpdiffs, "src")
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(srcRootFS) // clean up

	dstRootFS, err := ioutil.TempDir(tmpdiffs, "dst")
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(dstRootFS) // clean up

	srcImageExtract := compiler.CompilerBackendOptions{
		ImageName:   fromImage.ImageName,
		Destination: srcRootFS,
	}
	err = b.ExtractRootfs(srcImageExtract, false) // No need to keep permissions as we just collect file diffs
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while unpacking src image "+fromImage.ImageName)
	}

	dstImageExtract := compiler.CompilerBackendOptions{
		ImageName:   toImage.ImageName,
		Destination: dstRootFS,
	}
	err = b.ExtractRootfs(dstImageExtract, false)
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while unpacking dst image "+toImage.ImageName)
	}

	// Get Additions/Changes. dst -> src
	err = filepath.Walk(dstRootFS, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		realpath := strings.Replace(path, dstRootFS, "", -1)
		fileInfo, err := os.Lstat(filepath.Join(srcRootFS, realpath))
		if err == nil {
			var sizeA, sizeB int64
			sizeA = fileInfo.Size()

			if s, err := os.Lstat(filepath.Join(dstRootFS, realpath)); err == nil {
				sizeB = s.Size()
			}

			if sizeA != sizeB {
				// fmt.Println("File changed", path, filepath.Join(srcRootFS, realpath))
				res.Diffs.Changes = append(res.Diffs.Changes, compiler.ArtifactNode{
					Name: filepath.Join("/", realpath),
					Size: int(sizeB),
				})
			} else {
				// fmt.Println("File already exists", path, filepath.Join(srcRootFS, realpath))
			}
		} else {
			var sizeB int64

			if s, err := os.Lstat(filepath.Join(dstRootFS, realpath)); err == nil {
				sizeB = s.Size()
			}
			res.Diffs.Additions = append(res.Diffs.Additions, compiler.ArtifactNode{
				Name: filepath.Join("/", realpath),
				Size: int(sizeB),
			})

			// fmt.Println("File created", path, filepath.Join(srcRootFS, realpath))
		}

		return nil
	})
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while walking image destination")
	}

	// Get deletions. src -> dst
	err = filepath.Walk(srcRootFS, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		realpath := strings.Replace(path, srcRootFS, "", -1)
		if _, err = os.Lstat(filepath.Join(dstRootFS, realpath)); err != nil {
			// fmt.Println("File deleted", path, filepath.Join(srcRootFS, realpath))
			res.Diffs.Deletions = append(res.Diffs.Deletions, compiler.ArtifactNode{
				Name: filepath.Join("/", realpath),
			})
		}

		return nil
	})
	if err != nil {
		return []compiler.ArtifactLayer{}, errors.Wrap(err, "Error met while walking image source")
	}

	return []compiler.ArtifactLayer{res}, nil
}
