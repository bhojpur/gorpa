//go:generate bash -c "cd web && yarn install && yarn build"
// +generate bash -c "go get github.com/GeertJohan/go.rice/rice && rice embed-go"

package graphview

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
	"net/http"
	"sort"

	rice "github.com/GeertJohan/go.rice"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
)

// Serve serves the dependency graph view for a package
func Serve(addr string, pkgs ...*gorpa.Package) error {
	http.HandleFunc("/graph.json", serveDepGraphJSON(pkgs))
	http.Handle("/", http.FileServer(rice.MustFindBox("web/dist").HTTPBox()))
	return http.ListenAndServe(addr, nil)
}

type graph struct {
	Nodes []node `json:"nodes"`
	Links []link `json:"links"`
}

type node struct {
	Name      string `json:"name"`
	Component string `json:"comp"`

	Type   string `json:"type"`
	TypeID int    `json:"typeid"`
}

type link struct {
	Source int   `json:"source"`
	Target int   `json:"target"`
	Path   []int `json:"path"`
}

func serveDepGraphJSON(pkgs []*gorpa.Package) http.HandlerFunc {
	var (
		nodes []node
		links []link
	)
	for _, p := range pkgs {
		n, l := computeDependencyGraph(p, len(nodes))
		nodes = append(nodes, n...)
		links = append(links, l...)
	}

	js, _ := json.Marshal(graph{Nodes: nodes, Links: links})
	return func(w http.ResponseWriter, r *http.Request) {
		//nolint:errcheck
		w.Write(js)
	}
}

func computeDependencyGraph(pkg *gorpa.Package, offset int) ([]node, []link) {
	var (
		tdeps   = append(pkg.GetTransitiveDependencies(), pkg)
		nodes   = make([]node, len(tdeps))
		nodeidx = make(map[string]int)
		typeidx = make(map[string]int)
		links   []link
		walk    func(pkg *gorpa.Package, path []int)
	)

	for i, p := range tdeps {
		nodes[i] = node{Name: p.FullName(), Component: p.C.Name, Type: getPackageType(p)}
		nodeidx[nodes[i].Name] = offset + i
		typeidx[nodes[i].Type] = 0
	}
	types := make([]string, 0, len(typeidx))
	for k := range typeidx {
		types = append(types, k)
	}
	sort.Strings(types)
	for i, k := range types {
		typeidx[k] = i
	}
	for i, n := range nodes {
		n.TypeID = typeidx[n.Type]
		nodes[i] = n
	}

	walk = func(p *gorpa.Package, path []int) {
		src := nodeidx[p.FullName()]
		for _, dep := range p.GetDependencies() {
			links = append(links, link{
				Source: src,
				Target: nodeidx[dep.FullName()],
				Path:   append(path, src),
			})
			walk(dep, append(path, src))
		}
	}
	walk(pkg, nil)

	return nodes, links
}

func getPackageType(pkg *gorpa.Package) (typen string) {
	switch c := pkg.Config.(type) {
	case gorpa.DockerPkgConfig:
		typen = "docker"
	case gorpa.GenericPkgConfig:
		typen = "generic"
	case gorpa.GoPkgConfig:
		typen = "go-" + string(c.Packaging)
	case gorpa.YarnPkgConfig:
		typen = "yarn-" + string(c.Packaging)
	}
	return typen
}
