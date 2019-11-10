package api

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/heroku/docker-registry-client/registry"
	jww "github.com/spf13/jwalterweatherman"
)

const defaultRegistryBase = "https://registry-1.docker.io"

type DownloadOpts struct {
	RegistryBase string
	KeepLayers   bool
	UnpackMode   string
}

func DownloadAndUnpackImage(sourceImage, output string, opts *DownloadOpts) error {

	if opts.RegistryBase == "" {
		opts.RegistryBase = defaultRegistryBase
	}

	var TempDir = os.Getenv("TEMP_LAYER_FOLDER")
	if TempDir == "" {
		TempDir = "layers"
	}
	err := os.MkdirAll(TempDir, os.ModePerm)
	if err != nil {
		return err
	}
	if opts.KeepLayers == false {
		defer os.RemoveAll(TempDir)
	}

	if sourceImage != "" && strings.Contains(sourceImage, ":") {
		parts := strings.Split(sourceImage, ":")
		if parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("Bad usage. Image should be in this format: foo/my-image:latest")
		}
	}

	tagPart := "latest"
	repoPart := sourceImage
	parts := strings.Split(sourceImage, ":")
	if len(parts) > 1 {
		repoPart = parts[0]
		tagPart = parts[1]
	}

	jww.INFO.Println("Unpacking", repoPart, "tag", tagPart, "in", output)
	os.MkdirAll(output, os.ModePerm)
	username := "" // anonymous
	password := "" // anonymous
	hub, err := registry.New(opts.RegistryBase, username, password)
	if err != nil {
		jww.ERROR.Fatalln(err)
		return err
	}
	manifest, err := hub.Manifest(repoPart, tagPart)
	if err != nil {
		jww.ERROR.Fatalln(err)
		return err
	}
	layers := manifest.FSLayers
	layers_sha := make([]string, 0)
	for _, l := range layers {
		jww.INFO.Println("Layer ", l)
		// or obtain the digest from an existing manifest's FSLayer list
		s := string(l.BlobSum)
		i := strings.Index(s, ":")
		enc := s[i+1:]
		reader, err := hub.DownloadBlob(repoPart, l.BlobSum)
		layers_sha = append(layers_sha, enc)

		if reader != nil {
			defer reader.Close()
		}
		if err != nil {
			return err
		}

		where := path.Join(TempDir, enc)
		err = os.MkdirAll(where, os.ModePerm)
		if err != nil {
			jww.ERROR.Println(err)
			return err
		}

		out, err := os.Create(path.Join(where, "layer.tar"))
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, reader); err != nil {
			fmt.Println(err)
			return err
		}
	}

	jww.INFO.Println("Download complete")

	export, err := CreateExport(TempDir)
	if err != nil {
		fmt.Println(err)
		return err
	}

	jww.INFO.Println("Unpacking...")

	err = export.UnPackLayers(layers_sha, output, opts.UnpackMode)
	if err != nil {
		jww.INFO.Fatal(err)
		return err
	}

	jww.INFO.Println("Done")
	return nil
}

func CreateExport(layers string) (*Export, error) {

	export := &Export{
		Entries: map[string]*ExportedImage{},
		Path:    layers,
	}

	dirs, err := ioutil.ReadDir(export.Path)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {

		if !dir.IsDir() {
			continue
		}

		entry := &ExportedImage{
			Path:         filepath.Join(export.Path, dir.Name()),
			LayerTarPath: filepath.Join(export.Path, dir.Name(), "layer.tar"),
			LayerDirPath: filepath.Join(export.Path, dir.Name(), "layer"),
		}

		export.Entries[dir.Name()] = entry
	}

	return export, err
}
