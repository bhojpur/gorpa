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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// LinkYarnPackagesWithYarn2 uses `yarn link` to link all TS packages in-situ.
func LinkYarnPackagesWithYarn2(application *gorpa.Application) error {
	var (
		pkgIdx     = make(map[string]string)
		pkgJSONIdx = make(map[string]string)
	)
	for n, p := range application.Packages {
		if p.Type != gorpa.YarnPackage {
			continue
		}

		var pkgjsonFn string
		for _, src := range p.Sources {
			if strings.HasSuffix(src, "/package.json") {
				pkgjsonFn = src
				break
			}
		}
		if pkgjsonFn == "" {
			log.WithField("pkg", n).Warn("no package.json found - skipping")
			continue
		}
		pkgJSONIdx[n] = pkgjsonFn

		fc, err := ioutil.ReadFile(pkgjsonFn)
		if err != nil {
			return err
		}
		var pkgjson struct {
			Name string `json:"name"`
		}
		err = json.Unmarshal(fc, &pkgjson)
		if err != nil {
			return err
		}
		pkgIdx[n] = pkgjson.Name
	}

	for n, p := range application.Packages {
		if p.Type != gorpa.YarnPackage {
			continue
		}
		pkgjsonFn := pkgJSONIdx[n]

		fc, err := ioutil.ReadFile(pkgjsonFn)
		if err != nil {
			return err
		}
		var pkgjson map[string]interface{}
		err = json.Unmarshal(fc, &pkgjson)
		if err != nil {
			return err
		}

		var resolutions map[string]interface{}
		if res, ok := pkgjson["resolutions"]; ok {
			resolutions, ok = res.(map[string]interface{})
			if !ok {
				return xerrors.Errorf("%s: found resolutions but they're not a map", n)
			}
		} else {
			resolutions = make(map[string]interface{})
		}
		for _, dep := range p.GetTransitiveDependencies() {
			if dep.Type != gorpa.YarnPackage {
				continue
			}

			yarnPkg, ok := pkgIdx[dep.FullName()]
			if !ok {
				log.WithField("dep", dep.FullName()).WithField("pkg", n).Warn("did not find yarn package name - linking might be broken")
				continue
			}
			resolutions[yarnPkg] = fmt.Sprintf("portal://%s", dep.C.Origin)
		}
		if len(resolutions) > 0 {
			pkgjson["resolutions"] = resolutions
		}

		fd, err := os.OpenFile(pkgjsonFn, os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(fd)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		err = enc.Encode(pkgjson)
		fd.Close()
		if err != nil {
			return err
		}

		log.WithField("pkg", n).WithField("resolutions", resolutions).Debug("linked package")
	}

	var lerr error
	for n, p := range application.Packages {
		if p.Type != gorpa.YarnPackage {
			continue
		}

		cmd := exec.Command("yarn")
		log.WithField("pkg", n).WithField("cwd", p.C.Origin).WithField("cmd", "yarn").Debug("running yarn")
		cmd.Dir = p.C.Origin
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.WithError(err).Error("error while running yarn")
			lerr = xerrors.Errorf("yarn failed for %s: %w", n, err)
		}
	}

	return lerr
}
