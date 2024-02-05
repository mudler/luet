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
	"crypto/tls"
	"github.com/google/go-containerregistry/pkg/crane"
	"net"
	"net/http"
	"time"
)

// Available checks if the image is available in the remote endpoint.
func Available(image string, opt ...crane.Option) bool {
	// We use crane.insecure as we just check if the image is available
	// It's the daemon duty to use it or not based on the host settings

	// Dupe of the remote.DefaultTransport but with InsecureSkipVerify: true for the TLS config
	// They no longer provide a Clone method, so we have to copy the whole thing manually
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// We usually are dealing with 2 hosts (at most), split MaxIdleConns between them.
		MaxIdleConnsPerHost: 50,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	if len(opt) == 0 {
		opt = append(opt, crane.Insecure, crane.WithTransport(tr))
	}

	_, err := crane.Digest(image, opt...)
	return err == nil
}
