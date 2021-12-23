package cmd

import (
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bhojpur/gorpa/pkg/gorpa"
	"github.com/bhojpur/gorpa/pkg/prettyprint"
)

type fileDescription struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	Package string `json:"package" yaml:"package"`
}

type variantDescription struct {
	Name    string `json:"name" yaml:"name"`
	Sources struct {
		Include []string `json:"include" yaml:"include"`
		Exclude []string `json:"exclude" yaml:"exclude"`
	} `json:"srcs" yaml:"srcs"`
	Environment []string                                 `json:"env" yaml:"env"`
	Config      map[gorpa.PackageType]configDescription `json:"config" yaml:"config"`
}

// collectCmd represents the collect command
var collectCmd = &cobra.Command{
	Use:   "collect [components|packages|scripts|files]",
	Short: "Collects all packages in an application",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		application, err := getApplication()
		if err != nil {
			log.Fatal(err)
		}

		var tpe string
		if len(args) == 0 {
			tpe = "packages"
		} else {
			tpe = args[0]
		}

		selectStr, _ := cmd.Flags().GetString("select")
		var selector func(c *gorpa.Component) bool
		segs := strings.Split(selectStr, "=")
		if len(selectStr) == 0 {
			selector = func(c *gorpa.Component) bool {
				return true
			}
		} else if len(segs) == 1 {
			selector = func(c *gorpa.Component) bool {
				_, ok := c.Constants[segs[0]]
				return ok
			}
		} else if len(segs) == 2 {
			selector = func(c *gorpa.Component) bool {
				return c.Constants[segs[0]] == segs[1]
			}
		} else {
			log.Fatal("selector must either be a constant name or const=value")
		}

		w := getWriterFromFlags(cmd)
		switch tpe {
		case "components":
			if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
				w.FormatString = `{{ range . }}{{ .Name }}{{"\n"}}{{ end }}`
			}
			decs := make([]componentDescription, 0, len(application.Components))
			for _, comp := range application.Components {
				if !selector(comp) {
					continue
				}
				decs = append(decs, newComponentDescription(comp))
			}
			sort.Slice(decs, func(i, j int) bool { return decs[i].Name < decs[j].Name })
			err = w.Write(decs)
			if err != nil {
				log.Fatal(err)
			}
		case "packages":
			if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
				w.FormatString = `{{ range . }}{{ .Metadata.FullName }}{{"\t"}}{{ .Metadata.Version }}{{"\n"}}{{ end }}`
			}
			decs := make([]packageDescription, 0, len(application.Packages))
			for _, pkg := range application.Packages {
				if !selector(pkg.C) {
					continue
				}

				decs = append(decs, newPackageDesription(pkg))
			}
			sort.Slice(decs, func(i, j int) bool { return decs[i].Metadata.FullName < decs[j].Metadata.FullName })
			err = w.Write(decs)
			if err != nil {
				log.Fatal(err)
			}
		case "scripts":
			if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
				w.FormatString = `{{ range . }}{{ .FullName }}{{ if .Description }}{{"\t"}}{{ .Description }}{{ end }}{{"\n"}}{{ end }}`
			}
			decs := make([]scriptDescription, 0, len(application.Scripts))
			for _, scr := range application.Scripts {
				if !selector(scr.C) {
					continue
				}

				decs = append(decs, newScriptDescription(scr))
			}
			sort.Slice(decs, func(i, j int) bool { return decs[i].FullName < decs[j].FullName })
			err = w.Write(decs)
			if err != nil {
				log.Fatal(err)
			}
		case "files":
			if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
				w.FormatString = `{{ range . }}{{ .Name }}{{"\t"}}{{ .Version }}{{"\n"}}{{ end }}`
			}
			decs := make([]fileDescription, 0, len(application.Packages))
			for _, pkg := range application.Packages {
				if !selector(pkg.C) {
					continue
				}

				pkgn := pkg.FullName()
				mf, err := pkg.ContentManifest()
				if err != nil {
					log.Fatal(err)
				}
				fs := make([]fileDescription, len(mf))
				for i, f := range mf {
					segs := strings.Split(f, ":")
					fs[i] = fileDescription{Name: segs[0], Version: segs[1], Package: pkgn}
				}

				decs = append(decs, fs...)
			}
			sort.Slice(decs, func(i, j int) bool { return decs[i].Name < decs[j].Name })
			err = w.Write(decs)
			if err != nil {
				log.Fatal(err)
			}
		case "variants":
			if w.Format == prettyprint.TemplateFormat && w.FormatString == "" {
				w.FormatString = `{{ range . }}{{ .Name }}{{"\n"}}{{ end }}`
			}
			decs := make([]variantDescription, len(application.Variants))
			for i, v := range application.Variants {
				decs[i] = variantDescription{
					Name:        v.Name,
					Environment: v.Environment,
					Config:      make(map[gorpa.PackageType]configDescription),
				}
				decs[i].Sources.Exclude = v.Sources.Exclude
				decs[i].Sources.Include = v.Sources.Include
				for _, t := range []gorpa.PackageType{gorpa.DockerPackage, gorpa.GenericPackage, gorpa.GoPackage, gorpa.YarnPackage} {
					vntcfg, ok := v.Config(t)
					if !ok {
						continue
					}
					decs[i].Config[t] = newConfigDescription(t, vntcfg)
				}
			}
			err = w.Write(decs)
			if err != nil {
				log.Fatal(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(collectCmd)
	collectCmd.Flags().StringP("select", "l", "", "Filters packages by component constants (e.g. `-l foo` finds all packages whose components have a foo constant and `-l foo=bar` only prints packages whose components have a foo=bar constant)")

	addFormatFlags(collectCmd)
}
