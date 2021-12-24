package gorpa

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/in-toto/in-toto-golang/in_toto"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"sigs.k8s.io/bom/pkg/provenance"
)

const (
	provenanceBundleFilename = "provenance-bundle.jsonl"
)

// writeProvenance produces a provenanceWriter which ought to be used during package builds
func writeProvenance(p *Package, buildctx *buildContext, builddir string, subjects []in_toto.Subject) (err error) {
	if !p.C.W.Provenance.Enabled {
		return nil
	}

	bundle := make(map[string]struct{})
	err = p.getDependenciesProvenanceBundles(buildctx, bundle)
	if err != nil {
		return err
	}

	if p.C.W.Provenance.SLSA {
		env, err := p.ProduceSLSAEnvelope(subjects)
		if err != nil {
			return err
		}

		entry, err := json.Marshal(env)
		if err != nil {
			return err
		}

		bundle[string(entry)] = struct{}{}
	}

	f, err := os.OpenFile(filepath.Join(builddir, provenanceBundleFilename), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot write provenance for %s: %w", p.FullName(), err)
	}
	defer f.Close()

	for entry := range bundle {
		_, err = f.WriteString(entry + "\n")
		if err != nil {
			return fmt.Errorf("cannot write provenance for %s: %w", p.FullName(), err)
		}
	}
	return nil
}

func (p *Package) getDependenciesProvenanceBundles(buildctx *buildContext, out map[string]struct{}) error {
	deps := p.GetTransitiveDependencies()
	for _, dep := range deps {
		loc, exists := buildctx.LocalCache.Location(dep)
		if !exists {
			return PkgNotBuiltErr{dep}
		}

		err := extractBundleFromCachedArchive(dep, loc, out)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractBundleFromCachedArchive(dep *Package, loc string, out map[string]struct{}) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error extracting provenance bundle from %s: %w", loc, err)
		}
	}()

	f, err := os.Open(loc)
	if err != nil {
		return err
	}
	defer f.Close()

	g, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer g.Close()

	var (
		prevBundleSize = len(out)
		bundleFound    bool
	)

	a := tar.NewReader(g)
	var hdr *tar.Header
	for {
		hdr, err = a.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}

		if hdr.Name != "./"+provenanceBundleFilename && hdr.Name != "package/"+provenanceBundleFilename {
			continue
		}

		// TOOD(cw): use something other than a scanner. We've seen "Token Too Long" in first trials already.
		scan := bufio.NewScanner(io.LimitReader(a, hdr.Size))
		for scan.Scan() {
			out[scan.Text()] = struct{}{}
		}
		if scan.Err() != nil {
			return scan.Err()
		}
		bundleFound = true
		break
	}
	if err != nil {
		return
	}

	if !bundleFound {
		return fmt.Errorf("dependency %s has no provenance bundle", dep.FullName())
	}

	log.WithField("prevBundleSize", prevBundleSize).WithField("newBundleSize", len(out)).WithField("loc", loc).Debug("extracted bundle from cached archive")

	return nil
}

func (p *Package) ProduceSLSAEnvelope(subjects []in_toto.Subject) (res *provenance.Envelope, err error) {
	git := p.C.Git()
	if git.Commit == "" || git.Origin == "" {
		return nil, xerrors.Errorf("Git provenance is unclear - do not have any Git info")
	}

	now := time.Now()
	pred := provenance.NewSLSAPredicate()
	if p.C.Git().Dirty {
		files, err := p.inTotoMaterials()
		if err != nil {
			return nil, err
		}
		pred.Materials = files
	} else {
		pred.Materials = []in_toto.ProvenanceMaterial{
			{URI: "git+" + git.Origin, Digest: in_toto.DigestSet{"sha256": git.Commit}},
		}
	}

	pred.Builder = in_toto.ProvenanceBuilder{
		ID: "github.com/bhojpur/gorpa:" + Version,
	}
	pred.Metadata = &in_toto.ProvenanceMetadata{
		Completeness: in_toto.ProvenanceComplete{
			Arguments:   true,
			Environment: false,
			Materials:   true,
		},
		Reproducible:   false,
		BuildStartedOn: &now,
	}
	pred.Recipe = in_toto.ProvenanceRecipe{
		Type:       fmt.Sprintf("https://github.com/bhojpur/gorpa/build@%s:%d", p.Type, buildProcessVersions[p.Type]),
		Arguments:  os.Args,
		EntryPoint: p.FullName(),
	}

	stmt := provenance.NewSLSAStatement()
	stmt.Subject = subjects
	stmt.Predicate = pred

	payload, err := json.MarshalIndent(stmt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("cannot marshal provenance for %s: %w", p.FullName(), err)
	}

	var sigs []interface{}
	if p.C.W.Provenance.key != nil {
		sig, err := in_toto.GenerateSignature(payload, *p.C.W.Provenance.key)
		if err != nil {
			return nil, fmt.Errorf("cannot sign provenance for %s: %w", p.FullName(), err)
		}
		sigs = append(sigs, sig)
	}

	return &provenance.Envelope{
		PayloadType: in_toto.PayloadType,
		Payload:     base64.StdEncoding.EncodeToString(payload),
		Signatures:  sigs,
	}, nil
}

func (p *Package) inTotoMaterials() ([]in_toto.ProvenanceMaterial, error) {
	res := make([]in_toto.ProvenanceMaterial, 0, len(p.Sources))
	for _, src := range p.Sources {
		f, err := os.Open(src)
		if err != nil {
			return nil, xerrors.Errorf("cannot compute hash of %s: %w", src, err)
		}

		hash := sha256.New()
		_, err = io.Copy(hash, f)
		if err != nil {
			return nil, xerrors.Errorf("cannot compute hash of %s: %w", src, err)
		}
		f.Close()

		res = append(res, in_toto.ProvenanceMaterial{
			URI: "file://" + strings.TrimPrefix(strings.TrimPrefix(src, p.C.W.Origin), "/"),
			Digest: in_toto.DigestSet{
				"sha256": fmt.Sprintf("%x", hash.Sum(nil)),
			},
		})
	}
	return res, nil
}

type fileset map[string]struct{}

func computeFileset(dir string) (fileset, error) {
	res := make(fileset)
	err := filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		fn := strings.TrimPrefix(path, dir)
		res[fn] = struct{}{}
		return nil
	})
	log.WithField("prefix", dir).WithField("res", res).Debug("computing fileset")
	return res, err
}

// Sub produces a new fileset with all entries from the other fileset subjectraced
func (fset fileset) Sub(other fileset) fileset {
	res := make(fileset, len(fset))
	for fn := range fset {
		if _, ok := other[fn]; ok {
			continue
		}
		res[fn] = struct{}{}
	}
	return res
}

func (fset fileset) Subjects(base string) ([]in_toto.Subject, error) {
	res := make([]in_toto.Subject, 0, len(fset))
	for src := range fset {
		f, err := os.Open(filepath.Join(base, src))
		if err != nil {
			return nil, xerrors.Errorf("cannot compute hash of %s: %w", src, err)
		}

		hash := sha256.New()
		_, err = io.Copy(hash, f)
		if err != nil {
			return nil, xerrors.Errorf("cannot compute hash of %s: %w", src, err)
		}
		f.Close()

		res = append(res, in_toto.Subject{
			Name: src,
			Digest: in_toto.DigestSet{
				"sha256": fmt.Sprintf("%x", hash.Sum(nil)),
			},
		})
	}
	return res, nil
}
