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
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mudler/luet/pkg/api/core/types"
	installer "github.com/mudler/luet/pkg/installer"
	pkg "github.com/mudler/luet/pkg/package"
	"github.com/mudler/luet/pkg/solver"

	"github.com/mudler/luet/cmd/util"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var osCheckCmd = &cobra.Command{
	Use:   "oscheck",
	Short: "Checks packages integrity",
	Long: `List packages that are installed in the system which files are missing in the system.

	$ luet oscheck
	
To reinstall packages in the list:
	
	$ luet oscheck --reinstall
`,
	Aliases: []string{"i"},
	PreRun: func(cmd *cobra.Command, args []string) {
		util.BindSystemFlags(cmd)
		util.BindSolverFlags(cmd)
		viper.BindPFlag("onlydeps", cmd.Flags().Lookup("onlydeps"))
		viper.BindPFlag("nodeps", cmd.Flags().Lookup("nodeps"))
		viper.BindPFlag("force", cmd.Flags().Lookup("force"))
		viper.BindPFlag("yes", cmd.Flags().Lookup("yes"))
	},
	Run: func(cmd *cobra.Command, args []string) {

		force := viper.GetBool("force")
		onlydeps := viper.GetBool("onlydeps")
		yes := viper.GetBool("yes")

		downloadOnly, _ := cmd.Flags().GetBool("download-only")
		util.SetSystemConfig(util.DefaultContext)

		system := &installer.System{Database: util.DefaultContext.Config.GetSystemDB(), Target: util.DefaultContext.Config.GetSystem().Rootfs}
		packs := system.OSCheck()
		if !util.DefaultContext.Config.General.Quiet {
			if len(packs) == 0 {
				util.DefaultContext.Success("All good!")
				os.Exit(0)
			} else {
				util.DefaultContext.Info("Following packages are missing files or are incomplete:")
				for _, p := range packs {
					util.DefaultContext.Info(p.HumanReadableString())
				}
			}
		} else {
			var s []string
			for _, p := range packs {
				s = append(s, p.HumanReadableString())
			}
			fmt.Println(strings.Join(s, " "))
		}

		reinstall, _ := cmd.Flags().GetBool("reinstall")
		if reinstall {

			// Strip version for reinstall
			toInstall := pkg.Packages{}
			for _, p := range packs {
				new := p.Clone()
				new.SetVersion(">=0")
				toInstall = append(toInstall, new)
			}

			util.SetSolverConfig(util.DefaultContext)

			util.DefaultContext.Config.GetSolverOptions().Implementation = solver.SingleCoreSimple

			util.DefaultContext.Debug("Solver", util.DefaultContext.Config.GetSolverOptions().CompactString())

			// Load config protect configs
			util.DefaultContext.Config.LoadConfigProtect(util.DefaultContext)

			inst := installer.NewLuetInstaller(installer.LuetInstallerOptions{
				Concurrency:                 util.DefaultContext.Config.GetGeneral().Concurrency,
				SolverOptions:               *util.DefaultContext.Config.GetSolverOptions(),
				NoDeps:                      true,
				Force:                       force,
				OnlyDeps:                    onlydeps,
				PreserveSystemEssentialData: true,
				Ask:                         !yes,
				DownloadOnly:                downloadOnly,
				Context:                     util.DefaultContext,
				PackageRepositories:         util.DefaultContext.Config.SystemRepositories,
			})

			err := inst.Swap(packs, toInstall, system)
			if err != nil {
				util.DefaultContext.Fatal("Error: " + err.Error())
			}
		}
	},
}

func init() {
	osCheckCmd.Flags().String("system-dbpath", "", "System db path")
	osCheckCmd.Flags().String("system-target", "", "System rootpath")
	osCheckCmd.Flags().String("system-engine", "", "System DB engine")
	osCheckCmd.Flags().Bool("reinstall", false, "reinstall")

	osCheckCmd.Flags().String("solver-type", "", "Solver strategy ( Defaults none, available: "+types.AvailableResolvers+" )")
	osCheckCmd.Flags().Float32("solver-rate", 0.7, "Solver learning rate")
	osCheckCmd.Flags().Float32("solver-discount", 1.0, "Solver discount rate")
	osCheckCmd.Flags().Int("solver-attempts", 9000, "Solver maximum attempts")
	osCheckCmd.Flags().Bool("onlydeps", false, "Consider **only** package dependencies")
	osCheckCmd.Flags().Bool("force", false, "Skip errors and keep going (potentially harmful)")
	osCheckCmd.Flags().Bool("solver-concurrent", false, "Use concurrent solver (experimental)")
	osCheckCmd.Flags().BoolP("yes", "y", false, "Don't ask questions")
	osCheckCmd.Flags().Bool("download-only", false, "Download only")

	RootCmd.AddCommand(osCheckCmd)
}
