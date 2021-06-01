// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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

package client

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mudler/luet/pkg/compiler/types/artifact"
	fileHelper "github.com/mudler/luet/pkg/helpers/file"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/cavaliercoder/grab"
	"github.com/mudler/luet/pkg/config"

	"github.com/schollz/progressbar/v3"
)

type HttpClient struct {
	RepoData RepoData
}

func NewHttpClient(r RepoData) *HttpClient {
	return &HttpClient{RepoData: r}
}

func (c *HttpClient) PrepareReq(dst, url string) (*grab.Request, error) {

	req, err := grab.NewRequest(dst, url)
	if err != nil {
		return nil, err
	}

	if val, ok := c.RepoData.Authentication["token"]; ok {
		req.HTTPRequest.Header.Set("Authorization", "token "+val)
	} else if val, ok := c.RepoData.Authentication["basic"]; ok {
		req.HTTPRequest.Header.Set("Authorization", "Basic "+val)
	}

	return req, err
}

func Round(input float64) float64 {
	if input < 0 {
		return math.Ceil(input - 0.5)
	}
	return math.Floor(input + 0.5)
}

func (c *HttpClient) DownloadArtifact(a *artifact.PackageArtifact) (*artifact.PackageArtifact, error) {
	var u *url.URL = nil
	var err error
	var req *grab.Request
	var temp string

	artifactName := path.Base(a.Path)
	cacheFile := filepath.Join(config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath(), artifactName)
	ok := false

	// Check if file is already in cache
	if fileHelper.Exists(cacheFile) {
		Debug("Use artifact", artifactName, "from cache.")
	} else {

		temp, err = config.LuetCfg.GetSystem().TempDir("tree")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temp)

		client := grab.NewClient()

		for _, uri := range c.RepoData.Urls {
			Debug("Downloading artifact", artifactName, "from", uri)

			u, err = url.Parse(uri)
			if err != nil {
				continue
			}
			u.Path = path.Join(u.Path, artifactName)

			req, err = c.PrepareReq(temp, u.String())
			if err != nil {
				continue
			}

			resp := client.Do(req)

			bar := progressbar.NewOptions64(
				resp.Size(),
				progressbar.OptionSetDescription(
					fmt.Sprintf("[cyan] %s - [reset]",
						filepath.Base(resp.Request.HTTPRequest.URL.RequestURI()))),
				progressbar.OptionSetRenderBlankState(true),
				progressbar.OptionEnableColorCodes(config.LuetCfg.GetLogging().Color),
				progressbar.OptionClearOnFinish(),
				progressbar.OptionShowBytes(true),
				progressbar.OptionShowCount(),
				progressbar.OptionSetPredictTime(true),
				progressbar.OptionFullWidth(),
				progressbar.OptionSetTheme(progressbar.Theme{
					Saucer:        "[white]=[reset]",
					SaucerHead:    "[white]>[reset]",
					SaucerPadding: " ",
					BarStart:      "[",
					BarEnd:        "]",
				}))

			bar.Reset()
			// start download loop
			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()

		download_loop:

			for {
				select {
				case <-t.C:
					bar.Set64(resp.BytesComplete())

				case <-resp.Done:
					// download is complete
					break download_loop
				}
			}

			if err = resp.Err(); err != nil {
				continue
			}

			if err != nil {
				continue
			}

			Info("\nDownloaded", artifactName, "of",
				fmt.Sprintf("%.2f", (float64(resp.BytesComplete())/1000)/1000), "MB (",
				fmt.Sprintf("%.2f", (float64(resp.BytesPerSecond())/1024)/1024), "MiB/s )")

			Debug("\nCopying file ", filepath.Join(temp, artifactName), "to", cacheFile)
			err = fileHelper.CopyFile(filepath.Join(temp, artifactName), cacheFile)

			bar.Finish()
			ok = true
			break
		}

		if !ok {
			return nil, err
		}
	}

	newart := a
	newart.Path = cacheFile
	return newart, nil
}

func (c *HttpClient) DownloadFile(name string) (string, error) {
	var file *os.File = nil
	var u *url.URL = nil
	var err error
	var req *grab.Request
	var temp string

	ok := false

	temp, err = config.LuetCfg.GetSystem().TempDir("tree")
	if err != nil {
		return "", err
	}

	client := grab.NewClient()

	for _, uri := range c.RepoData.Urls {

		file, err = config.LuetCfg.GetSystem().TempFile("HttpClient")
		if err != nil {
			continue
		}

		u, err = url.Parse(uri)
		if err != nil {
			continue
		}
		u.Path = path.Join(u.Path, name)

		Debug("Downloading", u.String())

		req, err = c.PrepareReq(temp, u.String())
		if err != nil {
			continue
		}

		resp := client.Do(req)
		if err = resp.Err(); err != nil {
			continue
		}

		Info("Downloaded", filepath.Base(resp.Filename), "of",
			fmt.Sprintf("%.2f", (float64(resp.BytesComplete())/1000)/1000), "MB (",
			fmt.Sprintf("%.2f", (float64(resp.BytesPerSecond())/1024)/1024), "MiB/s )")

		err = fileHelper.CopyFile(filepath.Join(temp, name), file.Name())
		if err != nil {
			continue
		}
		ok = true
		break
	}

	if !ok {
		return "", err
	}

	return file.Name(), err
}
