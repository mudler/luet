package api

// This part is an extract from https://github.com/jwilder/docker-squash/

import (
	"fmt"
	"os"

	jww "github.com/spf13/jwalterweatherman"

	"os/exec"
)

type ExportedImage struct {
	Path         string
	JsonPath     string
	VersionPath  string
	LayerTarPath string
	LayerDirPath string
}

func (e *ExportedImage) CreateDirs() error {
	return os.MkdirAll(e.Path, 0755)
}

func (e *ExportedImage) TarLayer() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = os.Chdir(e.LayerDirPath)
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	cmd := exec.Command("sudo", "/bin/sh", "-c", fmt.Sprintf("%s cvf ../layer.tar ./", TarCmd))
	out, err := cmd.CombinedOutput()
	if err != nil {
		jww.INFO.Println(out)
		return err
	}
	return nil
}

func (e *ExportedImage) RemoveLayerDir() error {
	return os.RemoveAll(e.LayerDirPath)
}

func (e *ExportedImage) ExtractLayerDir(unpackmode string) error {
	err := os.MkdirAll(e.LayerDirPath, 0755)
	if err != nil {
		return err
	}

	if err := ExtractLayer(&ExtractOpts{
		Source:       e.LayerTarPath,
		Destination:  e.LayerDirPath,
		Compressed:   true,
		KeepDirlinks: true,
		Rootless:     false,
		UnpackMode:   unpackmode}); err != nil {
		return err
	}
	return nil
}
