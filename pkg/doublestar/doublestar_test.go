package doublestar_test

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
	"testing"

	"github.com/bhojpur/gorpa/pkg/doublestar"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		Pattern string
		Path    string
		Match   bool
	}{
		{"**", "/", true},
		{"**/*.go", "foo.go", true},
		{"**/foo.go", "foo.go", true},
		{"**/BUILD.yaml", "fixtures/scripts/BUILD.yaml", true},
		{"**/*.go", "a/b/c/foo.go", true},
		{"**/*.go", "/c/foo.go", true},
		{"**/*.go", "a/b/c/foo.txt", false},
		{"**/*.go", "a/b/c", false},
		{"**/*.go", "/a/b/c", false},
		{"/a/b/**", "/a/b/c", true},
		{"/a/b/**", "/a/b/c/d/e/f/g", true},
		{"/a/b/**", "/a/b", false},
		{"/a/b/**", "a/b/c", false},
		{"/a/b/**/c", "/a/b/c", true},
		{"/a/b/**/c", "/a/b/1/2/3/4/c", true},
		{"/a/b/**/c/*.go", "/a/b/1/2/3/4/c/foo.go", true},
		{"/a/b/**/c/*.go", "/a/b/1/2/3/4/c/foo.txt", false},
		{"/a/b/**/**/c", "/a/b/1/2/3/4/c", true},
		{"/a/b/**/**/c", "/a/b/1/c", true},
		{"/a/b/**/c/**/d", "/a/b/1/c/2/d", true},
		{"/a/b/**/c/**/d", "/a/b/1/c/2", false},
		{"*/*.go", "src/foo.go", true},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%03d_%s_%s", i, test.Pattern, test.Path), func(t *testing.T) {
			match, err := doublestar.Match(test.Pattern, test.Path)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}
			if match != test.Match {
				t.Errorf("unexpected match: expected %v, got %v", test.Match, match)
			}
		})
	}
}
