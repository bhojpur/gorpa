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

	"github.com/disiqueira/gotree"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// describeTreeCmd represents the describeTree command
var describeTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Prints the depepency tree of a package",
	Run: func(cmd *cobra.Command, args []string) {
		_, pkg, _, _ := getTarget(args, false)
		if pkg == nil {
			log.Fatal("tree needs a package")
		}

		var print func(parent gotree.Tree, pkg *gorpa.Package)
		print = func(parent gotree.Tree, pkg *gorpa.Package) {
			n := parent.Add(pkg.FullName())
			for _, dep := range pkg.GetDependencies() {
				print(n, dep)
			}
		}

		tree := gotree.New("APPLICATION")
		print(tree, pkg)
		_, err := fmt.Println(tree.Print())
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	describeCmd.AddCommand(describeTreeCmd)
}
