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
	. "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"
	. "github.com/mudler/luet/pkg/logger"
	"github.com/mudler/luet/pkg/solver"

	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:     "upgrade",
	Short:   "Upgrades the system",
	Aliases: []string{"u"},
	PreRun: func(cmd *cobra.Command, args []string) {
		LuetCfg.Viper.BindPFlag("system.database_path", installCmd.Flags().Lookup("system-dbpath"))
		LuetCfg.Viper.BindPFlag("system.rootfs", installCmd.Flags().Lookup("system-target"))
		LuetCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
		LuetCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
		LuetCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
		LuetCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
		LuetCfg.Viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		LuetCfg.Viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
		LuetCfg.Viper.BindPFlag("system.database_engine", cmd.Flags().Lookup("system-engine"))
	},
	Long: `Upgrades packages in parallel`,
	Run: func(cmd *cobra.Command, args []string) {

		repos := installer.Repositories{}
		for _, repo := range LuetCfg.SystemRepositories {
			if !repo.Enable {
				continue
			}

			r := installer.NewSystemRepository(repo)
			repos = append(repos, r)
		}

		stype := LuetCfg.Viper.GetString("solver.type")
		discount := LuetCfg.Viper.GetFloat64("solver.discount")
		rate := LuetCfg.Viper.GetFloat64("solver.rate")
		attempts := LuetCfg.Viper.GetInt("solver.max_attempts")
		force := LuetCfg.Viper.GetBool("force")
		nodeps, _ := cmd.Flags().GetBool("nodeps")
		full, _ := cmd.Flags().GetBool("full")
		universe, _ := cmd.Flags().GetBool("universe")
		clean, _ := cmd.Flags().GetBool("clean")
		sync, _ := cmd.Flags().GetBool("sync")
		concurrent, _ := cmd.Flags().GetBool("solver-concurrent")
		yes := LuetCfg.Viper.GetBool("yes")
		dbpath := LuetCfg.Viper.GetString("system.database_path")
		rootfs := LuetCfg.Viper.GetString("system.rootfs")
		engine := LuetCfg.Viper.GetString("system.database_engine")
		downloadOnly, _ := cmd.Flags().GetBool("download-only")

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

		Debug("Solver", LuetCfg.GetSolverOptions().String())

		// Load config protect configs
		installer.LoadConfigProtectConfs(LuetCfg)

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 LuetCfg.GetGeneral().Concurrency,
			SolverOptions:               *LuetCfg.GetSolverOptions(),
			Force:                       force,
			FullUninstall:               full,
			NoDeps:                      nodeps,
			SolverUpgrade:               universe,
			RemoveUnavailableOnUpgrade:  clean,
			UpgradeNewRevisions:         sync,
			PreserveSystemEssentialData: true,
			Ask:                         !yes,
			DownloadOnly:                downloadOnly,
		})
		inst.Repositories(repos)

		system := &installer.System{Database: LuetCfg.GetSystemDB(), Target: LuetCfg.GetSystem().Rootfs}
		if err := inst.Upgrade(system); err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {
	upgradeCmd.Flags().String("system-dbpath", "", "System db path")
	upgradeCmd.Flags().String("system-target", "", "System rootpath")
	upgradeCmd.Flags().String("system-engine", "", "System DB engine")

	upgradeCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
	upgradeCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	upgradeCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	upgradeCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	upgradeCmd.Flags().Bool("force", false, "Force upgrade by ignoring errors")
	upgradeCmd.Flags().Bool("nodeps", false, "Don't consider package dependencies (harmful! overrides checkconflicts and full!)")
	upgradeCmd.Flags().Bool("full", false, "Attempts to remove as much packages as possible which aren't required (slow)")
	upgradeCmd.Flags().Bool("universe", false, "Use ONLY the SAT solver to compute upgrades (experimental)")
	upgradeCmd.Flags().Bool("clean", false, "Try to drop removed packages (experimental, only when --universe is enabled)")
	upgradeCmd.Flags().Bool("sync", false, "Upgrade packages with new revisions (experimental)")
	upgradeCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	upgradeCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	upgradeCmd.Flags().Bool("download-only", false, "Download only")

	RootCmd.AddCommand(upgradeCmd)
}
