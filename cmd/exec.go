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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gookit/color"
	"github.com/segmentio/textio"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// execCmd represents the version command
var execCmd = &cobra.Command{
	Use:   "exec <cmd>",
	Short: "Executes a command in the application directories, sorted by package dependencies",
	Long: `Executes a command in the application directories, sorted by package dependencies.
This command can use a single package as starting point, and can traverse and filter its dependency tree.
For each matching package Bhojpur GoRPA will execute the specified command in the package component's origin.
To avoid executing the command in the same directory multiple times (e.g. when a component has multiple
matching packages), use --components which selects the components isntead of the packages.
Example use:
  # list all component directories of all yarn packages:
  gorpa exec --filter-type yarn -- pwd
  # run go get in all Go packages
  gorpa exec --filter-type go -- go get -v ./...
  # execute go build in all direct Go dependencies when any of the relevant source files changes:
  gorpa exec --package some/other:package --dependencies --filter-type go --parallel --watch -- go build
  # run tsc watch for all dependent yarn packages (once per component origin):
  gorpa exec --package some/other:package --transitive-dependencies --filter-type yarn --parallel -- tsc -a --preserveWatchOutput
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var (
			packages, _         = cmd.Flags().GetStringArray("package")
			includeDeps, _      = cmd.Flags().GetBool("dependencies")
			includeTransDeps, _ = cmd.Flags().GetBool("transitive-dependencies")
			components, _       = cmd.Flags().GetBool("components")
			filterType, _       = cmd.Flags().GetStringArray("filter-type")
			watch, _            = cmd.Flags().GetBool("watch")
			parallel, _         = cmd.Flags().GetBool("parallel")
		)

		ba, err := getApplication()
		if err != nil {
			log.WithError(err).Fatal("cannot load application")
		}

		var pkgs map[*gorpa.Package]struct{}
		if len(packages) == 0 {
			pkgs = make(map[*gorpa.Package]struct{}, len(ba.Packages))
			for _, p := range ba.Packages {
				pkgs[p] = struct{}{}
			}
		} else {
			pkgs = make(map[*gorpa.Package]struct{}, len(packages))
			for _, pn := range packages {
				pn := absPackageName(ba, pn)
				p, ok := ba.Packages[pn]
				if !ok {
					log.WithField("package", pn).Fatal("package not found")
				}
				pkgs[p] = struct{}{}
			}
		}

		if includeTransDeps {
			for p := range pkgs {
				for _, dep := range p.GetTransitiveDependencies() {
					pkgs[dep] = struct{}{}
				}
			}
		} else if includeDeps {
			for p := range pkgs {
				for _, dep := range p.GetDependencies() {
					pkgs[dep] = struct{}{}
				}
			}
		}

		for i, ft := range filterType {
			if ft == string(gorpa.DeprecatedTypescriptPackage) {
				filterType[i] = string(gorpa.YarnPackage)
			}
		}
		if len(filterType) > 0 {
			for pkg := range pkgs {
				var found bool
				for _, t := range filterType {
					if string(pkg.Type) == t {
						found = true
						break
					}
				}
				if found {
					continue
				}

				delete(pkgs, pkg)
			}
		}

		spkgs := make([]*gorpa.Package, 0, len(pkgs))
		for p := range pkgs {
			spkgs = append(spkgs, p)
		}
		gorpa.TopologicalSort(spkgs)

		locs := make([]commandExecLocation, 0, len(spkgs))
		if components {
			idx := make(map[string]struct{})
			for _, p := range spkgs {
				fn := p.C.Origin
				if _, ok := idx[fn]; ok {
					continue
				}
				idx[fn] = struct{}{}
				locs = append(locs, commandExecLocation{
					Component: p.C,
					Dir:       fn,
					Name:      p.C.Name,
				})
			}
		} else {
			for _, p := range spkgs {
				locs = append(locs, commandExecLocation{
					Component: p.C,
					Dir:       p.C.Origin,
					Package:   p,
					Name:      p.FullName(),
				})
			}
		}

		if watch {
			err := executeCommandInLocations(args, locs, parallel)
			if err != nil {
				log.Error(err)
			}

			evt, errs := gorpa.WatchSources(context.Background(), spkgs)
			for {
				select {
				case <-evt:
					err := executeCommandInLocations(args, locs, parallel)
					if err != nil {
						log.Error(err)
					}
				case err = <-errs:
					log.Fatal(err)
				}
			}
		}
		err = executeCommandInLocations(args, locs, parallel)
		if err != nil {
			log.WithError(err).Fatal("cannot execute command")
		}
	},
}

type commandExecLocation struct {
	Component *gorpa.Component
	Package   *gorpa.Package
	Dir       string
	Name      string
}

func executeCommandInLocations(execCmd []string, locs []commandExecLocation, parallel bool) error {
	var wg sync.WaitGroup
	for _, loc := range locs {
		if loc.Package != nil {
			log.WithField("dir", loc.Dir).WithField("pkg", loc.Package.FullName()).Debugf("running %q", execCmd)
		} else {
			log.WithField("dir", loc.Dir).Debugf("running %q", execCmd)
		}
		prefix := color.Gray.Render(fmt.Sprintf("[%s] ", loc.Name))

		cmd := exec.Command(execCmd[0], execCmd[1:]...)
		cmd.Dir = loc.Dir
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return fmt.Errorf("execution failed in %s (%s): %w", loc.Name, loc.Dir, err)
		}
		_ = pty.InheritSize(ptmx, os.Stdin)
		defer ptmx.Close()

		//nolint:errcheck
		go io.Copy(textio.NewPrefixWriter(os.Stdout, prefix), ptmx)
		//nolint:errcheck
		go io.Copy(ptmx, os.Stdin)
		if parallel {
			wg.Add(1)
			go func() {
				defer wg.Done()

				err = cmd.Wait()
				if err != nil {
					log.Errorf("execution failed in %s (%s): %v", loc.Name, loc.Dir, err)
				}
			}()
		} else {
			err = cmd.Wait()
			if err != nil {
				return fmt.Errorf("execution failed in %s (%s): %v", loc.Name, loc.Dir, err)
			}
		}
	}
	if parallel {
		wg.Wait()
	}

	return nil
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().StringArray("package", nil, "select a package by name")
	execCmd.Flags().Bool("dependencies", false, "select package dependencies")
	execCmd.Flags().Bool("transitive-dependencies", false, "select transitive package dependencies")
	execCmd.Flags().Bool("components", false, "select the package's components (e.g. instead of selecting three packages from the same component, execute just once in the component origin)")
	execCmd.Flags().StringArray("filter-type", nil, "only select packages of this type")
	execCmd.Flags().Bool("watch", false, "Watch source files and re-execute on change")
	execCmd.Flags().Bool("parallel", false, "Start all executions in parallel independent of their order")
	execCmd.Flags().SetInterspersed(true)
}
