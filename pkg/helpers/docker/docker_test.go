// Copyright © 2026 Ettore Di Giacinto <mudler@mocaccino.org>
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

package docker_test

import (
	"os"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/types"
	. "github.com/mudler/luet/pkg/helpers/docker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExtractDockerImage", func() {
	It("returns an error instead of panicking when the reference cannot be resolved", func() {
		ctx := context.NewContext()

		dest, err := os.MkdirTemp("", "extract-err")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(dest)

		// This host does not resolve, so remote.Image must fail. Before the
		// shadowing fix the error was dropped and img.Manifest() nil-panicked.
		_, err = ExtractDockerImage(ctx, "luet-nonexistent.invalid/nope:latest", dest, types.Platform{})
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("DownloadAndExtractDockerImage platform selection", func() {
	It("resolves different children of a multi-arch image", func() {
		ctx := context.NewContext()

		amdDest, err := os.MkdirTemp("", "plat-amd64")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(amdDest)
		armDest, err := os.MkdirTemp("", "plat-arm64")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(armDest)

		amd64, err := types.ParsePlatform("linux/amd64")
		Expect(err).ToNot(HaveOccurred())
		arm64, err := types.ParsePlatform("linux/arm64")
		Expect(err).ToNot(HaveOccurred())

		amdInfo, err := DownloadAndExtractDockerImage(ctx, "alpine:3.19", amdDest, nil, false, amd64)
		Expect(err).ToNot(HaveOccurred())

		armInfo, err := DownloadAndExtractDockerImage(ctx, "alpine:3.19", armDest, nil, false, arm64)
		Expect(err).ToNot(HaveOccurred())

		// alpine:3.19 is a multi-arch index; the two platforms must resolve to
		// different child manifests. Identical digests mean the platform
		// option was ignored and both requests fell back to one default.
		Expect(amdInfo.Target.Digest).ToNot(Equal(armInfo.Target.Digest))
	})
})
