package linker

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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/xerrors"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// LinkGoModules produces the neccesary "replace"ments in all of the package's
// go.mod files, s.t. the packages link in the application/work with Go's tooling in-situ.
func LinkGoModules(application *gorpa.Application) error {
	mods, err := collectReplacements(application)
	if err != nil {
		return err
	}

	for _, p := range application.Packages {
		if p.Type != gorpa.GoPackage {
			continue
		}

		var apmods []goModule
		for _, dep := range p.GetTransitiveDependencies() {
			mod, ok := mods[dep.FullName()]
			if !ok {
				log.WithField("dep", dep.FullName()).Warn("did not find go.mod for this package - linking will probably be broken")
				continue
			}

			apmods = append(apmods, mod)
		}

		sort.Slice(apmods, func(i, j int) bool {
			return apmods[i].Name < apmods[j].Name
		})

		err = linkGoModule(p, apmods)
		if err != nil {
			return err
		}
	}

	return nil
}

func linkGoModule(dst *gorpa.Package, mods []goModule) error {
	var goModFn string
	for _, f := range dst.Sources {
		if strings.HasSuffix(f, "go.mod") {
			goModFn = f
			break
		}
	}
	if goModFn == "" {
		return xerrors.Errorf("%w: go.mod not found", os.ErrNotExist)
	}
	fc, err := ioutil.ReadFile(goModFn)
	if err != nil {
		return err
	}
	gomod, err := modfile.Parse(goModFn, fc, nil)
	if err != nil {
		return err
	}

	for _, mod := range mods {
		relpath, err := filepath.Rel(filepath.Dir(goModFn), mod.OriginPath)
		if err != nil {
			return err
		}

		err = addReplace(gomod, module.Version{Path: mod.Name}, module.Version{Path: relpath}, true, mod.OriginPackage)
		if err != nil {
			return err
		}
		log.WithField("dst", dst.FullName()).WithField("dep", mod.Name).Debug("linked Go modules")
	}
	for _, mod := range mods {
		for _, r := range mod.Replacements {
			err = addReplace(gomod, r.Old, r.New, false, mod.OriginPackage)
			if err != nil {
				return err
			}
		}
	}

	gomod.Cleanup()
	fc, err = gomod.Format()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(goModFn, fc, 0644)
	if err != nil {
		return err
	}

	return nil
}

func addReplace(gomod *modfile.File, old, new module.Version, direct bool, source string) error {
	for _, rep := range gomod.Replace {
		if rep.Old.Path != old.Path || rep.Old.Version != old.Version {
			continue
		}
		if ok, tpe := isGorpaReplace(rep); ok && tpe != gorpaReplaceIgnore {
			err := gomod.DropReplace(old.Path, old.Version)
			if err != nil {
				return err
			}

			continue
		}

		// replacement already exists - cannot replace
		return xerrors.Errorf("replacement for %s exists already, but was not added by Bhojpur GoRPA", old.String())
	}

	err := gomod.AddReplace(old.Path, old.Version, new.Path, new.Version)
	if err != nil {
		return err
	}

	comment := "// gorpa"
	if !direct {
		comment += " indirect from " + source
	}
	for _, rep := range gomod.Replace {
		if rep.Old.Path == old.Path && rep.Old.Version == old.Version {
			rep.Syntax.InBlock = true
			rep.Syntax.Comments.Suffix = []modfile.Comment{{Token: comment, Suffix: true}}
		}
	}
	return nil
}

type goModule struct {
	Name          string
	OriginPath    string
	OriginPackage string
	Replacements  []*modfile.Replace
}

func collectReplacements(application *gorpa.Application) (mods map[string]goModule, err error) {
	mods = make(map[string]goModule)
	for n, p := range application.Packages {
		if p.Type != gorpa.GoPackage {
			continue
		}

		var goModFn string
		for _, f := range p.Sources {
			if strings.HasSuffix(f, "go.mod") {
				goModFn = f
				break
			}
		}
		if goModFn == "" {
			continue
		}

		fc, err := ioutil.ReadFile(goModFn)
		if err != nil {
			return nil, err
		}

		gomod, err := modfile.Parse(goModFn, fc, nil)
		if err != nil {
			return nil, err
		}

		var replace []*modfile.Replace
		for _, rep := range gomod.Replace {
			skip, _ := isGorpaReplace(rep)
			if !skip {
				replace = append(replace, rep)
				log.WithField("rep", rep.Old.String()).WithField("pkg", n).Debug("collecting replace")
			} else {
				log.WithField("rep", rep.Old.String()).WithField("pkg", n).Debug("ignoring gorpa replace")
			}
		}

		mods[n] = goModule{
			Name:          gomod.Module.Mod.Path,
			OriginPath:    filepath.Dir(goModFn),
			OriginPackage: n,
			Replacements:  replace,
		}
	}
	return mods, nil
}

type gorpaReplaceType int

const (
	gorpaReplaceDirect gorpaReplaceType = iota
	gorpaReplaceIndirect
	gorpaReplaceIgnore
)

func isGorpaReplace(rep *modfile.Replace) (ok bool, tpe gorpaReplaceType) {
	for _, c := range rep.Syntax.Suffix {
		if strings.Contains(c.Token, "gorpa") {
			ok = true

			if strings.Contains(c.Token, " indirect ") {
				tpe = gorpaReplaceIndirect
			} else if strings.Contains(c.Token, " ignore ") {
				tpe = gorpaReplaceIgnore
			} else {
				tpe = gorpaReplaceDirect
			}

			return
		}
	}

	return false, gorpaReplaceDirect
}
