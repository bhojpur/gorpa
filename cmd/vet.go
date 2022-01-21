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
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bhojpur/gorpa/pkg/prettyprint"
	"github.com/bhojpur/gorpa/pkg/vet"
)

// versionCmd represents the version command
var vetCmd = &cobra.Command{
	Use:   "vet [ls]",
	Short: "Validates the Bhojpur GoRPA application",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := getWriterFromFlags(cmd)
		if len(args) > 0 && args[0] == "ls" {
			if w.FormatString == "" && w.Format == prettyprint.TemplateFormat {
				w.FormatString = `{{ range . -}}
{{ .Name }}{{"\t"}}{{ .Description }}
{{ end }}`
			}
			err := w.Write(vet.Checks())
			if err != nil {
				return err
			}
		}

		ba, err := getApplication()
		if err != nil {
			return err
		}

		var opts []vet.RunOpt
		if checks, _ := cmd.Flags().GetStringArray("checks"); len(checks) > 0 {
			opts = append(opts, vet.WithChecks(checks))
		}
		if pkgs, _ := cmd.Flags().GetStringArray("packages"); len(pkgs) > 0 {
			idx := make(vet.StringSet)
			for _, p := range pkgs {
				idx[p] = struct{}{}
			}
			opts = append(opts, vet.OnPackages(idx))
		}
		if comps, _ := cmd.Flags().GetStringArray("components"); len(comps) > 0 {
			idx := make(vet.StringSet)
			for _, p := range comps {
				idx[p] = struct{}{}
			}
			opts = append(opts, vet.OnComponents(idx))
		}

		findings, errs := vet.Run(ba, opts...)
		if ignoreWarnings, _ := cmd.Flags().GetBool("ignore-warnings"); ignoreWarnings {
			n := 0
			for _, x := range findings {
				if x.Error {
					findings[n] = x
					n++
				}
			}
			findings = findings[:n]
		}

		if len(errs) != 0 {
			for _, err := range errs {
				log.Error(err.Error())
			}
			return nil
		}

		if w.FormatString == "" && w.Format == prettyprint.TemplateFormat {
			w.FormatString = `{{ range . }}
{{"\033"}}[90m{{ if .Package -}}📦{{"\t"}}{{ .Package.FullName }}{{ else if .Component }}🗃️{{"\t"}}{{ .Component.Name }}{{ end }}
✔️ {{ .Check }}{{"\033"}}[0m
{{ if .Error -}}❌{{ else }}⚠️{{ end -}}{{"\t"}}{{ .Description }}
{{ end }}`
		}
		err = w.Write(findings)
		if err != nil {
			return err
		}

		if len(findings) == 0 {
			os.Exit(0)
		} else {
			os.Exit(128)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(vetCmd)

	vetCmd.Flags().StringArray("checks", nil, "run these checks only")
	vetCmd.Flags().StringArray("packages", nil, "run checks on these packages only")
	vetCmd.Flags().StringArray("components", nil, "run checks on these components only")
	vetCmd.Flags().Bool("ignore-warnings", false, "ignores all warnings")
	addFormatFlags(vetCmd)
}
