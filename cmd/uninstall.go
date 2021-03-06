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
	helpers "github.com/mudler/luet/cmd/helpers"
	. "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:     "uninstall <pkg> <pkg2> ...",
	Short:   "Uninstall a package or a list of packages",
	Long:    `Uninstall packages`,
	Aliases: []string{"rm", "un"},
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
		LuetCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
		LuetCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
		LuetCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
		LuetCfg.Viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		LuetCfg.Viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		LuetCfg.Viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
		LuetCfg.Viper.BindPFlag("system.database_engine", cmd.Flags().Lookup("system-engine"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		toRemove := []pkg.Package{}
		for _, a := range args {

			pack, err := helpers.ParsePackageStr(a)
			if err != nil {
				Fatal("Invalid package string ", a, ": ", err.Error())
			}
			toRemove = append(toRemove, pack)
		}

		stype := LuetCfg.Viper.GetString("solver.type")
		discount := LuetCfg.Viper.GetFloat64("solver.discount")
		rate := LuetCfg.Viper.GetFloat64("solver.rate")
		attempts := LuetCfg.Viper.GetInt("solver.max_attempts")
		force := LuetCfg.Viper.GetBool("force")
		nodeps, _ := cmd.Flags().GetBool("nodeps")
		full, _ := cmd.Flags().GetBool("full")
		checkconflicts, _ := cmd.Flags().GetBool("conflictscheck")
		fullClean, _ := cmd.Flags().GetBool("full-clean")
		concurrent, _ := cmd.Flags().GetBool("solver-concurrent")
		yes := LuetCfg.Viper.GetBool("yes")
		dbpath := LuetCfg.Viper.GetString("system.database_path")
		rootfs := LuetCfg.Viper.GetString("system.rootfs")
		engine := LuetCfg.Viper.GetString("system.database_engine")

		LuetCfg.System.DatabaseEngine = engine
		LuetCfg.System.DatabasePath = dbpath
		LuetCfg.System.Rootfs = rootfs

		LuetCfg.GetSolverOptions().Type = stype
		LuetCfg.GetSolverOptions().LearnRate = float32(rate)
		LuetCfg.GetSolverOptions().Discount = float32(discount)
		LuetCfg.GetSolverOptions().MaxAttempts = attempts
		if concurrent {
			LuetCfg.GetSolverOptions().Implementation = solver.ParallelSimple
		} else {
			LuetCfg.GetSolverOptions().Implementation = solver.SingleCoreSimple
		}
		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())

		// Load config protect configs
		installer.LoadConfigProtectConfs(LuetCfg)

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 LuetCfg.GetGeneral().Concurrency,
			SolverOptions:               *LuetCfg.GetSolverOptions(),
			NoDeps:                      nodeps,
			Force:                       force,
			FullUninstall:               full,
			FullCleanUninstall:          fullClean,
			CheckConflicts:              checkconflicts,
			Ask:                         !yes,
			PreserveSystemEssentialData: true,
		})

		system := &installer.System{Database: LuetCfg.GetSystemDB(), Target: LuetCfg.GetSystem().Rootfs}

		if err := inst.Uninstall(system, toRemove...); err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {

	uninstallCmd.Flags().String("system-dbpath", "", "System db path")
	uninstallCmd.Flags().String("system-target", "", "System rootpath")
	uninstallCmd.Flags().String("system-engine", "", "System DB engine")

	uninstallCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
	uninstallCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	uninstallCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	uninstallCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	uninstallCmd.Flags().Bool("nodeps", false, "Don't consider package dependencies (harmful! overrides checkconflicts and full!)")
	uninstallCmd.Flags().Bool("force", false, "Force uninstall")
	uninstallCmd.Flags().Bool("full", false, "Attempts to remove as much packages as possible which aren't required (slow)")
	uninstallCmd.Flags().Bool("conflictscheck", true, "Check if the package marked for deletion is required by other packages")
	uninstallCmd.Flags().Bool("full-clean", false, "(experimental) Uninstall packages and all the other deps/revdeps of it.")
	uninstallCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	uninstallCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")

	RootCmd.AddCommand(uninstallCmd)
}
