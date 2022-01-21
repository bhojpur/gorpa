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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

func TestCheckDockerCopyFromPackage(t *testing.T) {
	tests := []struct {
		Name       string
		Dockerfile string
		Deps       []string
		Findings   []string
	}{
		{
			Name: "true positive copy",
			Dockerfile: `FROM alpine:latest
COPY from-some-pkg--build/hello.txt hello.txt`,
			Findings: []string{
				"Dockerfile copies from from-some-pkg--build/hello.txt which looks like a package path, but no dependency satisfies it",
			},
		},
		{
			Name: "true negative copy",
			Dockerfile: `FROM alpine:latest
COPY from-some-pkg--build/hello.txt hello.txt`,
			Deps: []string{"from-some-pkg:build"},
		},
		{
			Name: "true positive add",
			Dockerfile: `FROM alpine:latest
ADD from-some-pkg--build/hello.txt hello.txt`,
			Findings: []string{
				"Dockerfile copies from from-some-pkg--build/hello.txt which looks like a package path, but no dependency satisfies it",
			},
		},
		{
			Name: "true negative add",
			Dockerfile: `FROM alpine:latest
ADD from-some-pkg--build/hello.txt hello.txt`,
			Deps: []string{"from-some-pkg:build"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			failOnErr := func(err error) {
				if err != nil {
					t.Fatalf("cannot set up test: %q", err)
				}
			}

			tmpdir, err := ioutil.TempDir("", "gorpa-test-*")
			failOnErr(err)
			// defer os.RemoveAll(tmpdir)

			var pkgdeps string
			failOnErr(ioutil.WriteFile(filepath.Join(tmpdir, "APPLICATION.yaml"), []byte("environmentManifest:\n  - name: \"docker\"\n    command: [\"echo\"]"), 0644))
			for _, dep := range test.Deps {
				segs := strings.Split(dep, ":")
				loc := filepath.Join(tmpdir, segs[0])
				failOnErr(os.MkdirAll(loc, 0755))
				failOnErr(ioutil.WriteFile(filepath.Join(loc, "BUILD.yaml"), []byte(fmt.Sprintf(`packages:
- name: %s
  type: generic`, segs[1])), 0755))

				if pkgdeps == "" {
					pkgdeps = "\n  deps:\n"
				}
				pkgdeps += fmt.Sprintf("  - %s\n", dep)
			}
			failOnErr(os.MkdirAll(filepath.Join(tmpdir, "test-pkg"), 0755))
			failOnErr(ioutil.WriteFile(filepath.Join(tmpdir, "test-pkg", "Dockerfile"), []byte(test.Dockerfile), 0644))
			failOnErr(ioutil.WriteFile(filepath.Join(tmpdir, "test-pkg", "BUILD.yaml"), []byte(fmt.Sprintf(`packages:
- name: docker
  type: docker
  config:
    dockerfile: Dockerfile%s
`, pkgdeps)), 0644))

			ba, err := gorpa.FindApplication(tmpdir, gorpa.Arguments{}, "", "")
			failOnErr(err)
			pkg, ok := ba.Packages["test-pkg:docker"]
			if !ok {
				t.Fatalf("cannot find test package: test-pkg:docker")
			}

			findings, err := checkDockerCopyFromPackage(pkg)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}

			var fs []string
			if len(findings) > 0 {
				fs = make([]string, len(findings))
				for i := range findings {
					fs[i] = findings[i].Description
				}
			}
			if diff := cmp.Diff(test.Findings, fs); diff != "" {
				t.Errorf("MakeGatewayInfo() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
