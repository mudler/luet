// Copyright © 2019 Ettore Di Giacinto <mudler@gentoo.org>
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
	"log"

	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "imports other package manager tree into luet",
	Long:  `Parses external PM and produces a luet parsable tree`,
	Run: func(cmd *cobra.Command, args []string) {
		//Output := viper.GetString("output")

		if len(args) == 0 {
			log.Fatalln("Insufficient arguments")
		}

		//input := args[0]

	},
}

func init() {
	RootCmd.AddCommand(importCmd)
}
