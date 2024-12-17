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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/mudler/luet/pkg/helpers"

	"github.com/spf13/cobra"
)

func NewRepoAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [OPTIONS] https://..../something.yaml /local/file.yaml",
		Short: "Add a repository to the system",
		Args:  cobra.ExactArgs(1),
		Long: `
Adds a repository to the system. URLs, local files or inline repo can be specified, examples:

# URL/File:

 luet repo add /path/to/file

 luet repo add https://....

 luet repo add ... --name "foo"

# Inline (provided you have $PASSWORD environent variable set):

 luet repo add testfo --description "Bar" --url "FOZZ" --type "ff" --username "user" --passwd $(echo "$PASSWORD")

 `,
		Run: func(cmd *cobra.Command, args []string) {

			uri := args[0]
			d, _ := cmd.Flags().GetString("dir")
			yes, _ := cmd.Flags().GetBool("yes")

			desc, _ := cmd.Flags().GetString("description")
			t, _ := cmd.Flags().GetString("type")
			url, _ := cmd.Flags().GetString("url")
			ref, _ := cmd.Flags().GetString("reference")
			prio, _ := cmd.Flags().GetInt("priority")
			username, _ := cmd.Flags().GetString("username")
			passwd, _ := cmd.Flags().GetString("passwd")

			if len(util.DefaultContext.Config.RepositoriesConfDir) == 0 && d == "" {
				util.DefaultContext.Fatal("No repository dirs defined")
				return
			}
			if d == "" {
				d = util.DefaultContext.Config.RepositoriesConfDir[0]
			}

			var r *types.LuetRepository
			str, err := helpers.GetURI(uri)
			if err != nil {
				r = &types.LuetRepository{
					Enable:      true,
					Cached:      true,
					Name:        uri,
					Description: desc,
					ReferenceID: ref,
					Type:        t,
					Urls:        []string{url},
					Priority:    prio,
					Authentication: map[string]string{
						"username": username,
						"password": passwd,
					},
				}
			} else {
				r, err = types.LoadRepository([]byte(str))
				if err != nil {
					util.DefaultContext.Fatal(err)
				}
				if desc != "" {
					r.Description = desc
				}
				if ref != "" {
					r.ReferenceID = ref
				}
				if t != "" {
					r.Type = t
				}
				if url != "" {
					r.Urls = []string{url}
				}
				if prio != 0 {
					r.Priority = prio
				}
				if username != "" && passwd != "" {
					r.Authentication = map[string]string{
						"username": username,
						"password": passwd,
					}
				}
			}

			file := filepath.Join(util.DefaultContext.Config.System.Rootfs, d, fmt.Sprintf("%s.yaml", r.Name))

			b, err := yaml.Marshal(r)
			if err != nil {
				util.DefaultContext.Fatal(err)
			}

			util.DefaultContext.Infof("Adding repository to the sytem as %s", file)
			fmt.Println(string(b))
			util.DefaultContext.Info(r.String())

			if !yes && !util.DefaultContext.Ask() {
				util.DefaultContext.Info("Aborted by user")
				return
			}

			if err := ioutil.WriteFile(file, b, os.ModePerm); err != nil {
				util.DefaultContext.Fatal(err)
			}
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "Assume yes to questions")
	cmd.Flags().StringP("dir", "o", "", "Folder to write to")
	cmd.Flags().String("description", "", "Repository description")
	cmd.Flags().String("type", "", "Repository type")
	cmd.Flags().String("url", "", "Repository URL")
	cmd.Flags().String("reference", "", "Repository Reference ID")
	cmd.Flags().IntP("priority", "p", 99, "repository prio")
	cmd.Flags().String("username", "", "repository username")
	cmd.Flags().String("passwd", "", "repository password")
	return cmd
}
