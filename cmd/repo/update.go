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

package cmd_repo

import (
	"github.com/mudler/luet/cmd/util"
	installer "github.com/mudler/luet/pkg/installer"

	"github.com/spf13/cobra"
)

func NewRepoUpdateCommand() *cobra.Command {
	var ans = &cobra.Command{
		Use:   "update [repo1] [repo2] [OPTIONS]",
		Short: "Update a specific cached repository or all cached repositories.",
		Example: `
# Update all cached repositories:
$> luet repo update

# Update only repo1 and repo2
$> luet repo update repo1 repo2
`,
		Aliases: []string{"up"},
		PreRun: func(cmd *cobra.Command, args []string) {
		},
		Run: func(cmd *cobra.Command, args []string) {

			ignore, _ := cmd.Flags().GetBool("ignore-errors")
			force, _ := cmd.Flags().GetBool("force")

			if len(args) > 0 {
				for _, rname := range args {
					repo, err := util.DefaultContext.Config.GetSystemRepository(rname)
					if err != nil && !ignore {
						util.DefaultContext.Fatal(err.Error())
					} else if err != nil {
						continue
					}

					r := installer.NewSystemRepository(*repo)
					_, err = r.Sync(util.DefaultContext, force)
					if err != nil && !ignore {
						util.DefaultContext.Fatal("Error on sync repository " + rname + ": " + err.Error())
					}
				}

			} else {
				for _, repo := range util.DefaultContext.Config.SystemRepositories {
					if repo.Cached && repo.Enable {
						r := installer.NewSystemRepository(repo)
						_, err := r.Sync(util.DefaultContext, force)
						if err != nil && !ignore {
							util.DefaultContext.Fatal("Error on sync repository " + r.GetName() + ": " + err.Error())
						}
					}
				}
			}
		},
	}

	ans.Flags().BoolP("ignore-errors", "i", false, "Ignore errors on sync repositories.")
	ans.Flags().BoolP("force", "f", false, "Force resync.")

	return ans
}
