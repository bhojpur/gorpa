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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
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
