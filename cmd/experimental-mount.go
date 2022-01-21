//go:build linux
// +build linux

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
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:   "mount <destination>",
	Short: "[experimental] Mounts a package or application variant",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ba, err := getApplication()
		if err != nil {
			return fmt.Errorf("cannot load application: %q", err)
		}

		dest := args[0]
		err = os.MkdirAll(dest, 0777)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("cannot create destination dir: %q", err)
		}

		wdbase, _ := cmd.Flags().GetString("workdir")
		if wdbase != "" {
			err = os.MkdirAll(wdbase, 0777)
		} else {
			wdbase, err = ioutil.TempDir(filepath.Dir(dest), "gorpa-workdir-*")
		}
		if err != nil && !os.IsExist(err) {
			return err
		}
		var (
			delup = filepath.Join(wdbase, "delup")
			delmp = filepath.Join(wdbase, "delmp")
			wd    = filepath.Join(wdbase, "work")
			upper = filepath.Join(wdbase, "upper")
		)
		for _, p := range []string{delup, delmp, wd, upper} {
			err = os.MkdirAll(p, 0777)
			if err != nil && !os.IsExist(err) {
				return err
			}
		}

		// prepare delup
		err = syscall.Mount("overlay", delmp, "overlay", 0, fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", ba.Origin, delup, wd))
		if err != nil {
			return fmt.Errorf("cannot mount delup overlay: %q", err)
		}
		strict, _ := cmd.Flags().GetBool("strict")
		err = gorpa.DeleteNonApplicationFiles(delmp, &ba, strict)
		if err != nil {
			return err
		}

		// actually mount overlay
		err = syscall.Mount("overlay", dest, "overlay", 0, fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", delmp, upper, wd))
		if err != nil {
			return fmt.Errorf("cannot mount overlay: %q", err)
		}

		return nil
	},
}

func init() {
	addExperimentalCommand(rootCmd, mountCmd)

	mountCmd.Flags().String("workdir", "", "overlayfs workdir location (must be on the same volume as the destination)")
	mountCmd.Flags().Bool("strict", false, "keep only package source files")
}
