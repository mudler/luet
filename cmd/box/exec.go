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

package cmd_box

import (
	"os"

	b64 "encoding/base64"

	"github.com/mudler/luet/pkg/box"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/spf13/cobra"
)

func NewBoxExecCommand() *cobra.Command {
	var ans = &cobra.Command{
		Use:   "exec [OPTIONS]",
		Short: "Execute a binary in a box",
		Args:  cobra.OnlyValidArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
		},
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
			Info("Executing", args, "in", rootfs)

			b := box.NewBox(entrypoint, args, mounts, envs, rootfs, stdin, stdout, stderr)
			err := b.Run()
			if err != nil {
				Fatal(err)
			}
		},
	}
	path, err := os.Getwd()
	if err != nil {
		Fatal(err)
	}
	ans.Flags().String("rootfs", path, "Rootfs path")
	ans.Flags().Bool("stdin", false, "Attach to stdin")
	ans.Flags().Bool("stdout", true, "Attach to stdout")
	ans.Flags().Bool("stderr", true, "Attach to stderr")
	ans.Flags().Bool("decode", false, "Base64 decode")
	ans.Flags().StringArrayP("env", "e", []string{}, "Environment settings")
	ans.Flags().StringArrayP("mount", "m", []string{}, "List of paths to bind-mount from the host")

	ans.Flags().String("entrypoint", "/bin/sh", "Entrypoint command (/bin/sh)")

	return ans
}
