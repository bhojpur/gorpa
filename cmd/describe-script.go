package cmd

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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bhojpur/gorpa/pkg/prettyprint"
)

// describeScriptCmd represents the describeTree command
var describeScriptCmd = &cobra.Command{
	Use:   "script",
	Short: "Describes a script",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, _, script, exists := getTarget(args, true)
		if !exists || script == nil {
			log.Fatal("needs a script")
		}

		w := getWriterFromFlags(cmd)
		if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
			w.FormatString = `Name:{{"\t"}}{{ .FullName }}
{{ if .Description }}Description:{{"\t"}}{{ .Description }}{{ end }}
Type:{{"\t"}}{{ .Type }}
Workdir Layout:{{"\t"}}{{ .WorkdirLayout }}
{{ if .Dependencies -}}
Dependencies:
{{- range $k, $v := .Dependencies }}
{{"\t"}}{{ $v.FullName -}}{{"\t"}}{{ $v.Version -}}
{{ end -}}
{{ end }}
`
		}

		desc := newScriptDescription(script)
		err := w.Write(desc)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	describeCmd.AddCommand(describeScriptCmd)
	addFormatFlags(describeScriptCmd)
}
