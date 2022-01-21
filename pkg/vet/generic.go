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

	log "github.com/sirupsen/logrus"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

func init() {
	register(PackageCheck("use-package", "attempts to find broken package paths in the commands", gorpa.GenericPackage, checkArgsReferingToPackage))
}

func checkArgsReferingToPackage(pkg *gorpa.Package) ([]Finding, error) {
	cfg, ok := pkg.Config.(gorpa.GenericPkgConfig)
	if !ok {
		// this is an error as compared to a finding because the issue most likely is with Bhojpur GoRPA,
		// and not a user config error.
		return nil, fmt.Errorf("Generic package does not have generic package config")
	}

	checkForFindings := func(fs []Finding, segmentIndex int, seg string) (findings []Finding) {
		findings = fs
		if !filesystemSafePathPattern.MatchString(seg) {
			return findings
		}

		pth := filesystemSafePathPattern.FindString(seg)
		log.WithField("pth", pth).WithField("pkg", pkg.FullName()).Debug("found potential package use")

		// we've found something that looks like a path - check if we have a dependency that could satisfy it
		var satisfied bool
		for _, dep := range pkg.GetDependencies() {
			if pkg.BuildLayoutLocation(dep) == pth {
				satisfied = true
				break
			}
		}
		if satisfied {
			return findings
		}

		findings = append(findings, Finding{
			Description: fmt.Sprintf("Command/Test %d refers to %s which looks like a package path, but no dependency satisfies it", segmentIndex, seg),
			Component:   pkg.C,
			Package:     pkg,
			Error:       false,
		})
		return findings
	}

	var findings []Finding
	for i, cmd := range cfg.Commands {
		for _, seg := range cmd {
			findings = checkForFindings(findings, i, seg)
		}
	}
	for i, cmd := range cfg.Test {
		for _, seg := range cmd {
			findings = checkForFindings(findings, i, seg)
		}
	}

	return findings, nil
}
