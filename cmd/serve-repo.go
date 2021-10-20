// Copyright © 2020 Ettore Di Giacinto <mudler@gentoo.org>
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
	"net/http"
	"os"

	"github.com/mudler/luet/cmd/util"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serverepoCmd = &cobra.Command{
	Use:   "serve-repo",
	Short: "Embedded micro-http server",
	Long:  `Embedded mini http server for serving local repositories`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("dir", cmd.Flags().Lookup("dir"))
		viper.BindPFlag("address", cmd.Flags().Lookup("address"))
		viper.BindPFlag("port", cmd.Flags().Lookup("port"))
	},
	Run: func(cmd *cobra.Command, args []string) {

		dir := viper.GetString("dir")
		port := viper.GetString("port")
		address := viper.GetString("address")

		http.Handle("/", http.FileServer(http.Dir(dir)))

		util.DefaultContext.Info("Serving ", dir, " on HTTP port: ", port)
		util.DefaultContext.Fatal(http.ListenAndServe(address+":"+port, nil))
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		util.DefaultContext.Fatal(err)
	}
	serverepoCmd.Flags().String("dir", path, "Packages folder (output from build)")
	serverepoCmd.Flags().String("port", "9090", "Listening port")
	serverepoCmd.Flags().String("address", "0.0.0.0", "Listening address")

	RootCmd.AddCommand(serverepoCmd)
}
