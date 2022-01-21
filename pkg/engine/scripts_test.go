package engine_test

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
	// "path/filepath"
	"bytes"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/bhojpur/gorpa/cmd"
)

var dut = flag.Bool("dut", false, "run command/device under test")

func runDUT() {
	if *dut {
		cmd.Execute()
		os.Exit(0)
	}
}

func TestScriptArgs(t *testing.T) {
	runDUT()

	tests := []*CommandFixtureTest{
		{
			Name:                "unresolved arg",
			T:                   t,
			Args:                []string{"run", "fixtures/scripts:echo"},
			NoNestedApplication: true,
			ExitCode:            1,
		},
		{
			Name:                "resovled args",
			T:                   t,
			Args:                []string{"run", "fixtures/scripts:echo", "-Dmsg=foobar"},
			NoNestedApplication: true,
			ExitCode:            0,
		},
	}

	for _, test := range tests {
		test.Run()
	}
}

func TestWorkingDirLayout(t *testing.T) {
	runDUT()

	tests := []*CommandFixtureTest{
		{
			Name:                "origin",
			T:                   t,
			Args:                []string{"run", "fixtures/scripts:pwd-origin"},
			ExitCode:            0,
			NoNestedApplication: true,
			StdoutSub: `.
./BUILD.yaml`,
		},
		{
			Name:                "packages",
			T:                   t,
			Args:                []string{"run", "fixtures/scripts:pwd-packages"},
			ExitCode:            0,
			NoNestedApplication: true,
			StdoutSub: `.
./fixtures-pkgs-generic--something`,
		},
		// 		{
		// 			Name:              "origin nested",
		// 			T:                 t,
		// 			Args:              []string{"run", "-a", "fixtures", "//scripts:pwd-origin"},
		// 			ExitCode:          0,
		// 			NoNestedApplication: false,
		// 			StdoutSub: `.
		// ./BUILD.yaml`,
		// 		},
		// 		{
		// 			Name:              "packages nested",
		// 			T:                 t,
		// 			Args:              []string{"run", "fixtures/scripts:pwd-packages"},
		// 			ExitCode:          0,
		// 			NoNestedApplication: false,
		// 			StdoutSub: `.
		// ./fixtures-pkgs-generic--something`,
		// 		},
	}

	for _, test := range tests {
		test.Run()
	}
}

type CommandFixtureTest struct {
	Name                string
	T                   *testing.T
	Args                []string
	ExitCode            int
	NoNestedApplication bool
	StdoutSub           string
	NoStdoutSub         string
	StderrSub           string
	NoStderrSub         string
	Eval                func(t *testing.T, stdout, stderr string)
}

// Run executes the fixture test - do not forget to call this one
func (ft *CommandFixtureTest) Run() {
	if *dut {
		cmd.Execute()
		return
	}

	ft.T.Run(ft.Name, func(t *testing.T) {
		self, err := os.Executable()
		if err != nil {
			t.Fatalf("cannot identify test binary: %q", err)
		}
		cmd := exec.Command(self, append([]string{"--dut"}, ft.Args...)...)
		var (
			sout = bytes.NewBuffer(nil)
			serr = bytes.NewBuffer(nil)
		)
		cmd.Stdout = sout
		cmd.Stderr = serr
		cmd.Dir = "../../"
		if !ft.NoNestedApplication {
			env := os.Environ()
			env = append(env, "GORPA_NESTED_APPLICATION=true")
			cmd.Env = env
		}
		err = cmd.Run()

		var exitCode int
		if xerr, ok := err.(*exec.ExitError); ok {
			exitCode = xerr.ExitCode()
			err = nil
		}
		if err != nil {
			t.Fatalf("cannot re-run test binary: %q", err)
		}
		if exitCode != ft.ExitCode {
			t.Errorf("unepxected exit code: expected %d, actual %d (stderr: %s, stdout: %s)", ft.ExitCode, exitCode, serr.String(), sout.String())
		}
		var (
			stdout = sout.String()
			stderr = serr.String()
		)
		if !strings.Contains(stdout, ft.StdoutSub) {
			t.Errorf("stdout: expected to find \"%s\" in \"%s\"", ft.StdoutSub, stdout)
		}
		if ft.NoStdoutSub != "" && strings.Contains(stdout, ft.NoStdoutSub) {
			t.Errorf("stdout: expected not to find \"%s\" in \"%s\"", ft.NoStdoutSub, stdout)
		}
		if !strings.Contains(stderr, ft.StderrSub) {
			t.Errorf("stderr: expected to find \"%s\" in \"%s\"", ft.StderrSub, stderr)
		}
		if ft.NoStderrSub != "" && strings.Contains(stderr, ft.NoStderrSub) {
			t.Errorf("stderr: expected not to find \"%s\" in \"%s\"", ft.NoStderrSub, stderr)
		}
		if ft.Eval != nil {
			ft.Eval(t, stdout, stderr)
		}
	})
}
