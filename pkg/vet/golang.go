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
	"fmt"
	"strings"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

func init() {
	register(PackageCheck("has-gomod", "ensures all Go packages have a go.mod file in their source list", gorpa.GoPackage, checkGolangHasGomod))
	register(PackageCheck("has-buildflags", "checks for use of deprecated buildFlags config", gorpa.GoPackage, checkGolangHasBuildFlags))
}

func checkGolangHasGomod(pkg *gorpa.Package) ([]Finding, error) {
	var (
		foundGoMod bool
		foundGoSum bool
	)
	for _, src := range pkg.Sources {
		if strings.HasSuffix(src, "/go.mod") {
			foundGoMod = true
		}
		if strings.HasSuffix(src, "/go.sum") {
			foundGoSum = true
		}
		if foundGoSum && foundGoMod {
			return nil, nil
		}
	}

	var f []Finding
	if !foundGoMod {
		f = append(f, Finding{
			Component:   pkg.C,
			Description: "package sources contain no go.mod file",
			Error:       true,
			Package:     pkg,
		})
	}
	if !foundGoSum {
		f = append(f, Finding{
			Component:   pkg.C,
			Description: "package sources contain no go.sum file",
			Error:       true,
			Package:     pkg,
		})
	}
	return f, nil
}

func checkGolangHasBuildFlags(pkg *gorpa.Package) ([]Finding, error) {
	goCfg, ok := pkg.Config.(gorpa.GoPkgConfig)
	if !ok {
		return nil, fmt.Errorf("Go package does not have Go package config")
	}

	if len(goCfg.BuildFlags) > 0 {
		return []Finding{{
			Component:   pkg.C,
			Description: "buildFlags are deprecated, use buildCommand instead",
			Error:       false,
			Package:     pkg,
		}}, nil
	}

	return nil, nil
}
