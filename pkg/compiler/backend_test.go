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

package compiler_test

import (
	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/backend"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backend factory", func() {
	ctx := context.NewContext()

	Context("Resolving backend names", func() {
		It("returns a buildah backend for 'buildah'", func() {
			b, err := compiler.NewBackend(ctx, backend.BuildahBackend)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(BeAssignableToTypeOf(&backend.SimpleBuildah{}))
		})

		// The literal "img" is deliberately hardcoded here rather than using
		// backend.ImgBackend. luet-k8s (https://github.com/mudler/luet-k8s)
		// hardcodes the string "img" in its own source, so this alias is a
		// public contract with an external consumer: it must keep resolving to
		// the buildah backend. Pinning the literal means renaming or removing
		// the constant cannot silently move the goalpost. Do not "clean this
		// up" by deleting the alias.
		It("returns a buildah backend for the deprecated 'img' alias", func() {
			b, err := compiler.NewBackend(ctx, "img")
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(BeAssignableToTypeOf(&backend.SimpleBuildah{}))
		})

		It("returns a docker backend for 'docker'", func() {
			b, err := compiler.NewBackend(ctx, backend.DockerBackend)
			Expect(err).ToNot(HaveOccurred())
			Expect(b).To(BeAssignableToTypeOf(&backend.SimpleDocker{}))
		})

		It("returns an error for an unknown backend", func() {
			b, err := compiler.NewBackend(ctx, "notabackend")
			Expect(err).To(HaveOccurred())
			Expect(b).To(BeNil())
		})
	})
})
