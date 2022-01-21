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
	"encoding/json"
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

type checkFunc struct {
	info CheckInfo

	runPkg func(pkg *gorpa.Package) ([]Finding, error)
	runCmp func(pkg *gorpa.Component) ([]Finding, error)
}

func (cf *checkFunc) Info() CheckInfo {
	return cf.info
}

func (cf *checkFunc) Init(gorpa.Application) error {
	return nil
}

func (cf *checkFunc) RunPkg(pkg *gorpa.Package) ([]Finding, error) {
	if cf.runPkg == nil {
		return nil, xerrors.Errorf("not a package check")
	}
	return cf.runPkg(pkg)
}

func (cf *checkFunc) RunCmp(pkg *gorpa.Component) ([]Finding, error) {
	if cf.runCmp == nil {
		return nil, xerrors.Errorf("has no component check")
	}
	return cf.runCmp(pkg)
}

// PackageCheck produces a new check for a Bhojpur GoRPA package
func PackageCheck(name, desc string, tpe gorpa.PackageType, chk func(pkg *gorpa.Package) ([]Finding, error)) Check {
	return &checkFunc{
		info: CheckInfo{
			Name:          fmt.Sprintf("%s:%s", tpe, name),
			Description:   desc,
			AppliesToType: &tpe,
			PackageCheck:  true,
		},
		runPkg: chk,
	}
}

// ComponentCheck produces a new check for a Bhojpur GoRPA component
func ComponentCheck(name, desc string, chk func(pkg *gorpa.Component) ([]Finding, error)) Check {
	return &checkFunc{
		info: CheckInfo{
			Name:         fmt.Sprintf("component:%s", name),
			Description:  desc,
			PackageCheck: false,
		},
		runCmp: chk,
	}
}

// Check implements a vet check
type Check interface {
	Info() CheckInfo

	Init(ba gorpa.Application) error
	RunPkg(pkg *gorpa.Package) ([]Finding, error)
	RunCmp(pkg *gorpa.Component) ([]Finding, error)
}

// CheckInfo describes a check
type CheckInfo struct {
	Name          string
	Description   string
	PackageCheck  bool
	AppliesToType *gorpa.PackageType
}

// Finding describes a check finding. If the package is nil, the finding applies to the component
type Finding struct {
	Check       string
	Component   *gorpa.Component
	Package     *gorpa.Package
	Description string
	Error       bool
}

// MarshalJSON marshals a finding to JSON
func (f Finding) MarshalJSON() ([]byte, error) {
	var p struct {
		Check       string `json:"check"`
		Component   string `json:"component"`
		Package     string `json:"package,omitempty"`
		Description string `json:"description,omitempty"`
		Error       bool   `json:"error"`
	}
	p.Check = f.Check
	p.Component = f.Component.Name
	if f.Package != nil {
		p.Package = f.Package.FullName()
	}
	p.Description = f.Description
	p.Error = f.Error

	return json.Marshal(p)
}

var _checks = make(map[string]Check)

func register(c Check) {
	cn := c.Info().Name
	if _, exists := _checks[cn]; exists {
		panic(fmt.Sprintf("check %s is already registered", cn))
	}
	_checks[cn] = c
}

// Checks returns a list of all available checks
func Checks() []Check {
	l := make([]Check, 0, len(_checks))
	for _, c := range _checks {
		l = append(l, c)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Info().Name < l[j].Info().Name })
	return l
}

// RunOpt modifies the run behaviour
type RunOpt func(*runOptions)

type runOptions struct {
	Packages   StringSet
	Components StringSet
	Checks     []string
}

// StringSet identifies a string as part of a set
type StringSet map[string]struct{}

// OnPackages makes run check these packages only
func OnPackages(n StringSet) RunOpt {
	return func(r *runOptions) {
		r.Packages = n
	}
}

// OnComponents makes run check these components only
func OnComponents(n StringSet) RunOpt {
	return func(r *runOptions) {
		r.Components = n
	}
}

// WithChecks runs these checks only
func WithChecks(n []string) RunOpt {
	return func(r *runOptions) {
		r.Checks = n
	}
}

// Run runs all checks on all packages
func Run(application gorpa.Application, options ...RunOpt) ([]Finding, []error) {
	var opts runOptions
	for _, o := range options {
		o(&opts)
	}

	var checks []Check
	if len(opts.Checks) == 0 {
		checks = make([]Check, 0, len(_checks))
		for _, c := range _checks {
			checks = append(checks, c)
		}
	} else {
		log.WithField("checks", opts.Checks).Debug("running selected checks only")
		for _, cn := range opts.Checks {
			c, ok := _checks[cn]
			if !ok {
				return nil, []error{xerrors.Errorf("check %s not found", cn)}
			}
			checks = append(checks, c)
		}
	}
	for _, check := range checks {
		err := check.Init(application)
		if err != nil {
			return nil, []error{err}
		}
	}

	var (
		findings []Finding
		errs     []error

		runCompCheck = func(c Check, comp *gorpa.Component) {
			info := c.Info()
			if info.PackageCheck {
				return
			}

			log.WithField("check", info.Name).WithField("cmp", comp.Name).Debug("running component check")
			f, err := c.RunCmp(comp)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", comp.Name, err))
				return
			}
			for i := range f {
				f[i].Check = info.Name
			}
			findings = append(findings, f...)
		}
		runPkgCheck = func(c Check, pkg *gorpa.Package) {
			info := c.Info()
			if !info.PackageCheck {
				return
			}

			if info.AppliesToType != nil && *info.AppliesToType != pkg.Type {
				return
			}

			log.WithField("check", info.Name).WithField("pkg", pkg.FullName()).Debug("running package check")
			f, err := c.RunPkg(pkg)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", pkg.FullName(), err))
				return
			}
			for i := range f {
				f[i].Check = info.Name
			}
			findings = append(findings, f...)
		}
	)

	if len(opts.Components) > 0 {
		for n, comp := range application.Components {
			if _, ok := opts.Components[n]; !ok {
				continue
			}

			for _, check := range checks {
				runCompCheck(check, comp)
			}
		}
	} else if len(opts.Packages) > 0 {
		for n, pkg := range application.Packages {
			if _, ok := opts.Packages[n]; !ok {
				continue
			}

			for _, check := range checks {
				runPkgCheck(check, pkg)
			}
		}
	} else {
		for _, check := range checks {
			for _, comp := range application.Components {
				runCompCheck(check, comp)
			}

			for _, pkg := range application.Packages {
				runPkgCheck(check, pkg)
			}
		}
	}

	return findings, errs
}
