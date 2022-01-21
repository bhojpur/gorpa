package provutil

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
	"strings"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
	"github.com/in-toto/in-toto-golang/in_toto"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/bom/pkg/provenance"
)

type Assertion struct {
	Name        string
	Description string
	Run         func(stmt *provenance.Statement) []Violation
	RunEnvelope func(env *provenance.Envelope) []Violation
}

type Violation struct {
	Assertion *Assertion
	Statement *provenance.Statement
	Desc      string
}

func (v Violation) String() string {
	if v.Statement == nil {
		return fmt.Sprintf("failed %s: %s", v.Assertion.Name, v.Desc)
	}

	return fmt.Sprintf("%s failed %s: %s", v.Statement.Predicate.Recipe.EntryPoint, v.Assertion.Name, v.Desc)
}

type Assertions []*Assertion

func (a Assertions) AssertEnvelope(env *provenance.Envelope) (failed []Violation) {
	for _, as := range a {
		if as.RunEnvelope == nil {
			continue
		}

		res := as.RunEnvelope(env)
		for i := range res {
			res[i].Assertion = as
		}
		failed = append(failed, res...)
	}
	return
}

func (a Assertions) AssertStatement(stmt *provenance.Statement) (failed []Violation) {
	// we must not keep a reference to stmt around - it will change for each invocation
	s := *stmt
	for _, as := range a {
		if as.Run == nil {
			continue
		}

		res := as.Run(stmt)
		for i := range res {
			res[i].Statement = &s
			res[i].Assertion = as
		}
		failed = append(failed, res...)
	}
	return
}

var AssertBuiltWithGorpa = &Assertion{
	Name:        "built-with-gorpa",
	Description: "ensures all bundle entries have been built with Bhojpur GoRPA",
	Run: func(stmt *provenance.Statement) []Violation {
		if strings.HasPrefix(stmt.Predicate.Builder.ID, gorpa.ProvenanceBuilderID) {
			return nil
		}

		return []Violation{
			{Desc: "was not built using Bhojpur GoRPA"},
		}
	},
}

func AssertBuiltWithGorpaVersion(version string) *Assertion {
	return &Assertion{
		Name:        "built-with-gorpa-version",
		Description: "ensures all bundle entries which have been built using Bhojpur GoRPA, used version " + version,
		Run: func(stmt *provenance.Statement) []Violation {
			if !strings.HasPrefix(stmt.Predicate.Builder.ID, gorpa.ProvenanceBuilderID) {
				return nil
			}

			if stmt.Predicate.Builder.ID != gorpa.ProvenanceBuilderID+":"+version {
				return []Violation{{Desc: "was built using Bhojpur GoRPA version " + strings.TrimPrefix(stmt.Predicate.Builder.ID, gorpa.ProvenanceBuilderID+":")}}
			}

			return nil
		},
	}
}

var AssertGitMaterialOnly = &Assertion{
	Name:        "git-material-only",
	Description: "ensures all subjects were built from Git material only",
	Run: func(stmt *provenance.Statement) []Violation {
		for _, m := range stmt.Predicate.Materials {
			if strings.HasPrefix(m.URI, "git+") || strings.HasPrefix(m.URI, "git://") {
				continue
			}

			return []Violation{{
				Desc: "contains non-Git material, e.g. " + m.URI,
			}}
		}
		return nil
	},
}

func AssertSignedWith(key in_toto.Key) *Assertion {
	return &Assertion{
		Name:        "signed-with",
		Description: "ensures all envelopes are signed with the given key",
		RunEnvelope: func(env *provenance.Envelope) []Violation {
			for _, s := range env.Signatures {
				raw, err := json.Marshal(s)
				if err != nil {
					return []Violation{{Desc: "assertion error: " + err.Error()}}
				}
				var sig in_toto.Signature
				err = json.Unmarshal(raw, &sig)
				if err != nil {
					return []Violation{{Desc: "assertion error: " + err.Error()}}
				}

				err = in_toto.VerifySignature(key, sig, []byte(env.Payload))
				if err != nil {
					log.WithError(err).WithField("signature", sig).Debug("signature does not match")
					continue
				}

				return nil
			}
			return []Violation{{Desc: "not signed with the given key"}}
		},
	}
}
