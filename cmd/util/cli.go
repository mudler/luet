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

package util

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/marcsauter/single"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/installer"
)

var DefaultContext = types.NewContext()

var lockedCommands = []string{"install", "uninstall", "upgrade"}
var bannerCommands = []string{"install", "build", "uninstall", "upgrade"}

func BindSystemFlags(cmd *cobra.Command) {
	viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
	viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
	viper.BindPFlag("system.database_engine", cmd.Flags().Lookup("system-engine"))
}

func BindSolverFlags(cmd *cobra.Command) {
	viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
	viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
	viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
	viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
}

func BindValuesFlags(cmd *cobra.Command) {
	viper.BindPFlag("values", cmd.Flags().Lookup("values"))
}

func ValuesFlags() []string {
	return viper.GetStringSlice("values")
}

func SetSystemConfig(ctx *types.Context) {
	dbpath := viper.GetString("system.database_path")
	rootfs := viper.GetString("system.rootfs")
	engine := viper.GetString("system.database_engine")

	ctx.Config.System.DatabaseEngine = engine
	ctx.Config.System.DatabasePath = dbpath
	ctx.Config.System.SetRootFS(rootfs)
}

func SetSolverConfig(ctx *types.Context) (c *types.LuetSolverOptions) {
	stype := viper.GetString("solver.type")
	discount := viper.GetFloat64("solver.discount")
	rate := viper.GetFloat64("solver.rate")
	attempts := viper.GetInt("solver.max_attempts")

	ctx.Config.GetSolverOptions().Type = stype
	ctx.Config.GetSolverOptions().LearnRate = float32(rate)
	ctx.Config.GetSolverOptions().Discount = float32(discount)
	ctx.Config.GetSolverOptions().MaxAttempts = attempts

	return &types.LuetSolverOptions{
		Type:        stype,
		LearnRate:   float32(rate),
		Discount:    float32(discount),
		MaxAttempts: attempts,
	}
}

func SetCliFinalizerEnvs(ctx *types.Context, finalizerEnvs []string) error {
	if len(finalizerEnvs) > 0 {
		for _, v := range finalizerEnvs {
			idx := strings.Index(v, "=")
			if idx < 0 {
				return errors.New("Found invalid runtime finalizer environment: " + v)
			}

			ctx.Config.SetFinalizerEnv(v[0:idx], v[idx+1:])
		}

	}

	return nil
}

// TemplateFolders returns the default folders which holds shared template between packages in a given tree path
func TemplateFolders(ctx *types.Context, fromRepo bool, treePaths []string) []string {
	templateFolders := []string{}
	for _, t := range treePaths {
		templateFolders = append(templateFolders, filepath.Join(t, "templates"))
	}
	if fromRepo {
		for _, s := range installer.SystemRepositories(ctx.Config.SystemRepositories) {
			templateFolders = append(templateFolders, filepath.Join(s.TreePath, "templates"))
		}
	}
	return templateFolders
}

func IntroScreen() {
	luetLogo, _ := pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("LU", pterm.NewStyle(pterm.FgLightMagenta)),
		pterm.NewLettersFromStringWithStyle("ET", pterm.NewStyle(pterm.FgLightBlue))).
		Srender()

	pterm.DefaultCenter.Print(luetLogo)

	pterm.DefaultCenter.Print(pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).WithMargin(10).Sprint("Luet - 0-deps container-based package manager"))
}

func HandleLock(c *types.Context) {
	if os.Getenv("LUET_NOLOCK") != "true" {
		if len(os.Args) > 1 {
			for _, lockedCmd := range lockedCommands {
				if os.Args[1] == lockedCmd {
					s := single.New("luet")
					if err := s.CheckLock(); err != nil && err == single.ErrAlreadyRunning {
						c.Fatal("another instance of the app is already running, exiting")
					} else if err != nil {
						// Another error occurred, might be worth handling it as well
						c.Fatal("failed to acquire exclusive app lock:", err.Error())
					}
					defer s.TryUnlock()
					break
				}
			}
		}
	}
}

func DisplayVersionBanner(c *types.Context, banner func(), version func() string, license []string) {
	display := false
	if len(os.Args) > 1 {
		for _, c := range bannerCommands {
			if os.Args[1] == c {
				display = true
			}
		}
	}
	if display {
		if c.Config.General.Quiet {
			pterm.Info.Printf("Luet %s\n", version())
			pterm.Info.Println(strings.Join(license, "\n"))
		} else {
			banner()
			pterm.DefaultCenter.Print(version())
			for _, l := range license {
				pterm.DefaultCenter.Print(l)
			}
		}
	}
}
