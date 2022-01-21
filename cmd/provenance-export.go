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
	"encoding/base64"
	"encoding/json"
	"io"
	"os"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
	"github.com/bhojpur/gorpa/pkg/provutil"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/bom/pkg/provenance"
)

// provenanceExportCmd represents the provenance export command
var provenanceExportCmd = &cobra.Command{
	Use:   "export <package>",
	Short: "Exports the provenance bundle of a (previously built) package",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bundleFN, pkgFN, pkg, err := getProvenanceTarget(cmd, args)
		if err != nil {
			log.WithError(err).Fatal("cannot locate bundle")
		}

		decode, _ := cmd.Flags().GetBool("decode")
		out := json.NewEncoder(os.Stdout)

		export := func(env *provenance.Envelope) error {
			if !decode {
				return out.Encode(env)
			}

			dec, err := base64.StdEncoding.DecodeString(env.Payload)
			if err != nil {
				return err
			}

			// we make a Marshal(Unmarshal(...)) detour here to ensure we're still outputing
			// newline delimited JSON. We have no idea how the payload actually looks like, just
			// that it's valid JSON.
			var decc map[string]interface{}
			err = json.Unmarshal(dec, &decc)
			if err != nil {
				return err
			}
			err = out.Encode(decc)
			if err != nil {
				return err
			}

			return nil
		}

		if pkg == nil {
			f, err := os.Open(bundleFN)
			if err != nil {
				log.WithError(err).Fatal("cannot open attestation bundle")
			}
			defer f.Close()
			err = provutil.DecodeBundle(f, export)
		} else {
			err = gorpa.AccessAttestationBundleInCachedArchive(pkgFN, func(bundle io.Reader) error {
				return provutil.DecodeBundle(bundle, export)
			})
		}
		if err != nil {
			log.WithError(err).Fatal("cannot extract attestation bundle")
		}
	},
}

func init() {
	provenanceExportCmd.Flags().Bool("decode", false, "decode the base64 payload of the envelopes")

	provenanceCmd.AddCommand(provenanceExportCmd)
	addBuildFlags(provenanceExportCmd)
}
