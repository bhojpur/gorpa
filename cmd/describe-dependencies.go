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
	"os/exec"
	"strings"
	"time"

	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
	"github.com/bhojpur/gorpa/pkg/graphview"
)

// describeDependenciesCmd represents the describeDot command
var describeDependenciesCmd = &cobra.Command{
	Use:   "dependencies",
	Short: "Describes the depenencies package on the console, in Graphviz's dot format or as interactive website",
	RunE: func(cmd *cobra.Command, args []string) error {
		var pkgs []*gorpa.Package
		if len(args) > 0 {
			_, pkg, _, _ := getTarget(args, false)
			if pkg == nil {
				log.Fatal("graphview needs a package")
			}
			pkgs = []*gorpa.Package{pkg}
		} else {
			ba, err := getApplication()
			if err != nil {
				log.Fatal(err)
			}

			allpkgs := ba.Packages
			for _, p := range allpkgs {
				for _, d := range p.GetDependencies() {
					delete(allpkgs, d.FullName())
				}
			}
			for _, p := range allpkgs {
				pkgs = append(pkgs, p)
			}
		}

		if dot, _ := cmd.Flags().GetBool("dot"); dot {
			return printDepGraphAsDot(pkgs)
		} else if serve, _ := cmd.Flags().GetString("serve"); serve != "" {
			serveDepGraph(serve, pkgs)
		} else {
			for _, pkg := range pkgs {
				printDepTree(pkg, 0)
			}
		}

		return nil
	},
}

func printDepTree(pkg *gorpa.Package, indent int) {
	var tpe string
	switch pkg.Type {
	case gorpa.DockerPackage:
		tpe = "docker"
	case gorpa.GenericPackage:
		tpe = "generic"
	case gorpa.GoPackage:
		tpe = "go"
	case gorpa.YarnPackage:
		tpe = "yarn"
	}

	fmt.Printf("%*s%s %s\n", indent, "", color.Gray.Sprintf("[%7s]", tpe), pkg.FullName())
	for _, p := range pkg.GetDependencies() {
		printDepTree(p, indent+4)
	}
}

func printDepGraphAsDot(pkgs []*gorpa.Package) error {
	var (
		nodes = make(map[string]string)
		edges []string
	)

	for _, pkg := range pkgs {
		allpkg := append(pkg.GetTransitiveDependencies(), pkg)
		for _, p := range allpkg {
			ver, err := p.Version()
			if err != nil {
				return err
			}
			if _, exists := nodes[ver]; exists {
				continue
			}
			nodes[ver] = fmt.Sprintf("p%s [label=\"%s\"];", ver, p.FullName())
		}
		for _, p := range allpkg {
			ver, err := p.Version()
			if err != nil {
				return err
			}

			for _, dep := range p.GetDependencies() {
				depver, err := dep.Version()
				if err != nil {
					return err
				}
				edges = append(edges, fmt.Sprintf("p%s -> p%s;", ver, depver))
			}
		}
	}

	fmt.Println("digraph G {")
	for _, n := range nodes {
		fmt.Printf("  %s\n", n)
	}
	for _, e := range edges {
		fmt.Printf("  %s\n", e)
	}
	fmt.Println("}")
	return nil
}

func serveDepGraph(addr string, pkgs []*gorpa.Package) {
	go func() {
		browser := os.Getenv("BROWSER")
		if browser == "" {
			return
		}

		time.Sleep(2 * time.Second)
		taddr := addr
		if strings.HasPrefix(taddr, ":") {
			taddr = fmt.Sprintf("localhost%s", addr)
		}
		taddr = fmt.Sprintf("http://%s", taddr)
		//nolint:errcheck
		exec.Command(browser, taddr).Start()
	}()

	log.Infof("serving dependency graph on %s", addr)
	log.Fatal(graphview.Serve(addr, pkgs...))
}

func init() {
	describeCmd.AddCommand(describeDependenciesCmd)

	describeDependenciesCmd.Flags().Bool("dot", false, "produce Graphviz dot output")
	describeDependenciesCmd.Flags().String("serve", "", "serve the interactive dependency graph on this address")
}
