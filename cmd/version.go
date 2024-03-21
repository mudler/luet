package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of luet",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
