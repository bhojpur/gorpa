package gokart

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
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/praetorian-inc/gokart/util"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const AnalyzerFilename = "_gokart-analyzer.yaml"

func BuildAnalyzerConfig(wd string, apiDepPattern *regexp.Regexp) error {
	var cfg util.ConfigFile
	err := yaml.Unmarshal(util.DefaultAnalyzersContent, &cfg)
	if err != nil {
		return err
	}

	out, err := exec.Command("go", "mod", "graph").CombinedOutput()
	if err != nil {
		return err
	}
	deps := make(map[string]struct{})
	for _, l := range strings.Split(string(out), "\n") {
		segs := strings.Split(l, " ")
		segs = strings.Split(segs[0], "@")
		nme := segs[0]

		if !apiDepPattern.MatchString(nme) {
			continue
		}

		deps[nme] = struct{}{}
	}

	depsSorted := make([]string, 0, len(deps))
	for dep := range deps {
		depsSorted = append(depsSorted, dep)
		logrus.WithField("dep", dep).Debug("found a GoKart relevant API dependency")
	}
	sort.Strings(depsSorted)

	for _, dep := range depsSorted {
		var nfo struct {
			Dir     string
			GoFiles []string
		}
		out, err = exec.Command("go", "list", "-json", dep).CombinedOutput()
		if err != nil {
			logrus.WithError(err).Warnf("cannot list depenency %s", dep)
			continue
		}
		err = json.Unmarshal(out, &nfo)
		if err != nil {
			logrus.WithError(err).Warnf("cannot list depenency %s", dep)
			continue
		}

		fset := token.NewFileSet()
		for _, fn := range nfo.GoFiles {
			fn := filepath.Join(nfo.Dir, fn)
			fc, err := ioutil.ReadFile(fn)
			if err != nil {
				logrus.WithError(err).Warnf("cannot read %s", fn)
				continue
			}
			f, err := parser.ParseFile(fset, fn, fc, parser.SkipObjectResolution)
			if err != nil {
				logrus.WithError(err).Warnf("cannot parse %s", fn)
				continue
			}
			for _, dec := range f.Decls {
				decl, ok := dec.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, spec := range decl.Specs {
					t, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					cfg.Sources.Types[dep] = append(cfg.Sources.Types[dep], t.Name.String())
				}
			}
		}
	}

	cfgFC, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(wd, AnalyzerFilename), cfgFC, 0644)
}
