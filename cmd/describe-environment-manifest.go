package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// describeEnvironmentManifestCmd represents the describeManifest command
var describeEnvironmentManifestCmd = &cobra.Command{
	Use:   "environment-manifest",
	Short: "Prints the environment manifest of an application",
	Run: func(cmd *cobra.Command, args []string) {
		ws, err := getApplication()
		if err != nil {
			log.WithError(err).Fatal("cannot load application")
		}

		err = ws.EnvironmentManifest.Write(os.Stdout)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	describeCmd.AddCommand(describeEnvironmentManifestCmd)
}
