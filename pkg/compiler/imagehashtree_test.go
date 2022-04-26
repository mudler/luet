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
	"github.com/mudler/luet/pkg/api/core/types"

	"github.com/mudler/luet/pkg/api/core/context"
	. "github.com/mudler/luet/pkg/compiler"
	sd "github.com/mudler/luet/pkg/compiler/backend"
	pkg "github.com/mudler/luet/pkg/database"
	"github.com/mudler/luet/pkg/tree"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImageHashTree", func() {
	ctx := context.NewContext()
	generalRecipe := tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
	compiler := NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), Concurrency(2))
	hashtree := NewHashTree(generalRecipe.GetDatabase())
	Context("Simple package definition", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))
			err := generalRecipe.Load("../../tests/fixtures/buildable")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})

		It("Calculates the hash correctly", func() {

			spec, err := compiler.FromPackage(&types.Package{Name: "b", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(packageHash.Target.Hash.BuildHash).To(Equal("bf767dba10e4aa9c25e09f1f61ed9944b8e4736f72b2a1f9ac0125f68a714580"), packageHash.Target.Hash.BuildHash)
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("6ce76e1a85f02841db083e59d4f9d3e4ab16154f925c1d81014c4938a6b1b1f9"), packageHash.Target.Hash.PackageHash)
			Expect(packageHash.BuilderImageHash).To(Equal("builder-4ba2735d6368f56627776f8fb8ce6a16"), packageHash.BuilderImageHash)
		})
	})

	expectedPackageHash := "562b4295b87d561af237997e1320560ee9495a02f69c3c77391b783d2e01ced2"

	Context("complex package definition", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			err := generalRecipe.Load("../../tests/fixtures/upgrade_old_repo_revision")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})
		It("Calculates the hash correctly", func() {
			spec, err := compiler.FromPackage(&types.Package{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())

			expectedHash := "c5b87e16b2ecafc67e671d8e2c38adf4c4a6eed2a80180229d5892d52e81779b"

			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal(expectedPackageHash), packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash)
			Expect(packageHash.SourceHash).To(Equal(expectedPackageHash), packageHash.SourceHash)
			Expect(packageHash.BuilderImageHash).To(Equal("builder-d934bd6bbf716f5d598d764532bc585c"), packageHash.BuilderImageHash)

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("78cace3ee661d14cb2b6236df3dcdc789e36c26a1701ba3e0213e355540a1174"), packageHash.Target.Hash.PackageHash)
			a := &types.Package{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())

			Expect(hash).To(Equal(expectedHash), hash)

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())
			Expect(assertionA.Hash.PackageHash).To(Equal(expectedPackageHash))
			b := &types.Package{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())

			Expect(assertionB.Hash.PackageHash).To(Equal(expectedHash))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())

			Expect(hashB).To(Equal("9ece11c782e862e366ab4b42fdaaea9d89abe41ff4d9ed1bd24c81f6041bc9da"), hashB)
		})
	})

	Context("complex package definition, with small change in build.yaml", func() {
		BeforeEach(func() {
			generalRecipe = tree.NewCompilerRecipe(pkg.NewInMemoryDatabase(false))

			//Definition of A here is slightly changed in the steps build.yaml file (1 character only)
			err := generalRecipe.Load("../../tests/fixtures/upgrade_old_repo_revision_content_changed")
			Expect(err).ToNot(HaveOccurred())
			compiler = NewLuetCompiler(sd.NewSimpleDockerBackend(ctx), generalRecipe.GetDatabase(), Concurrency(2))
			hashtree = NewHashTree(generalRecipe.GetDatabase())

		})
		It("Calculates the hash correctly", func() {
			spec, err := compiler.FromPackage(&types.Package{Name: "c", Category: "test", Version: "1.0"})
			Expect(err).ToNot(HaveOccurred())

			packageHash, err := hashtree.Query(compiler, spec)
			Expect(err).ToNot(HaveOccurred())
			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).ToNot(Equal(expectedPackageHash), packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash)
			sourceHash := "726635a86f03483c432e33d80ba85443cf30453960826bd813d816786f712bcf"
			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal(sourceHash), packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash)
			Expect(packageHash.SourceHash).To(Equal(sourceHash), packageHash.SourceHash)
			Expect(packageHash.SourceHash).ToNot(Equal(expectedPackageHash), packageHash.SourceHash)

			Expect(packageHash.BuilderImageHash).To(Equal("builder-d326b367b72ae030a545e8713d45c9aa"), packageHash.BuilderImageHash)

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("e99b996d2ae378e901668b2f56b184af694fe1f1bc92544a2813d6102738098d"), packageHash.Target.Hash.PackageHash)
			a := &types.Package{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())

			Expect(hash).To(Equal("c5b87e16b2ecafc67e671d8e2c38adf4c4a6eed2a80180229d5892d52e81779b"), hash)

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())

			Expect(assertionA.Hash.PackageHash).To(Equal("726635a86f03483c432e33d80ba85443cf30453960826bd813d816786f712bcf"), assertionA.Hash.PackageHash)
			Expect(assertionA.Hash.PackageHash).ToNot(Equal(expectedPackageHash), assertionA.Hash.PackageHash)

			b := &types.Package{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())

			Expect(assertionB.Hash.PackageHash).To(Equal("c5b87e16b2ecafc67e671d8e2c38adf4c4a6eed2a80180229d5892d52e81779b"), assertionB.Hash.PackageHash)
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())

			Expect(hashB).To(Equal("9ece11c782e862e366ab4b42fdaaea9d89abe41ff4d9ed1bd24c81f6041bc9da"), hashB)
		})
	})

})
