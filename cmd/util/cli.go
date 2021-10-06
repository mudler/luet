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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mudler/luet/pkg/config"
	. "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/installer"
)

func BindSystemFlags(cmd *cobra.Command) {
	LuetCfg.Viper.BindPFlag("system.database_path", cmd.Flags().Lookup("system-dbpath"))
	LuetCfg.Viper.BindPFlag("system.rootfs", cmd.Flags().Lookup("system-target"))
	LuetCfg.Viper.BindPFlag("system.database_engine", cmd.Flags().Lookup("system-engine"))
}

func BindSolverFlags(cmd *cobra.Command) {
	LuetCfg.Viper.BindPFlag("solver.type", cmd.Flags().Lookup("solver-type"))
	LuetCfg.Viper.BindPFlag("solver.discount", cmd.Flags().Lookup("solver-discount"))
	LuetCfg.Viper.BindPFlag("solver.rate", cmd.Flags().Lookup("solver-rate"))
	LuetCfg.Viper.BindPFlag("solver.max_attempts", cmd.Flags().Lookup("solver-attempts"))
}

func BindValuesFlags(cmd *cobra.Command) {
	LuetCfg.Viper.BindPFlag("values", cmd.Flags().Lookup("values"))
}

func ValuesFlags() []string {
	return LuetCfg.Viper.GetStringSlice("values")
}

func SetSystemConfig() {
	dbpath := LuetCfg.Viper.GetString("system.database_path")
	rootfs := LuetCfg.Viper.GetString("system.rootfs")
	engine := LuetCfg.Viper.GetString("system.database_engine")

	LuetCfg.System.DatabaseEngine = engine
	LuetCfg.System.DatabasePath = dbpath
	LuetCfg.System.SetRootFS(rootfs)
}

func SetSolverConfig() (c *config.LuetSolverOptions) {
	stype := LuetCfg.Viper.GetString("solver.type")
	discount := LuetCfg.Viper.GetFloat64("solver.discount")
	rate := LuetCfg.Viper.GetFloat64("solver.rate")
	attempts := LuetCfg.Viper.GetInt("solver.max_attempts")

	LuetCfg.GetSolverOptions().Type = stype
	LuetCfg.GetSolverOptions().LearnRate = float32(rate)
	LuetCfg.GetSolverOptions().Discount = float32(discount)
	LuetCfg.GetSolverOptions().MaxAttempts = attempts

	return &config.LuetSolverOptions{
		Type:        stype,
		LearnRate:   float32(rate),
		Discount:    float32(discount),
		MaxAttempts: attempts,
	}
}

func SetCliFinalizerEnvs(finalizerEnvs []string) error {
	if len(finalizerEnvs) > 0 {
		for _, v := range finalizerEnvs {
			idx := strings.Index(v, "=")
			if idx < 0 {
				return errors.New("Found invalid runtime finalizer environment: " + v)
			}

			LuetCfg.SetFinalizerEnv(v[0:idx], v[idx+1:])
		}

	}

	return nil
}

// TemplateFolders returns the default folders which holds shared template between packages in a given tree path
func TemplateFolders(fromRepo bool, treePaths []string) []string {
	templateFolders := []string{}
	for _, t := range treePaths {
		templateFolders = append(templateFolders, filepath.Join(t, "templates"))
	}
	if fromRepo {
		for _, s := range installer.SystemRepositories(LuetCfg) {
			templateFolders = append(templateFolders, filepath.Join(s.TreePath, "templates"))
		}
	}
	return templateFolders
}
