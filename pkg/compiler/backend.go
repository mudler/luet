package compiler

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	artifact "github.com/mudler/luet/pkg/api/core/types/artifact"

	"github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/config"
	"github.com/pkg/errors"

	. "github.com/mudler/luet/pkg/logger"
)

func NewBackend(s string) (CompilerBackend, error) {
	var compilerBackend CompilerBackend

	switch s {
	case backend.ImgBackend:
		compilerBackend = backend.NewSimpleImgBackend()
	case backend.DockerBackend:
		compilerBackend = backend.NewSimpleDockerBackend()
	default:
		return nil, errors.New("invalid backend. Unsupported")
	}

	return compilerBackend, nil
}

type CompilerBackend interface {
	BuildImage(backend.Options) error
	ExportImage(backend.Options) error
	RemoveImage(backend.Options) error
	ImageDefinitionToTar(backend.Options) error
	ExtractRootfs(opts backend.Options, keepPerms bool) error

	CopyImage(string, string) error
	DownloadImage(opts backend.Options) error

	Push(opts backend.Options) error
	ImageAvailable(string) bool

	ImageExists(string) bool
}

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
func GenerateChanges(b CompilerBackend, fromImage, toImage backend.Options) ([]artifact.ArtifactLayer, error) {

	res := artifact.ArtifactLayer{FromImage: fromImage.ImageName, ToImage: toImage.ImageName}

	tmpdiffs, err := config.LuetCfg.GetSystem().TempDir("extraction")
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(tmpdiffs) // clean up

	srcRootFS, err := ioutil.TempDir(tmpdiffs, "src")
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(srcRootFS) // clean up

	dstRootFS, err := ioutil.TempDir(tmpdiffs, "dst")
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while creating tempdir for rootfs")
	}
	defer os.RemoveAll(dstRootFS) // clean up

	srcImageExtract := backend.Options{
		ImageName:   fromImage.ImageName,
		Destination: srcRootFS,
	}
	Debug("Extracting source image", fromImage.ImageName)
	err = b.ExtractRootfs(srcImageExtract, false) // No need to keep permissions as we just collect file diffs
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while unpacking src image "+fromImage.ImageName)
	}

	dstImageExtract := backend.Options{
		ImageName:   toImage.ImageName,
		Destination: dstRootFS,
	}
	Debug("Extracting destination image", toImage.ImageName)
	err = b.ExtractRootfs(dstImageExtract, false)
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while unpacking dst image "+toImage.ImageName)
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
				res.Diffs.Changes = append(res.Diffs.Changes, artifact.ArtifactNode{
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
			res.Diffs.Additions = append(res.Diffs.Additions, artifact.ArtifactNode{
				Name: filepath.Join("/", realpath),
				Size: int(sizeB),
			})

			// fmt.Println("File created", path, filepath.Join(srcRootFS, realpath))
		}

		return nil
	})
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while walking image destination")
	}

	// Get deletions. src -> dst
	err = filepath.Walk(srcRootFS, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		realpath := strings.Replace(path, srcRootFS, "", -1)
		if _, err = os.Lstat(filepath.Join(dstRootFS, realpath)); err != nil {
			// fmt.Println("File deleted", path, filepath.Join(srcRootFS, realpath))
			res.Diffs.Deletions = append(res.Diffs.Deletions, artifact.ArtifactNode{
				Name: filepath.Join("/", realpath),
			})
		}

		return nil
	})
	if err != nil {
		return []artifact.ArtifactLayer{}, errors.Wrap(err, "Error met while walking image source")
	}

	diffs := []artifact.ArtifactLayer{res}

	if config.LuetCfg.GetGeneral().Debug {
		summary := ComputeArtifactLayerSummary(diffs)
		for _, l := range summary.Layers {
			Debug(fmt.Sprintf("Diff %s -> %s: add %d (%d bytes), del %d (%d bytes), change %d (%d bytes)",
				l.FromImage, l.ToImage,
				l.AddFiles, l.AddSizes,
				l.DelFiles, l.DelSizes,
				l.ChangeFiles, l.ChangeSizes))
		}
	}

	return diffs, nil
}

type ArtifactLayerSummary struct {
	FromImage   string `json:"image1"`
	ToImage     string `json:"image2"`
	AddFiles    int    `json:"add_files"`
	AddSizes    int64  `json:"add_sizes"`
	DelFiles    int    `json:"del_files"`
	DelSizes    int64  `json:"del_sizes"`
	ChangeFiles int    `json:"change_files"`
	ChangeSizes int64  `json:"change_sizes"`
}

type ArtifactLayersSummary struct {
	Layers []ArtifactLayerSummary `json:"summary"`
}

func ComputeArtifactLayerSummary(diffs []artifact.ArtifactLayer) ArtifactLayersSummary {

	ans := ArtifactLayersSummary{
		Layers: make([]ArtifactLayerSummary, 0),
	}

	for _, layer := range diffs {
		sum := ArtifactLayerSummary{
			FromImage:   layer.FromImage,
			ToImage:     layer.ToImage,
			AddFiles:    0,
			AddSizes:    0,
			DelFiles:    0,
			DelSizes:    0,
			ChangeFiles: 0,
			ChangeSizes: 0,
		}
		for _, a := range layer.Diffs.Additions {
			sum.AddFiles++
			sum.AddSizes += int64(a.Size)
		}
		for _, d := range layer.Diffs.Deletions {
			sum.DelFiles++
			sum.DelSizes += int64(d.Size)
		}
		for _, c := range layer.Diffs.Changes {
			sum.ChangeFiles++
			sum.ChangeSizes += int64(c.Size)
		}
		ans.Layers = append(ans.Layers, sum)
	}

	return ans
}
