// Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
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

var replaceCmd = &cobra.Command{
	Use:     "replace <pkg1> <pkg2> --for <pkg3> --for <pkg4> ...",
	Short:   "replace a set of packages",
	Aliases: []string{"r"},
	Long: `Replaces one or a group of packages without asking questions:

	$ luet replace -y system/busybox ... --for shells/bash --for system/coreutils ...
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
		util.BindSolverFlags(cmd)
		LuetCfg.Viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		LuetCfg.Viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		LuetCfg.Viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		LuetCfg.Viper.BindPFlag("for", cmd.Flags().Lookup("for"))

		LuetCfg.Viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
	},
	Run: func(cmd *cobra.Command, args []string) {
		var toUninstall pkg.Packages
		var toAdd pkg.Packages

		f := LuetCfg.Viper.GetStringSlice("for")
		force := LuetCfg.Viper.GetBool("force")
		nodeps := LuetCfg.Viper.GetBool("nodeps")
		onlydeps := LuetCfg.Viper.GetBool("onlydeps")
		yes := LuetCfg.Viper.GetBool("yes")
		downloadOnly, _ := cmd.Flags().GetBool("download-only")

		util.SetSystemConfig()
		util.SetSolverConfig()
		for _, a := range args {
			pack, err := helpers.ParsePackageStr(a)
			if err != nil {
				Fatal("Invalid package string ", a, ": ", err.Error())
			}
			toUninstall = append(toUninstall, pack)
		}

		for _, a := range f {
			pack, err := helpers.ParsePackageStr(a)
			if err != nil {
				Fatal("Invalid package string ", a, ": ", err.Error())
			}
			toAdd = append(toAdd, pack)
		}

		// This shouldn't be necessary, but we need to unmarshal the repositories to a concrete struct, thus we need to port them back to the Repositories type
		repos := installer.Repositories{}
		for _, repo := range LuetCfg.SystemRepositories {
			if !repo.Enable {
				continue
			}
			r := installer.NewSystemRepository(repo)
			repos = append(repos, r)
		}

		LuetCfg.GetSolverOptions().Implementation = solver.SingleCoreSimple

		Debug("Solver", LuetCfg.GetSolverOptions().CompactString())

		// Load config protect configs
		installer.LoadConfigProtectConfs(LuetCfg)

		inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
			Concurrency:                 LuetCfg.GetGeneral().Concurrency,
			SolverOptions:               *LuetCfg.GetSolverOptions(),
			NoDeps:                      nodeps,
			Force:                       force,
			OnlyDeps:                    onlydeps,
			PreserveSystemEssentialData: true,
			Ask:                         !yes,
			DownloadOnly:                downloadOnly,
		})
		inst.Repositories(repos)

		system := &installer.System{Database: LuetCfg.GetSystemDB(), Target: LuetCfg.GetSystem().Rootfs}
		err := inst.Swap(toUninstall, toAdd, system)
		if err != nil {
			Fatal("Error: " + err.Error())
		}
	},
}

func init() {

	replaceCmd.Flags().String("system-dbpath", "", "System db path")
	replaceCmd.Flags().String("system-target", "", "System rootpath")
	replaceCmd.Flags().String("system-engine", "", "System DB engine")

	replaceCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+AvailableResolvers+" )")
	replaceCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	replaceCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	replaceCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	replaceCmd.Flags().Bool("nodeps", false, "Don't consider package dependencies (harmful!)")
	replaceCmd.Flags().Bool("onlydeps", false, "Consider **only** package dependencies")
	replaceCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")
	replaceCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	replaceCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	replaceCmd.Flags().StringSlice("for", []string{}, "Packages that has to be installed in place of others")
	replaceCmd.Flags().Bool("download-only", false, "Download only")

	RootCmd.AddCommand(replaceCmd)
}
