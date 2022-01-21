package cmd

// Copyright (c) 2018 Bhojpur Consulting Private Limited, India. All rights reserved.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bhojpur/gorpa/pkg/linker"
)

// linkCmd represents the version command
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Links all packages in-situ",
	RunE: func(cmd *cobra.Command, args []string) error {
		ba, err := getApplication()
		if err != nil {
			return err
		}

		if ok, _ := cmd.Flags().GetBool("go-link"); ok {
			err = linker.LinkGoModules(&ba)
			if err != nil {
				return err
			}
		} else {
			log.Info("go module linking disabled")
		}

		if ok, _ := cmd.Flags().GetBool("yarn2-link"); ok {
			err = linker.LinkYarnPackagesWithYarn2(&ba)
			if err != nil {
				return err
			}
		} else {
			log.Info("yarn2 package linking disabled")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)

	linkCmd.Flags().Bool("yarn2-link", false, "link yarn packages using yarn2 resolutions")
	linkCmd.Flags().Bool("go-link", true, "link Go modules")
}
