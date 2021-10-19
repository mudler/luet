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
	"strconv"
	"time"

	"github.com/mudler/luet/pkg/api/core/types/artifact"
	. "github.com/mudler/luet/pkg/logger"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"

	"github.com/cavaliercoder/grab"
	"github.com/mudler/luet/pkg/config"
)

type HttpClient struct {
	RepoData RepoData
	Cache    *artifact.ArtifactCache

	ProgressBarArea *pterm.AreaPrinter
}

func NewHttpClient(r RepoData) *HttpClient {
	return &HttpClient{
		RepoData: r,
		Cache:    artifact.NewCache(config.LuetCfg.GetSystem().GetSystemPkgsCacheDirPath()),
	}
}

func NewGrabClient() *grab.Client {
	httpTimeout := 360
	timeout := os.Getenv("HTTP_TIMEOUT")
	if timeout != "" {
		timeoutI, err := strconv.Atoi(timeout)
		if err == nil {
			httpTimeout = timeoutI
		}
	}

	return &grab.Client{
		UserAgent: "grab",
		HTTPClient: &http.Client{
			Timeout: time.Duration(httpTimeout) * time.Second,
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
	temp, err := config.LuetCfg.GetSystem().TempDir("download")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)

	client := NewGrabClient()

	for _, uri := range c.RepoData.Urls {
		file, err = config.LuetCfg.GetSystem().TempFile("HttpClient")
		if err != nil {
			Debug("Failed downloading", p, "from", uri)

			continue
		}
		Debug("Downloading artifact", p, "from", uri)

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
		pb := pterm.DefaultProgressbar.WithTotal(int(resp.Size()))
		if c.ProgressBarArea != nil {
			pb = pb.WithPrintTogether(c.ProgressBarArea)
		}
		pb, _ = pb.WithTitle(filepath.Base(resp.Request.HTTPRequest.URL.RequestURI())).Start()
		// start download loop
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()

	download_loop:

		for {
			select {
			case <-t.C:
				//	bar.Set64(resp.BytesComplete())
				//pb.Increment()
				pb.Increment().Current = int(resp.BytesComplete())
			case <-resp.Done:
				// download is complete
				break download_loop
			}
		}

		if err = resp.Err(); err != nil {
			continue
		}

		Info("Downloaded", p, "of",
			fmt.Sprintf("%.2f", (float64(resp.BytesComplete())/1000)/1000), "MB (",
			fmt.Sprintf("%.2f", (float64(resp.BytesPerSecond())/1024)/1024), "MiB/s )")
		pb.Stop()
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
		Debug("Use artifact", artifactName, "from cache.")
	} else {
		d, err := c.DownloadFile(artifactName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed downloading %s", artifactName)
		}

		newart.Path = d
		c.Cache.Put(newart)
	}

	return newart, nil
}
