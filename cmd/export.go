package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bhojpur/gorpa/pkg/gorpa"
)

// exportCmd represents the version command
var exportCmd = &cobra.Command{
	Use:   "export <destination>",
	Short: "Copies an Application to the destination",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(args[0]); err == nil {
			return fmt.Errorf("destination %s exists already", args[0])
		}

		application, err := getApplication()
		if err != nil {
			return err
		}

		strict, _ := cmd.Flags().GetBool("strict")
		return gorpa.CopyApplication(args[0], &application, strict)
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().Bool("strict", false, "keep only package source files")
}
