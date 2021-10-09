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
	installer "github.com/mudler/luet/pkg/installer"
	"github.com/mudler/luet/pkg/solver"

	helpers "github.com/mudler/luet/cmd/helpers"
	"github.com/mudler/luet/cmd/util"
	. "github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/logger"
	pkg "github.com/mudler/luet/pkg/package"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install <pkg1> <pkg2> ...",
	Short: "Install a package",
	Long: `Installs one or more packages without asking questions:

	$ luet install -y utils/busybox utils/yq ...
	
To install only deps of a package:
	
	$ luet install --onlydeps utils/busybox ...
	
To not install deps of a package:
	
	$ luet install --nodeps utils/busybox ...

To force install a package:
	
	$ luet install --force utils/busybox ...
`,
	Aliases: []string{"i"},
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
		util.BindSolverFlags(cmd)
		LuetCfg.Viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		LuetCfg.Viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		LuetCfg.Viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		LuetCfg.Viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var toInstall pkg.Packages

		for _, a := range args {
			pack, err := helpers.ParsePackageStr(a)
			if err != nil {
				Fatal("Invalid package string ", a, ": ", err.Error())
			}
			toInstall = append(toInstall, pack)
		}

		force := LuetCfg.Viper.GetBool("force")
		nodeps := LuetCfg.Viper.GetBool("nodeps")
		onlydeps := LuetCfg.Viper.GetBool("onlydeps")
		concurrent, _ := cmd.Flags().GetBool("solver-concurrent")
		yes := LuetCfg.Viper.GetBool("yes")
		downloadOnly, _ := cmd.Flags().GetBool("download-only")
		finalizerEnvs, _ := cmd.Flags().GetStringArray("finalizer-env")

		util.SetSystemConfig()
		util.SetSolverConfig()

		if concurrent {
			LuetCfg.GetSolverOptions().Implementation = solver.ParallelSimple
		} else {
			LuetCfg.GetSolverOptions().Implementation = solver.SingleCoreSimple
		}

		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())
		repos := installer.SystemRepositories(LuetCfg)

		// Load config protect configs
		installer.LoadConfigProtectConfs(LuetCfg)

		// Load finalizer runtime environments
		err := util.SetCliFinalizerEnvs(finalizerEnvs)
		if err != nil {
			Fatal(err.Error())
		}

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 LuetCfg.GetGeneral().Concurrency,
			SolverOptions:               *LuetCfg.GetSolverOptions(),
			NoDeps:                      nodeps,
			Force:                       force,
			OnlyDeps:                    onlydeps,
			PreserveSystemEssentialData: true,
			DownloadOnly:                downloadOnly,
			Ask:                         !yes,
			Relaxed:                     relax,
		})
		inst.Repositories(repos)

		system := &installer.System{Database: LuetCfg.GetSystemDB(), Target: LuetCfg.GetSystem().Rootfs}
		err = inst.Install(toInstall, system)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {
	installCmd.Flags().String("system-dbpath", "", "System db path")
	installCmd.Flags().String("system-target", "", "System rootpath")
	installCmd.Flags().String("system-engine", "", "System DB engine")

	installCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
	installCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	installCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	installCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	installCmd.Flags().Bool("nodeps", false, "Don't consider package dependencies (harmful!)")
	installCmd.Flags().Bool("relax", false, "Relax installation constraints")

	installCmd.Flags().Bool("onlydeps", false, "Consider **only** package dependencies")
	installCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")
	installCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	installCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	installCmd.Flags().Bool("download-only", false, "Download only")
	installCmd.Flags().StringArray("finalizer-env", []string{},
		"Set finalizer environment in the format key=value.")

	RootCmd.AddCommand(installCmd)
}
