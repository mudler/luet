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

			Expect(packageHash.Target.Hash.BuildHash).To(Equal("ec62e3e2cfb4c520c8b2561797c005d248c2659295f3660fa1a66582fc4dc280"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("5fa15a0eb0534eaa78ef1b4e32fe72704effaa5e54399b7cab6d630aa0aeac5c"))
			Expect(packageHash.BuilderImageHash).To(Equal("builder-96e0c42b5741376ebcf0a47c8ec1c481"))
		})
	})

	expectedPackageHash := "bc6d354e8b9480b70c6f17eafa34cef387b8443ad150b7c9528fb7e94b764e90"

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

			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal(expectedPackageHash))
			Expect(packageHash.SourceHash).To(Equal(expectedPackageHash))
			Expect(packageHash.BuilderImageHash).To(Equal("builder-9b2bc16985446c41eca8f7922ec98078"))

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("bb84a30ced857725fcb575e87fe33d4aefe911abfdd5f9063bbaeb9e4b94e9e2"))
			a := &pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())

			Expect(hash).To(Equal("484f14294d96fd3b51cec1f2db37a269b7b903f3516b74b0cb0771b65d85b799"))

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())
			Expect(assertionA.Hash.PackageHash).To(Equal(expectedPackageHash))
			b := &pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())
			Expect(assertionB.Hash.PackageHash).To(Equal("484f14294d96fd3b51cec1f2db37a269b7b903f3516b74b0cb0771b65d85b799"))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())
			Expect(hashB).To(Equal("828c983e755353190540565a29e71c9eb4c48d6303e1fd2c523235b7c2339c73"))
		})
	})

	Context("complex package definition, with small change in build.yaml", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			//Definition of A here is slightly changed in the steps build.yaml file (1 character only)
			err := generalRecipe.Load("../../tests/fixtures/upgrade_old_repo_revision_content_changed")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(), generalRecipe.GetDatabase(), options.Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})
		It("Calculates the hash correctly", func() {
			spec, err := compiler.FromPackage(&pkg.DefaultPackage{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).ToNot(Equal(expectedPackageHash))
			sourceHash := "ed1bd90e696904982a1f51998646a335067329e1a262994b5ae15c579106ac81"
			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal(sourceHash))
			Expect(packageHash.SourceHash).To(Equal(sourceHash))
			Expect(packageHash.SourceHash).ToNot(Equal(expectedPackageHash))

			Expect(packageHash.BuilderImageHash).To(Equal("builder-f4b0e366e0a42774428fbdc9aa325648"))

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("2618f12851a596f6801e2665e07147da98a0a151f44500a54ca8b76b869e378d"))
			a := &pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).To(Equal("484f14294d96fd3b51cec1f2db37a269b7b903f3516b74b0cb0771b65d85b799"))

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())

			Expect(assertionA.Hash.PackageHash).To(Equal("ed1bd90e696904982a1f51998646a335067329e1a262994b5ae15c579106ac81"))
			Expect(assertionA.Hash.PackageHash).ToNot(Equal(expectedPackageHash))

			b := &pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())

			Expect(assertionB.Hash.PackageHash).To(Equal("484f14294d96fd3b51cec1f2db37a269b7b903f3516b74b0cb0771b65d85b799"))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())

			Expect(hashB).To(Equal("828c983e755353190540565a29e71c9eb4c48d6303e1fd2c523235b7c2339c73"))
		})
	})

})
