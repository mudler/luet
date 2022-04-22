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
	"fmt"
	"os"
	"strings"

	"github.com/marcsauter/single"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mudler/luet/pkg/api/core/context"
	"github.com/mudler/luet/pkg/api/core/template"
	"github.com/mudler/luet/pkg/installer"
)

var lockedCommands = []string{"install", "uninstall", "upgrade"}
var bannerCommands = []string{"install", "build", "uninstall", "upgrade"}

func BindValuesFlags(cmd *cobra.Command) {
	viper.BindPFlag("values", cmd.Flags().Lookup("values"))
}

func ValuesFlags() []string {
	return viper.GetStringSlice("values")
}

// TemplateFolders returns the default folders which holds shared template between packages in a given tree path
func TemplateFolders(ctx *context.Context, i installer.BuildTreeResult, treePaths []string) []string {
	templateFolders := []string{}
	for _, t := range treePaths {
		templateFolders = append(templateFolders, template.FindPossibleTemplatesDir(t)...)
	}
	for _, r := range i.TemplatesDir {
		templateFolders = append(templateFolders, r...)
	}

	return templateFolders
}

func HandleLock() {
	if os.Getenv("LUET_NOLOCK") == "true" {
		return
	}

	if len(os.Args) == 0 {
		return
	}

	for _, lockedCmd := range lockedCommands {
		if os.Args[1] == lockedCmd {
			s := single.New("luet")
			if err := s.CheckLock(); err != nil && err == single.ErrAlreadyRunning {
				fmt.Println("another instance of the app is already running, exiting")
				os.Exit(1)
			} else if err != nil {
				// Another error occurred, might be worth handling it as well
				fmt.Println("failed to acquire exclusive app lock:", err.Error())
				os.Exit(1)
			}
			defer s.TryUnlock()
			break
		}
	}
}

func DisplayVersionBanner(c *context.Context, version func() string, license []string) {
	display := false
	if len(os.Args) > 1 {
		for _, c := range bannerCommands {
			if os.Args[1] == c {
				display = true
			}
		}
	}
	if display {
		pterm.Info.Printf("Luet %s\n", version())
		pterm.Info.Println(strings.Join(license, "\n"))
	}
}
