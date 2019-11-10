package api

// This part is an extract from https://github.com/jwilder/docker-squash/

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	jww "github.com/spf13/jwalterweatherman"
)

type Export struct {
	Entries map[string]*ExportedImage
	Path    string
}

func (e *Export) ExtractLayers(unpackmode string) error {

	jww.INFO.Println("Extracting layers...")

	for _, entry := range e.Entries {
		jww.INFO.Println("  - ", entry.LayerTarPath)
		err := entry.ExtractLayerDir(unpackmode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Export) UnPackLayers(order []string, layerDir string, unpackmode string) error {
	err := os.MkdirAll(layerDir, 0755)
	if err != nil {
		return err
	}

	for _, ee := range order {
		entry := e.Entries[ee]
		if _, err := os.Stat(entry.LayerTarPath); os.IsNotExist(err) {
			continue
		}

		err := ExtractLayer(&ExtractOpts{
			Source:       entry.LayerTarPath,
			Destination:  layerDir,
			Compressed:   true,
			KeepDirlinks: true,
			Rootless:     false,
			UnpackMode:   unpackmode})
		if err != nil {
			jww.INFO.Println(err.Error())
			return err
		}

		jww.INFO.Println("  -  Deleting whiteouts for layer " + ee)
		err = e.deleteWhiteouts(layerDir)
		if err != nil {
			return err
		}
	}
	return nil
}

const TarCmd = "tar"

func (e *Export) deleteWhiteouts(location string) error {
	return filepath.Walk(location, func(p string, info os.FileInfo, err error) error {
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		if info == nil {
			return nil
		}

		name := info.Name()
		parent := filepath.Dir(p)
		// if start with whiteout
		if strings.Index(name, ".wh.") == 0 {
			deletedFile := path.Join(parent, name[len(".wh."):len(name)])
			// remove deleted files
			if err := os.RemoveAll(deletedFile); err != nil {
				return err
			}
			// remove the whiteout itself
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
		return nil
	})
}
