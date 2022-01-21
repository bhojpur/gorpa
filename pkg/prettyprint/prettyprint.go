package prettyprint

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
	"io"
	"text/tabwriter"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Format is an output format for pretty printing
type Format string

const (
	// TemplateFormat produces text/template-based output
	TemplateFormat Format = "template"
	// JSONFormat produces JSON output
	JSONFormat Format = "json"
	// YAMLFormat produces YAML output
	YAMLFormat Format = "yaml"
)

// Writer preconfigures the write function
type Writer struct {
	Out          io.Writer
	Format       Format
	FormatString string
}

// Write prints the input in the preconfigred way
func (w *Writer) Write(in interface{}) error {
	switch w.Format {
	case TemplateFormat:
		return writeTemplate(w.Out, in, w.FormatString)
	case JSONFormat:
		return json.NewEncoder(w.Out).Encode(in)
	case YAMLFormat:
		return yaml.NewEncoder(w.Out).Encode(in)
	default:
		return fmt.Errorf("unknown format: %s", w.Format)
	}
}

func writeTemplate(out io.Writer, in interface{}, tplc string) error {
	tpl := template.New("template")
	tpl, err := tpl.Parse(tplc)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	return tpl.Execute(w, in)
}
