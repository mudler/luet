// Copyright © 2020-2021 Ettore Di Giacinto <mudler@gentoo.org>
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
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-units"

	config "github.com/mudler/luet/pkg/config"
	"github.com/mudler/luet/pkg/helpers/docker"
	. "github.com/mudler/luet/pkg/logger"

	"github.com/spf13/cobra"
)

func NewUnpackCommand() *cobra.Command {

	c := &cobra.Command{
		Use:   "unpack image path",
		Short: "Unpack a docker image natively",
		Long: `unpack doesn't need the docker daemon to run, and unpacks a docker image in the specified directory:
		
	luet util unpack golang:alpine /alpine
`,
		PreRun: func(cmd *cobra.Command, args []string) {

			if len(args) != 2 {
				Fatal("Expects an image and a path")
			}

		},
		Run: func(cmd *cobra.Command, args []string) {

			image := args[0]
			destination, err := filepath.Abs(args[1])
			if err != nil {
				Error("Invalid path %s", destination)
				os.Exit(1)
			}

			verify, _ := cmd.Flags().GetBool("verify")
			user, _ := cmd.Flags().GetString("auth-username")
			pass, _ := cmd.Flags().GetString("auth-password")
			authType, _ := cmd.Flags().GetString("auth-type")
			server, _ := cmd.Flags().GetString("auth-server-address")
			identity, _ := cmd.Flags().GetString("auth-identity-token")
			registryToken, _ := cmd.Flags().GetString("auth-registry-token")

			temp, err := config.LuetCfg.GetSystem().TempDir("contentstore")
			if err != nil {
				Fatal("Cannot create a tempdir", err.Error())
			}

			Info("Downloading", image, "to", destination)
			auth := &types.AuthConfig{
				Username:      user,
				Password:      pass,
				ServerAddress: server,
				Auth:          authType,
				IdentityToken: identity,
				RegistryToken: registryToken,
			}

			info, err := docker.DownloadAndExtractDockerImage(temp, image, destination, auth, verify)
			if err != nil {
				Error(err.Error())
				os.Exit(1)
			}
			Info(fmt.Sprintf("Pulled: %s %s", info.Target.Digest, info.Name))
			Info(fmt.Sprintf("Size: %s", units.BytesSize(float64(info.ContentSize))))
		},
	}

	c.Flags().String("auth-username", "", "Username to authenticate to registry/notary")
	c.Flags().String("auth-password", "", "Password to authenticate to registry")
	c.Flags().String("auth-type", "", "Auth type")
	c.Flags().String("auth-server-address", "", "Authentication server address")
	c.Flags().String("auth-identity-token", "", "Authentication identity token")
	c.Flags().String("auth-registry-token", "", "Authentication registry token")
	c.Flags().Bool("verify", false, "Verify signed images to notary before to pull")
	return c
}

var utilGroup = &cobra.Command{
	Use:   "util [command] [OPTIONS]",
	Short: "General luet internal utilities exposed",
}

func init() {
	RootCmd.AddCommand(utilGroup)

	utilGroup.AddCommand(
		NewUnpackCommand(),
	)
}
