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
			Expect(packageHash.Target.Hash.BuildHash).To(Equal("4db24406e8db30a3310a1cf8c4d4e19597745e6d41b189dc51a73ac4a50cc9e6"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("4c867c9bab6c71d9420df75806e7a2f171dbc08487852ab4e2487bab04066cf2"))
			Expect(packageHash.BuilderImageHash).To(Equal("builder-e6f9c5552a67c463215b0a9e4f7c7fc8"))
		})
	})

	expectedPackageHash := "15811d83d0f8360318c54d91dcae3714f8efb39bf872572294834880f00ee7a8"

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
			Expect(packageHash.BuilderImageHash).To(Equal("builder-3d739cab442aec15a6da238481df73c5"))

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("99c4ebb4bc4754985fcc28677badf90f525aa231b1db0fe75659f11b86dc20e8"))
			a := &pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).To(Equal("f48f28ab62f1379a4247ec763681ccede68ea1e5c25aae8fb72459c0b2f8742e"))

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())
			Expect(assertionA.Hash.PackageHash).To(Equal(expectedPackageHash))
			b := &pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())
			Expect(assertionB.Hash.PackageHash).To(Equal("f48f28ab62f1379a4247ec763681ccede68ea1e5c25aae8fb72459c0b2f8742e"))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())
			Expect(hashB).To(Equal("2668e418eab6861404834ad617713e39b8e58f68016a1fbfcc9384efdd037376"))
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
			sourceHash := "1d91b13d0246fa000085a1071c63397d21546300b17f69493f22315a64b717d4"
			Expect(packageHash.Dependencies[len(packageHash.Dependencies)-1].Hash.PackageHash).To(Equal(sourceHash))
			Expect(packageHash.SourceHash).To(Equal(sourceHash))
			Expect(packageHash.SourceHash).ToNot(Equal(expectedPackageHash))

			Expect(packageHash.BuilderImageHash).To(Equal("builder-03ee108a7c56b17ee568ace0800dd16d"))

			//Expect(packageHash.Target.Hash.BuildHash).To(Equal("79d7107d13d578b362e6a7bf10ec850efce26316405b8d732ce8f9e004d64281"))
			Expect(packageHash.Target.Hash.PackageHash).To(Equal("7677da23b2cc866c2d07aa4a58fbf703340f2f78c0efbb1ba9faf8979f250c87"))
			a := &pkg.DefaultPackage{Name: "a", Category: "test", Version: "1.1"}
			hash, err := packageHash.DependencyBuildImage(a)
			Expect(err).ToNot(HaveOccurred())
			Expect(hash).To(Equal("f48f28ab62f1379a4247ec763681ccede68ea1e5c25aae8fb72459c0b2f8742e"))

			assertionA := packageHash.Dependencies.Search(a.GetFingerPrint())

			Expect(assertionA.Hash.PackageHash).To(Equal("1d91b13d0246fa000085a1071c63397d21546300b17f69493f22315a64b717d4"))
			Expect(assertionA.Hash.PackageHash).ToNot(Equal(expectedPackageHash))

			b := &pkg.DefaultPackage{Name: "b", Category: "test", Version: "1.0"}
			assertionB := packageHash.Dependencies.Search(b.GetFingerPrint())

			Expect(assertionB.Hash.PackageHash).To(Equal("f48f28ab62f1379a4247ec763681ccede68ea1e5c25aae8fb72459c0b2f8742e"))
			hashB, err := packageHash.DependencyBuildImage(b)
			Expect(err).ToNot(HaveOccurred())

			Expect(hashB).To(Equal("2668e418eab6861404834ad617713e39b8e58f68016a1fbfcc9384efdd037376"))
		})
	})

})
