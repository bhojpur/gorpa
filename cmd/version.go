package cmd

import (
	"fmt"

	"github.com/bhojpur/gorpa/pkg/gorpa"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of this Bhojpur GoRPA software build",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(gorpa.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
