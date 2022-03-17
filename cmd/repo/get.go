// Copyright © 2022 Ettore Di Giacinto <mudler@luet.io>
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

package cmd_repo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/cmd/util"

	"github.com/spf13/cobra"
)

func NewRepoGetCommand() *cobra.Command {
	var ans = &cobra.Command{
		Use:   "get [OPTIONS] name",
		Short: "get repository in the system",
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
		},
		Run: func(cmd *cobra.Command, args []string) {
			o, _ := cmd.Flags().GetString("output")

			for _, repo := range util.DefaultContext.Config.SystemRepositories {
				if repo.Name != args[0] {
					continue
				}

				switch strings.ToLower(o) {
				case "json":
					b, _ := json.Marshal(repo)
					fmt.Println(string(b))
				case "yaml":
					b, _ := yaml.Marshal(repo)
					fmt.Println(string(b))
				default:
					fmt.Println(repo)
				}
				break
			}
		},
	}

	ans.Flags().StringP("output", "o", "", "output format (json, yaml, text)")

	return ans
}
