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
	"fmt"
	"os"

	b64 "encoding/base64"

	"github.com/mudler/luet/cmd/util"
	"github.com/mudler/luet/pkg/box"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec --rootfs /path [command]",
	Short: "Execute a command in the rootfs context",
	Long:  `Uses unshare technique and pivot root to execute a command inside a folder containing a valid rootfs`,
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	// If you change this, look at pkg/box/exec that runs this command and adapt
	Run: func(cmd *cobra.Command, args []string) {

		stdin, _ := cmd.Flags().GetBool("stdin")
		stdout, _ := cmd.Flags().GetBool("stdout")
		stderr, _ := cmd.Flags().GetBool("stderr")
		rootfs, _ := cmd.Flags().GetString("rootfs")
		base, _ := cmd.Flags().GetBool("decode")

		entrypoint, _ := cmd.Flags().GetString("entrypoint")
		envs, _ := cmd.Flags().GetStringArray("env")
		mounts, _ := cmd.Flags().GetStringArray("mount")

		if base {
			var ss []string
			for _, a := range args {
				sDec, _ := b64.StdEncoding.DecodeString(a)
				ss = append(ss, string(sDec))
			}
			//If the command to run is complex,using base64 to avoid bad input

			args = ss
		}
		util.DefaultContext.Info("Executing", args, "in", rootfs)

		b := box.NewBox(entrypoint, args, mounts, envs, rootfs, stdin, stdout, stderr)
		err := b.Exec()
		if err != nil {
			util.DefaultContext.Fatal(errors.Wrap(err, fmt.Sprintf("entrypoint: %s rootfs: %s", entrypoint, rootfs)))
		}
	},
}

func init() {
	path, err := os.Getwd()
	if err != nil {
		util.DefaultContext.Fatal(err)
	}
	execCmd.Hidden = true
	execCmd.Flags().String("rootfs", path, "Rootfs path")
	execCmd.Flags().Bool("stdin", false, "Attach to stdin")
	execCmd.Flags().Bool("stdout", false, "Attach to stdout")
	execCmd.Flags().Bool("stderr", false, "Attach to stderr")
	execCmd.Flags().Bool("decode", false, "Base64 decode")

	execCmd.Flags().StringArrayP("env", "e", []string{}, "Environment settings")
	execCmd.Flags().StringArrayP("mount", "m", []string{}, "List of paths to bind-mount from the host")

	execCmd.Flags().String("entrypoint", "/bin/sh", "Entrypoint command (/bin/sh)")

	RootCmd.AddCommand(execCmd)
}
