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
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// fmtCmd represents the version command
var fmtCmd = &cobra.Command{
	Use:   "fmt [files...]",
	Short: "Formats BUILD.yaml files",
	RunE: func(cmd *cobra.Command, args []string) error {
		fns := args
		if len(fns) == 0 {
			ba, err := getApplication()
			if err != nil {
				return err
			}
			for _, comp := range ba.Components {
				fns = append(fns, filepath.Join(comp.Origin, "BUILD.yaml"))
			}
		}

		var (
			inPlace, _ = cmd.Flags().GetBool("in-place")
			fix, _     = cmd.Flags().GetBool("fix")
		)
		for _, fn := range fns {
			err := formatBuildYaml(fn, inPlace, fix)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func formatBuildYaml(fn string, inPlace, fix bool) error {
	f, err := os.OpenFile(fn, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var out io.Writer = os.Stdout
	if inPlace {
		buf := bytes.NewBuffer(nil)
		out = buf
		//nolint:errcheck
		defer func() {
			f.Seek(0, 0)
			f.Truncate(0)

			io.Copy(f, buf)
		}()
	} else {
		fmt.Printf("---\n# %s\n", fn)
	}

	err = gorpa.FormatBUILDyaml(out, f, fix)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(fmtCmd)

	fmtCmd.Flags().BoolP("in-place", "i", false, "format file in place rather than printing it to stdout")
	fmtCmd.Flags().BoolP("fix", "f", false, "fix issues other than formatting (e.g. deprecated package types)")
}
