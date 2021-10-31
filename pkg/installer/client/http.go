// Copyright © 2019-2021 Ettore Di Giacinto <mudler@gentoo.org>
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
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/api/core/types/artifact"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/cavaliercoder/grab"
)

type HttpClient struct {
	RepoData RepoData
	Cache    *artifact.ArtifactCache
	context  *types.Context
}

func NewHttpClient(r RepoData, ctx *types.Context) *HttpClient {
	return &HttpClient{
		RepoData: r,
		Cache:    artifact.NewCache(ctx.Config.GetSystem().GetSystemPkgsCacheDirPath()),
		context:  ctx,
	}
}

func NewGrabClient(timeout int) *grab.Client {
	return &grab.Client{
		UserAgent: "grab",
		HTTPClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
	}
}

func (c *HttpClient) prepareReq(dst, url string) (*grab.Request, error) {

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

func (c *HttpClient) DownloadFile(p string) (string, error) {
	var file *os.File = nil
	var downloaded bool
	temp, err := c.context.Config.GetSystem().TempDir("download")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)

	client := NewGrabClient(c.context.Config.General.HTTPTimeout)

	for _, uri := range c.RepoData.Urls {
		file, err = c.context.Config.GetSystem().TempFile("HttpClient")
		if err != nil {
			c.context.Debug("Failed downloading", p, "from", uri)

			continue
		}
		c.context.Debug("Downloading artifact", p, "from", uri)

		u, err := url.Parse(uri)
		if err != nil {
			continue
		}
		u.Path = path.Join(u.Path, p)

		req, err := c.prepareReq(file.Name(), u.String())
		if err != nil {
			continue
		}

		resp := client.Do(req)

		// Initialize a progressbar only if we have one in the current context
		var pb *pterm.ProgressbarPrinter
		if c.context.ProgressBar != nil {
			pb, _ = c.context.ProgressBar.WithTotal(int(resp.Size())).WithTitle(filepath.Base(resp.Request.HTTPRequest.URL.RequestURI())).Start()
		}
		// start download loop
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()

	download_loop:

		for {
			select {
			case <-t.C:
				//	update the progress bar
				if pb != nil {
					pb.Increment().Current = int(resp.BytesComplete())
				}
			case <-resp.Done:
				//	update the progress bar
				if pb != nil {
					pb.Increment().Current = int(resp.BytesComplete())
				}
				// download is complete
				break download_loop
			}
		}

		if err = resp.Err(); err != nil {
			continue
		}

		c.context.Info("Downloaded", p, "of",
			fmt.Sprintf("%.2f", (float64(resp.BytesComplete())/1000)/1000), "MB (",
			fmt.Sprintf("%.2f", (float64(resp.BytesPerSecond())/1024)/1024), "MiB/s )")

		if pb != nil {
			// stop the progressbar if active
			pb.Stop()
		}
		//bar.Finish()
		downloaded = true
		break
	}

	if !downloaded {
		return "", errors.Wrap(err, "artifact not available in any of the specified url locations")
	}
	return file.Name(), nil
}

func (c *HttpClient) DownloadArtifact(a *artifact.PackageArtifact) (*artifact.PackageArtifact, error) {
	newart := a.ShallowCopy()
	artifactName := path.Base(a.Path)

	fileName, err := c.Cache.Get(a)
	// Check if file is already in cache
	if err == nil {
		newart.Path = fileName
		c.context.Debug("Use artifact", artifactName, "from cache.")
	} else {
		d, err := c.DownloadFile(artifactName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed downloading %s", artifactName)
		}

		defer os.RemoveAll(d)
		newart.Path = d
		c.Cache.Put(newart)

		fileName, err := c.Cache.Get(newart)
		if err != nil {
			return nil, errors.Wrapf(err, "failed getting file from cache %v", newart)
		}

		newart.Path = fileName
	}

	return newart, nil
}
