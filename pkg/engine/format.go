package engine

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
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// FormatBUILDyaml formats a component's build.yaml file
func FormatBUILDyaml(out io.Writer, in io.Reader, fixIssues bool) error {
	var n yaml.Node
	err := yaml.NewDecoder(in).Decode(&n)
	if err != nil {
		return err
	}

	sortPackageDeps(&n)
	if fixIssues {
		replaceTypescriptPackageType(&n)
	}

	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	return enc.Encode(&n)
}

func sortPackageDeps(n *yaml.Node) {
	if len(n.Content) < 1 {
		return
	}

	nde := n.Content[0]
	for rootIdx, rootNde := range nde.Content {
		if rootNde.Value != "packages" || rootIdx == len(nde.Content)-1 {
			continue
		}

		nde := nde.Content[rootIdx+1]
		if len(nde.Content) < 1 {
			return
		}
		nde = nde.Content[0]

		for pkgIdx, pkgNde := range nde.Content {
			if pkgNde.Value != "deps" || pkgIdx == len(nde.Content)-1 {
				continue
			}

			nde := nde.Content[pkgIdx+1]
			sort.Slice(nde.Content, func(i, j int) bool { return nde.Content[i].Value < nde.Content[j].Value })
		}
	}
}

func replaceTypescriptPackageType(n *yaml.Node) {
	if len(n.Content) < 1 {
		return
	}

	nde := n.Content[0]
	for rootIdx, rootNde := range nde.Content {
		if rootNde.Value != "packages" || rootIdx == len(nde.Content)-1 {
			continue
		}

		nde := nde.Content[rootIdx+1]
		if len(nde.Content) < 1 {
			return
		}

		for _, nde := range nde.Content {
			tpe := searchInMapFor(nde, "type")
			if tpe == nil || tpe.Value != string(DeprecatedTypescriptPackage) {
				continue
			}
			tpe.Value = string(YarnPackage)

			nde = searchInMapFor(nde, "config")
			if nde == nil {
				continue
			}
			nde = searchInMapFor(nde, "commands")
			if nde == nil {
				continue
			}
			hasBuildCmd := searchInMapFor(nde, "build") != nil
			if hasBuildCmd {
				continue
			}
			nde.Content = append(nde.Content,
				&yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: "build",
				},
				&yaml.Node{
					Kind:  yaml.SequenceNode,
					Tag:   "!!seq",
					Style: yaml.FlowStyle,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "npx", Style: yaml.DoubleQuotedStyle},
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tsc", Style: yaml.DoubleQuotedStyle},
					},
				},
			)
		}
	}
}

func searchInMapFor(nde *yaml.Node, key string) (val *yaml.Node) {
	for pkgIdx, pkgNde := range nde.Content {
		if pkgNde.Value != key || pkgIdx == len(nde.Content)-1 {
			continue
		}

		return nde.Content[pkgIdx+1]
	}
	return nil
}
