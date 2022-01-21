package vet

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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

func init() {
	register(PackageCheck("deprecated-type", "checks if the package uses the deprecated typescript type", gorpa.YarnPackage, checkYarnDeprecatedType))
	register(&checkImplicitTransitiveDependencies{})
}

func checkYarnDeprecatedType(pkg *gorpa.Package) ([]Finding, error) {
	var rp struct {
		Type string `yaml:"type"`
	}
	err := yaml.Unmarshal(pkg.Definition, &rp)
	if err != nil {
		return nil, err
	}

	if rp.Type == string(gorpa.DeprecatedTypescriptPackage) {
		return []Finding{
			{
				Description: "package uses deprecated \"typescript\" type - use \"yarn\" instead (run `gorpa fmt -fi` to fix this)",
				Component:   pkg.C,
				Package:     pkg,
			},
		}, nil
	}

	return nil, nil
}

type pkgJSON struct {
	Name         string                 `json:"name"`
	Dependencies map[string]interface{} `json:"dependencies"`
}

type checkImplicitTransitiveDependencies struct {
	pkgs map[string][]string
}

func (c *checkImplicitTransitiveDependencies) Info() CheckInfo {
	tpe := gorpa.YarnPackage
	return CheckInfo{
		Name:          "yarn:implicit-transitive-dependency",
		Description:   "checks if the package's code uses another Yarn package in the application without declaring the dependency",
		AppliesToType: &tpe,
		PackageCheck:  true,
	}
}

func (c *checkImplicitTransitiveDependencies) Init(ba gorpa.Application) error {
	c.pkgs = make(map[string][]string)
	for pn, p := range ba.Packages {
		if p.Type != gorpa.YarnPackage {
			continue
		}

		pkgJSON, err := c.getPkgJSON(p)
		if err != nil {
			return err
		}

		if pkgJSON.Name == "" {
			continue
		}
		c.pkgs[pkgJSON.Name] = append(c.pkgs[pkgJSON.Name], pn)
	}
	return nil
}

func (c *checkImplicitTransitiveDependencies) getPkgJSON(pkg *gorpa.Package) (*pkgJSON, error) {
	var (
		found bool
		pkgFN = filepath.Join(pkg.C.Origin, "package.json")
	)
	for _, src := range pkg.Sources {
		if src == pkgFN {
			found = true
			break
		}
	}
	if !found {
		return nil, xerrors.Errorf("package %s has no package.json", pkg.FullName())
	}

	fc, err := ioutil.ReadFile(pkgFN)
	if err != nil {
		return nil, err
	}
	var res pkgJSON
	err = json.Unmarshal(fc, &res)
	if err != nil {
		return nil, err
	}

	if res.Name == "" {
		return nil, xerrors.Errorf("package %s has no Yarn package name", pkg.FullName())
	}

	return &res, nil
}

func (c *checkImplicitTransitiveDependencies) grepInFile(fn string, pat *regexp.Regexp) (contains bool, err error) {
	f, err := os.Open(fn)
	if err != nil {
		return
	}
	defer f.Close()

	r := bufio.NewReader(f)
	for {
		bt, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}

		if pat.Match(bt) {
			return true, nil
		}
	}

	return false, nil
}

func (c *checkImplicitTransitiveDependencies) RunCmp(pkg *gorpa.Component) ([]Finding, error) {
	return nil, fmt.Errorf("not a component check")
}

func (c *checkImplicitTransitiveDependencies) RunPkg(pkg *gorpa.Package) ([]Finding, error) {
	depsInCode := make(map[string]string)
	for _, src := range pkg.Sources {
		switch filepath.Ext(src) {
		case ".js":
		case ".ts":
		default:
			continue
		}

		for yarnpkg := range c.pkgs {
			r, _ := regexp.Compile(fmt.Sprintf("['\"]%s['\"/]", yarnpkg))
			ok, err := c.grepInFile(src, r)
			if err != nil {
				return nil, err
			}
			if ok {
				depsInCode[yarnpkg] = src
			}
		}
	}

	var findings []Finding
	for yarnDep, src := range depsInCode {
		var found bool
		for _, gorpaDep := range c.pkgs[yarnDep] {
			for _, dep := range pkg.GetDependencies() {
				if dep.FullName() == gorpaDep {
					found = true
					break
				}
			}
		}
		if found {
			continue
		}

		findings = append(findings, Finding{
			Description: fmt.Sprintf("%s depends on the application Yarn-package %s (provided by %s) but does not declare that dependency in its BUILD.yaml", src, yarnDep, strings.Join(c.pkgs[yarnDep], ", ")),
			Error:       true,
			Component:   pkg.C,
			Package:     pkg,
		})
	}

	pkgjson, err := c.getPkgJSON(pkg)
	if err != nil {
		return findings, err
	}
	for yarnDep, src := range depsInCode {
		_, found := pkgjson.Dependencies[yarnDep]
		if found {
			continue
		}

		log.WithField("pkg", pkg.FullName()).WithField("pkgJsonDeclaredDeps", pkgjson.Dependencies).WithField("yarnName", pkgjson.Name).Debug("found use of implicit transitive dependency")
		findings = append(findings, Finding{
			Description: fmt.Sprintf("%s depends on the application Yarn-package %s but does not declare that dependency in its package.json", src, yarnDep),
			Component:   pkg.C,
			Package:     pkg,
		})
	}

	return findings, nil
}
