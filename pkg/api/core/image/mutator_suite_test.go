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

package image_test

import (
	"testing"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestImageApi(t *testing.T) {
	RegisterFailHandler(Fail)
	b := backend.NewSimpleDockerBackend(context.NewContext())
	b.DownloadImage(backend.Options{ImageName: "alpine"})
	b.DownloadImage(backend.Options{ImageName: "golang:alpine"})
	b.DownloadImage(backend.Options{ImageName: "golang:1.16-alpine3.14"})
	RunSpecs(t, "Image API Suite")
}
