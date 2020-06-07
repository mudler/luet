// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
//                  Daniele Rondina <geaaru@sabayonlinux.org>
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

	config "github.com/mudler/luet/pkg/config"
	installer "github.com/mudler/luet/pkg/installer"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "Print config",
	Long:    `Show luet configuration`,
	Aliases: []string{"c"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.LuetCfg.GetLogging())
		fmt.Println(config.LuetCfg.GetGeneral())
		fmt.Println(config.LuetCfg.GetSystem())
		if len(config.LuetCfg.CacheRepositories) > 0 {
			fmt.Println("repetitors:")
			for _, r := range config.LuetCfg.CacheRepositories {
				fmt.Println("  - ", r.String())
			}
		}
		if len(config.LuetCfg.SystemRepositories) > 0 {
			fmt.Println("repositories:")
			for _, r := range config.LuetCfg.SystemRepositories {
				fmt.Println("  - ", r.String())
			}
		}

		if len(config.LuetCfg.RepositoriesConfDir) > 0 {
			fmt.Println("repos_confdir:")
			for _, dir := range config.LuetCfg.RepositoriesConfDir {
				fmt.Println("  - ", dir)
			}
		}

		if len(config.LuetCfg.ConfigProtectConfDir) > 0 {

			// Load config protect configs
			installer.LoadConfigProtectConfs(config.LuetCfg)

			fmt.Println("config_protect_confdir:")
			for _, dir := range config.LuetCfg.ConfigProtectConfDir {
				fmt.Println("  - ", dir)
			}

			if len(config.LuetCfg.GetConfigProtectConfFiles()) > 0 {
				fmt.Println("protect_conf_files:")
				for _, file := range config.LuetCfg.GetConfigProtectConfFiles() {
					fmt.Println("  - ", file.String())
				}
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(configCmd)
}
