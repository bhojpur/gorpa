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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixtureLoadApplication(t *testing.T) {
	runDUT()

	tests := []*CommandFixtureTest{
		{
			Name:                "single application packages",
			T:                   t,
			Args:                []string{"collect", "-a", "fixtures/nested-ba/baa"},
			NoNestedApplication: true,
			ExitCode:            0,
			StdoutSub:           "pkg1:app",
		},
		{
			Name:                "application components",
			T:                   t,
			Args:                []string{"collect", "-a", "fixtures/nested-ba/baa", "components"},
			NoNestedApplication: true,
			ExitCode:            0,
			StdoutSub:           "//\npkg0\npkg1",
		},
		{
			Name:                "ignore nested applications",
			T:                   t,
			Args:                []string{"collect", "-a", "fixtures/nested-ba", "components"},
			NoNestedApplication: true,
			ExitCode:            1,
			StderrSub:           "pkg0:app: package \\\"baa/pkg0:app\\\" is unknown",
		},
		{
			Name:      "nested application packages",
			T:         t,
			Args:      []string{"collect", "-a", "fixtures/nested-ba"},
			StdoutSub: "pkg0:app",
			ExitCode:  0,
		},
		{
			Name:      "nested application components",
			T:         t,
			Args:      []string{"collect", "components", "-a", "fixtures/nested-ba"},
			StdoutSub: "pkg0",
			ExitCode:  0,
		},
		{
			Name:      "nested application scripts",
			T:         t,
			Args:      []string{"collect", "scripts", "-a", "fixtures/nested-ba"},
			StdoutSub: "baa/pkg1:echo\nbaa:echo",
			ExitCode:  0,
		},
		{
			Name:      "nested application override default args",
			T:         t,
			Args:      []string{"run", "-a", "fixtures/nested-ba", "baa/pkg1:echo"},
			StdoutSub: "hello root",
			ExitCode:  0,
		},
		{
			Name: "environment manifest",
			T:    t,
			Args: []string{"describe", "-a", "fixtures/nested-ba/baa", "environment-manifest"},
			Eval: func(t *testing.T, stdout, stderr string) {
				for _, k := range []string{"os", "arch", "foobar"} {
					if !strings.Contains(stdout, fmt.Sprintf("%s: ", k)) {
						t.Errorf("missing %s entry in environment manifest", k)
					}
				}
			},
			ExitCode: 0,
		},
	}

	for _, test := range tests {
		test.Run()
	}
}

func TestPackageDefinition(t *testing.T) {
	runDUT()

	type pkginfo struct {
		Metadata struct {
			Version string `json:"version"`
		} `json:"metadata"`
	}

	tests := []struct {
		Name    string
		Layouts []map[string]string
		Tester  []func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest
	}{
		{
			Name: "def change changes version",
			Layouts: []map[string]string{
				{
					"APPLICATION.yaml": "",
					"pkg1/BUILD.yaml":  "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"doesNotExist\"",
				},
				{
					"pkg1/BUILD.yaml": "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"alsoDoesNotExist\"",
				},
			},
			Tester: []func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest{
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							state["v"] = dest.Metadata.Version
						},
					}
				},
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							if state["v"] == dest.Metadata.Version {
								t.Errorf("definition change did not change version")
							}
						},
					}
				},
			},
		},
		{
			Name: "comp change doesnt change version",
			Layouts: []map[string]string{
				{
					"APPLICATION.yaml": "",
					"pkg1/BUILD.yaml":  "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"doesNotExist\"",
				},
				{
					"pkg1/BUILD.yaml": "const:\n  foobar: baz\npackages:\n- name: foo\n  type: generic\n  srcs:\n  - \"doesNotExist\"",
				},
			},
			Tester: []func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest{
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							state["v"] = dest.Metadata.Version
						},
					}
				},
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							if state["v"] != dest.Metadata.Version {
								t.Errorf("component change did change package version")
							}
						},
					}
				},
			},
		},
		{
			Name: "dependency def change changes version",
			Layouts: []map[string]string{
				{
					"APPLICATION.yaml": "",
					"pkg1/BUILD.yaml":  "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"doesNotExist\"\n- name: bar\n  type: generic\n  srcs:\n  - \"doesNotExist\"\n  deps:\n  - :foo",
				},
				{
					"pkg1/BUILD.yaml": "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"alsoDoesNotExist\"\n- name: bar\n  type: generic\n  srcs:\n  - \"doesNotExist\"\n  deps:\n  - :foo",
				},
			},
			Tester: []func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest{
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							state["v"] = dest.Metadata.Version
						},
					}
				},
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-a", loc, "-o", "json", "pkg1:bar"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							if state["v"] == dest.Metadata.Version {
								t.Errorf("dependency def change didn't change version")
							}
						},
					}
				},
			},
		},
		{
			Name: "build args dont change version",
			Layouts: []map[string]string{
				{
					"APPLICATION.yaml": "",
					"pkg1/BUILD.yaml":  "packages:\n- name: foo\n  type: generic\n  srcs:\n  - \"doesNotExist\"\n  config:\n    commands:\n    - [\"echo\", \"${msg}\"]",
				},
				{},
			},
			Tester: []func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest{
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-Dmsg=foo", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							state["v"] = dest.Metadata.Version
						},
					}
				},
				func(t *testing.T, loc string, state map[string]string) *CommandFixtureTest {
					return &CommandFixtureTest{
						T:    t,
						Args: []string{"describe", "-Dmsg=bar", "-a", loc, "-o", "json", "pkg1:foo"},
						Eval: func(t *testing.T, stdout, stderr string) {
							var dest pkginfo
							err := json.Unmarshal([]byte(stdout), &dest)
							if err != nil {
								fmt.Println(stdout)
								t.Fatal(err)
							}
							if state["v"] != dest.Metadata.Version {
								t.Errorf("build arg did change version")
							}
						},
					}
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			loc, err := ioutil.TempDir("", "pkgdeftest-*")
			if err != nil {
				t.Fatalf("cannot create temporary dir: %q", err)
			}

			state := make(map[string]string)
			for i, l := range test.Layouts {
				for k, v := range l {
					err := os.MkdirAll(filepath.Join(loc, filepath.Dir(k)), 0755)
					if err != nil && !os.IsExist(err) {
						t.Fatalf("cannot create filesystem layout: %q", err)
					}
					err = ioutil.WriteFile(filepath.Join(loc, k), []byte(v), 0644)
					if err != nil && !os.IsExist(err) {
						t.Fatalf("cannot create filesystem layout: %q", err)
					}
				}

				tester := test.Tester[i](t, loc, state)
				tester.Name = fmt.Sprintf("test-%003d", i)
				tester.Run()
			}
		})
	}
}
