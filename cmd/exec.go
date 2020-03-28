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
	"os"

	b64 "encoding/base64"

	"github.com/mudler/luet/pkg/box"
	. "github.com/mudler/luet/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var execCmd = &cobra.Command{
	Use:   "exec --rootfs /path [command]",
	Short: "Execute a command in the rootfs context",
	Long:  `Uses unshare technique and pivot root to execute a command inside a folder containing a valid rootfs`,
	PreRun: func(cmd *cobra.Command, args []string) {
		viper.BindPFlag("stdin", cmd.Flags().Lookup("stdin"))
		viper.BindPFlag("stdout", cmd.Flags().Lookup("stdout"))
		viper.BindPFlag("stderr", cmd.Flags().Lookup("stderr"))
		viper.BindPFlag("rootfs", cmd.Flags().Lookup("rootfs"))
		viper.BindPFlag("decode", cmd.Flags().Lookup("decode"))
		viper.BindPFlag("entrypoint", cmd.Flags().Lookup("entrypoint"))

	},
	// If you change this, look at pkg/box/exec that runs this command and adapt
	Run: func(cmd *cobra.Command, args []string) {

		stdin := viper.GetBool("stdin")
		stdout := viper.GetBool("stdout")
		stderr := viper.GetBool("stderr")
		rootfs := viper.GetString("rootfs")
		base := viper.GetBool("decode")

		entrypoint := viper.GetString("entrypoint")
		if base {
			var ss []string
			for _, a := range args {
				sDec, _ := b64.StdEncoding.DecodeString(a)
				ss = append(ss, string(sDec))
			}
			//If the command to run is complex,using base64 to avoid bad input

			args = ss
		}
		Info("Executing", args, "in", rootfs)

		b := box.NewBox(entrypoint, args, rootfs, stdin, stdout, stderr)
		err := b.Exec()
		if err != nil {
			Fatal(err)
		}
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	execCmd.Hidden = true
	execCmd.Flags().String("rootfs", path, "Rootfs path")
	execCmd.Flags().Bool("stdin", false, "Attach to stdin")
	execCmd.Flags().Bool("stdout", false, "Attach to stdout")
	execCmd.Flags().Bool("stderr", false, "Attach to stderr")
	execCmd.Flags().Bool("decode", false, "Base64 decode")

	execCmd.Flags().String("entrypoint", "/bin/sh", "Entrypoint command (/bin/sh)")

	RootCmd.AddCommand(execCmd)
}
