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
package cmd

import (
	"os"
	"path/filepath"

	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/pkg/compiler"
	"github.com/mudler/luet/pkg/compiler/types/compression"
	. "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"

	//	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createrepoCmd = &cobra.Command{
	Use:   "create-repo",
	Short: "Create a luet repository from a build",
	Long: `Builds tree metadata from a set of packages and a tree definition:

	$ luet create-repo

Provide specific paths for packages, tree, and metadata output which is generated:

	$ luet create-repo --packages my/packages/path --tree my/tree/path --output my/packages/path ...

Provide name and description of the repository:

	$ luet create-repo --name "foo" --description "bar" ...

Change compression method:
	
	$ luet create-repo --tree-compression gzip --meta-compression gzip

Create a repository from the metadata description defined in the luet.yaml config file:

	$ luet create-repo --repo repository1
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("packages", cmd.Flags().Lookup("packages"))
		viper.BindPFlag("tree", cmd.Flags().Lookup("tree"))
		viper.BindPFlag("output", cmd.Flags().Lookup("output"))
		viper.BindPFlag("backend", cmd.Flags().Lookup("backend"))
		viper.BindPFlag("name", cmd.Flags().Lookup("name"))
		viper.BindPFlag("descr", cmd.Flags().Lookup("descr"))
		viper.BindPFlag("urls", cmd.Flags().Lookup("urls"))
		viper.BindPFlag("type", cmd.Flags().Lookup("type"))
		viper.BindPFlag("tree-compression", cmd.Flags().Lookup("tree-compression"))
		viper.BindPFlag("tree-filename", cmd.Flags().Lookup("tree-filename"))
		viper.BindPFlag("meta-compression", cmd.Flags().Lookup("meta-compression"))
		viper.BindPFlag("meta-filename", cmd.Flags().Lookup("meta-filename"))
		viper.BindPFlag("reset-revision", cmd.Flags().Lookup("reset-revision"))
		viper.BindPFlag("repo", cmd.Flags().Lookup("repo"))

		viper.BindPFlag("force-push", cmd.Flags().Lookup("force-push"))
		viper.BindPFlag("push-images", cmd.Flags().Lookup("push-images"))

	},
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		var repo *installer.LuetSystemRepository

		treePaths := viper.GetStringSlice("tree")
		dst := viper.GetString("output")
		packages := viper.GetString("packages")
		name := viper.GetString("name")
		descr := viper.GetString("descr")
		urls := viper.GetStringSlice("urls")
		t := viper.GetString("type")
		reset := viper.GetBool("reset-revision")
		treetype := viper.GetString("tree-compression")
		treeName := viper.GetString("tree-filename")
		metatype := viper.GetString("meta-compression")
		metaName := viper.GetString("meta-filename")
		source_repo := viper.GetString("repo")
		backendType := viper.GetString("backend")
		fromRepo, _ := cmd.Flags().GetBool("from-repositories")

		treeFile := installer.NewDefaultTreeRepositoryFile()
		metaFile := installer.NewDefaultMetaRepositoryFile()
		compilerBackend, err := compiler.NewBackend(backendType)
		helpers.CheckErr(err)
		force := viper.GetBool("force-push")
		imagePush := viper.GetBool("push-images")

		if source_repo != "" {
			// Search for system repository
			lrepo, err := LuetCfg.GetSystemRepository(source_repo)
			helpers.CheckErr(err)

			if len(treePaths) <= 0 {
				treePaths = []string{lrepo.TreePath}
			}

			if t == "" {
				t = lrepo.Type
			}

			repo, err = installer.GenerateRepository(lrepo.Name,
				lrepo.Description, t,
				lrepo.Urls,
				lrepo.Priority,
				packages,
				treePaths,
				pkg.NewInMemoryDatabase(false),
				compilerBackend,
				dst,
				imagePush,
				force,
				fromRepo,
				LuetCfg)
			helpers.CheckErr(err)

		} else {
			repo, err = installer.GenerateRepository(name, descr, t, urls, 1, packages,
				treePaths, pkg.NewInMemoryDatabase(false), compilerBackend, dst, imagePush, force, fromRepo, LuetCfg)
			helpers.CheckErr(err)
		}

		if treetype != "" {
			treeFile.SetCompressionType(compression.Implementation(treetype))
		}

		if treeName != "" {
			treeFile.SetFileName(treeName)
		}

		if metatype != "" {
			metaFile.SetCompressionType(compression.Implementation(metatype))
		}

		if metaName != "" {
			metaFile.SetFileName(metaName)
		}

		repo.SetRepositoryFile(installer.REPOFILE_TREE_KEY, treeFile)
		repo.SetRepositoryFile(installer.REPOFILE_META_KEY, metaFile)

		err = repo.Write(dst, reset, true)
		helpers.CheckErr(err)

	},
}

func init() {
	path, err := os.Getwd()
	helpers.CheckErr(err)

	createrepoCmd.Flags().String("packages", filepath.Join(path, "build"), "Packages folder (output from build)")
	createrepoCmd.Flags().StringSliceP("tree", "t", []string{path}, "Path of the source trees to use.")
	createrepoCmd.Flags().String("output", filepath.Join(path, "build"), "Destination for generated archives. With 'docker' repository type, it should be an image reference (e.g 'foo/bar')")
	createrepoCmd.Flags().String("name", "luet", "Repository name")
	createrepoCmd.Flags().String("descr", "luet", "Repository description")
	createrepoCmd.Flags().StringSlice("urls", []string{}, "Repository URLs")
	createrepoCmd.Flags().String("type", "disk", "Repository type (disk, http, docker)")
	createrepoCmd.Flags().Bool("reset-revision", false, "Reset repository revision.")
	createrepoCmd.Flags().String("repo", "", "Use repository defined in configuration.")
	createrepoCmd.Flags().String("backend", "docker", "backend used (docker,img)")

	createrepoCmd.Flags().Bool("force-push", false, "Force overwrite of docker images if already present online")
	createrepoCmd.Flags().Bool("push-images", false, "Enable/Disable docker image push for docker repositories")

	createrepoCmd.Flags().String("tree-compression", "gzip", "Compression alg: none, gzip, zstd")
	createrepoCmd.Flags().String("tree-filename", installer.TREE_TARBALL, "Repository tree filename")
	createrepoCmd.Flags().String("meta-compression", "none", "Compression alg: none, gzip, zstd")
	createrepoCmd.Flags().String("meta-filename", installer.REPOSITORY_METAFILE+".tar", "Repository metadata filename")
	createrepoCmd.Flags().Bool("from-repositories", false, "Consume the user-defined repositories to pull specfiles from")

	RootCmd.AddCommand(createrepoCmd)
}
