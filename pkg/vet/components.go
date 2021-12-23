package vet

import (
	"bytes"
	"io/ioutil"
	"path/filepath"

	"github.com/bhojpur/gorpa/pkg/gorpa"
)

func init() {
	register(ComponentCheck("fmt", "ensures the BUILD.yaml of a component is Bhojpur GoRPA formatted", checkComponentsFmt))
}

func checkComponentsFmt(comp *gorpa.Component) ([]Finding, error) {
	fc, err := ioutil.ReadFile(filepath.Join(comp.Origin, "BUILD.yaml"))
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)
	err = gorpa.FormatBUILDyaml(buf, bytes.NewReader(fc), false)
	if err != nil {
		return nil, err
	}

	if bytes.EqualFold(buf.Bytes(), fc) {
		return nil, nil
	}

	return []Finding{
		{
			Component:   comp,
			Description: "component's BUILD.yaml is not formatted using `gorpa fmt`",
			Error:       false,
		},
	}, nil
}
