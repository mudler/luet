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

	. "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <term>",
	Short: "Search packages",
	Long:  `Search for installed and available packages`,
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("installed", cmd.Flags().Lookup("installed"))
		LuetCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
		LuetCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
		LuetCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
		LuetCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var systemDB pkg.PackageDatabase

		if len(args) != 1 {
			Fatal("Wrong number of arguments (expected 1)")
		}
		installed := LuetCfg.Viper.GetBool("installed")
		stype := LuetCfg.Viper.GetString("solver.type")
		discount := LuetCfg.Viper.GetFloat64("solver.discount")
		rate := LuetCfg.Viper.GetFloat64("solver.rate")
		attempts := LuetCfg.Viper.GetInt("solver.max_attempts")
		searchWithLabel, _ := cmd.Flags().GetBool("by-label")
		searchWithLabelMatch, _ := cmd.Flags().GetBool("by-label-regex")

		LuetCfg.GetSolverOptions().Type = stype
		LuetCfg.GetSolverOptions().LearnRate = float32(rate)
		LuetCfg.GetSolverOptions().Discount = float32(discount)
		LuetCfg.GetSolverOptions().MaxAttempts = attempts

		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())

		if !installed {

			repos := installer.Repositories{}
			for _, repo := range LuetCfg.SystemRepositories {
				if !repo.Enable {
					continue
				}
				r := installer.NewSystemRepository(repo)
				repos = append(repos, r)
			}

			inst := installer.NewLuetInstaller(
				installer.LuetInstallerOptions{
					Concurrency:   LuetCfg.GetGeneral().Concurrency,
					SolverOptions: *LuetCfg.GetSolverOptions(),
				},
			)
			inst.Repositories(repos)
			synced, err := inst.SyncRepositories(false)
			if err != nil {
				Fatal("Error: " + err.Error())
			}

			Info("--- Search results: ---")

			matches := []installer.PackageMatch{}
			if searchWithLabel {
				matches = synced.SearchLabel(args[0])
			} else if searchWithLabelMatch {
				matches = synced.SearchLabelMatch(args[0])
			} else {
				matches = synced.Search(args[0])
			}
			for _, m := range matches {
				Info(":package:", m.Package.GetCategory(), m.Package.GetName(),
					m.Package.GetVersion(), "repository:", m.Repo.GetName())
			}
		} else {

			if LuetCfg.GetSystem().DatabaseEngine == "boltdb" {
				systemDB = pkg.NewBoltDatabase(
					filepath.Join(LuetCfg.GetSystem().GetSystemRepoDatabaseDirPath(), "luet.db"))
			} else {
				systemDB = pkg.NewInMemoryDatabase(true)
			}
			system := &installer.System{Database: systemDB, Target: LuetCfg.GetSystem().Rootfs}

			var err error
			iMatches := []pkg.Package{}
			if searchWithLabel {
				iMatches, err = system.Database.FindPackageLabel(args[0])
			} else if searchWithLabelMatch {
				iMatches, err = system.Database.FindPackageLabelMatch(args[0])
			} else {
				iMatches, err = system.Database.FindPackageMatch(args[0])
			}

			if err != nil {
				Fatal("Error: " + err.Error())
			}

			for _, pack := range iMatches {
				Info(":package:", pack.GetCategory(), pack.GetName(), pack.GetVersion())
			}

		}

	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	searchCmd.Flags().String("system-dbpath", path, "System db path")
	searchCmd.Flags().String("system-target", path, "System rootpath")
	searchCmd.Flags().Bool("installed", false, "Search between system packages")
	searchCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
	searchCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	searchCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	searchCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	searchCmd.Flags().Bool("by-label", false, "Search packages through label")
	searchCmd.Flags().Bool("by-label-regex", false, "Search packages through label regex")
	RootCmd.AddCommand(searchCmd)
}
