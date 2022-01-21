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
	"io"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

var (
	dockerfileCandidates      = []string{"Dockerfile", "gorpa.Dockerfile"}
	packageTypeDetectionFiles = map[gorpa.PackageType][]string{
		gorpa.DockerPackage: dockerfileCandidates,
		gorpa.GoPackage:     {"go.mod", "go.sum"},
		gorpa.YarnPackage:   {"package.json", "yarn.lock"},
	}
	initPackageGenerator = map[gorpa.PackageType]func(name string) ([]byte, error){
		gorpa.DockerPackage:  initDockerPackage,
		gorpa.GoPackage:      initGoPackage,
		gorpa.YarnPackage:    initYarnPackage,
		gorpa.GenericPackage: initGenericPackage,
	}
)

// initCmd represents the version command
var initCmd = &cobra.Command{
	Use:       "init <name>",
	Short:     "Initializes a new Bhojpur GoRPA package (and component if need be) in the current directory",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"go", "yarn", "docker", "generic"},
	RunE: func(cmd *cobra.Command, args []string) error {
		var tpe gorpa.PackageType
		if tper, _ := cmd.Flags().GetString("type"); tper != "" {
			tpe = gorpa.PackageType(tper)
		} else {
			tpe = detectPossiblePackageType()
		}

		generator, ok := initPackageGenerator[tpe]
		if !ok {
			return fmt.Errorf("unknown package type: %q", tpe)
		}

		tpl, err := generator(args[0])
		if err != nil {
			return err
		}
		var pkg yaml.Node
		err = yaml.Unmarshal(tpl, &pkg)
		if err != nil {
			log.WithField("template", string(tpl)).Warn("broken package template")
			return fmt.Errorf("This is a Bhojpur GoRPA bug. Cannot parse package template: %w", err)
		}

		f, err := os.OpenFile("BUILD.yaml", os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		var cmp yaml.Node
		err = yaml.NewDecoder(f).Decode(&cmp)
		if err == io.EOF {
			err = yaml.Unmarshal([]byte(`packages: []`), &cmp)
		}
		if err != nil {
			return err
		}

		cmps := cmp.Content[0].Content
		for i, nde := range cmps {
			if !(nde.Value == "packages" && i < len(cmps)-1 && cmps[i+1].Kind == yaml.SequenceNode) {
				continue
			}

			pkgs := cmps[i+1]
			pkgs.Style = yaml.FoldedStyle
			pkgs.Content = append(pkgs.Content, pkg.Content[0])
			cmps[i+1] = pkgs
		}
		cmp.Content[0].Content = cmps

		_, err = f.Seek(0, 0)
		if err != nil {
			return err
		}
		err = yaml.NewEncoder(f).Encode(&cmp)
		if err != nil {
			return err
		}

		return nil
	},
}

func detectPossiblePackageType() gorpa.PackageType {
	for tpe, fns := range packageTypeDetectionFiles {
		for _, fn := range fns {
			_, err := os.Stat(fn)
			if err != nil {
				continue
			}

			return tpe
		}
	}

	return gorpa.GenericPackage
}

func initGoPackage(name string) ([]byte, error) {
	return []byte(fmt.Sprintf(`name: %s
type: go
srcs:
  - go.mod
  - go.sum
  - "**/*.go"
env:
  - CGO_ENABLED=0
config:
  packaging: app
`, name)), nil
}

func initDockerPackage(name string) ([]byte, error) {
	var dockerfile string
	for _, f := range dockerfileCandidates {
		if _, err := os.Stat(f); err == nil {
			dockerfile = f
			break
		}
	}
	if dockerfile == "" {
		return nil, fmt.Errorf("no Dockerfile found")
	}

	log.Warnf("Please update your BUILD.yaml and change the image reference of the new \"%s\" package", name)
	return []byte(fmt.Sprintf(`name: %s
type: docker
config:
  dockerfile: %s
  image: some/image/in/some:repo`, name, dockerfile)), nil
}

func initYarnPackage(name string) ([]byte, error) {
	return []byte(fmt.Sprintf(`name: %s
type: yarn
srcs:
  - package.json
  - "src/**"
config:
  yarnLock: yarn.lock
  tsconfig: tsconfig.json
`, name)), nil
}

func initGenericPackage(name string) ([]byte, error) {
	fs, err := ioutil.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var srcs []string
	for _, f := range fs {
		if f.Name() == "BUILD.yaml" {
			continue
		}
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}

		var da string
		if f.IsDir() {
			da = "/**"
		}
		srcs = append(srcs, fmt.Sprintf("  - \"%s%s\"", f.Name(), da))
	}

	log.Warnf("Please update your BUILD.yaml and change the commands of the new \"%s\" package", name)
	return []byte(fmt.Sprintf(`name: %s
type: generic
srcs:
%s
config:
  comamnds:
  - ["echo", "commands", "go", "here"]
`, name, strings.Join(srcs, "\n"))), nil
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringP("type", "t", "", "type of the new package")
}
