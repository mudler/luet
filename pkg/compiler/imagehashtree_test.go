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

package compiler_test

import (
	. "github.com/mudler/luet/pkg/compiler"
	sd "github.com/mudler/luet/pkg/compiler/backend"
	"github.com/mudler/luet/pkg/compiler/types/options"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImageHashTree", func() {
	generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
	compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), options.Concurrency(2))
	hashtree := NewHashTree(generalRecipe.GetDatabase())
	Context("Simple package definition", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), options.Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})

		It("Calculates the hash correctly", func() {

			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(packageHash.Target.Hash.BuildHash).To(Equal("6490e800fe443b99328fc363529aee74bda513930fb27ce6ab814d692bba068e"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.BuilderImageHash).To(Equal("builder-79462b60bf899ad79db63f194a3c9c2a"))
		})
	})

	Context("complex package definition", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/upgrade_old_repo_revision")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), options.Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})
		It("Calculates the hash correctly", func() {
			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal("c46e653125d71ee3fd696b3941ec1ed6e8a0268f896204c7a222a5aa03eb9982"))
			Expect(packageHash.SourceHash).To(Equal("c46e653125d71ee3fd696b3941ec1ed6e8a0268f896204c7a222a5aa03eb9982"))
			Expect(packageHash.BuilderImageHash).To(Equal("builder-37f4d05ba8a39525742ca364f69b4090"))

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("bb1d9a99c0c309a297c75b436504e664a42121fadbb4e035bda403cd418117aa"))
			a := &pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())
			Expect(assertionA.Hash.PackageHash).To(Equal("c46e653125d71ee3fd696b3941ec1ed6e8a0268f896204c7a222a5aa03eb9982"))
			b := &pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())
			Expect(assertionB.Hash.PackageHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())
			Expect(hashB).To(Equal("6490e800fe443b99328fc363529aee74bda513930fb27ce6ab814d692bba068e"))
		})
	})

})
