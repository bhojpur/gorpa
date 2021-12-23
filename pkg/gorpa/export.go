package gorpa

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// CopyApplication copies all folders/files from an application to a destination.
// If strict is true we'll only copy the files that leeway actully knows are source files.
// Otherwise we'll copy all files that are not excluded by the variant.
func CopyApplication(dst string, application *Application, strict bool) error {
	out, err := exec.Command("cp", "-R", application.Origin, dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}

	return DeleteNonApplicationFiles(dst, application, strict)
}

// DeleteNonApplicationFiles removes all files that do not belong to an application.
// If strict is true this function deletes all files that are not listed as source in a package.
// If strict is fales this function deletes files excluded by a variant.
func DeleteNonApplicationFiles(dst string, application *Application, strict bool) (err error) {
	var (
		excl = make(map[string]struct{})
		incl = make(map[string]struct{})
	)
	if strict {
		for _, pkg := range application.Packages {
			for _, s := range pkg.Sources {
				rels := strings.TrimPrefix(s, application.Origin)
				incl[rels] = struct{}{}

				// package sources are files only - we need to include their parent directories as well
				for p := filepath.Dir(rels); p != "/"; p = filepath.Dir(p) {
					incl[p] = struct{}{}
				}
			}
		}
	} else {
		err = filepath.Walk(application.Origin, func(path string, info os.FileInfo, err error) error {
			s := strings.TrimPrefix(path, application.Origin)
			incl[s] = struct{}{}
			return nil
		})
		if err != nil {
			return err
		}

		if application.SelectedVariant != nil {
			vinc, vexc, err := application.SelectedVariant.ResolveSources(application, dst)
			if err != nil {
				return err
			}

			for _, p := range vinc {
				incl[strings.TrimPrefix(p, dst)] = struct{}{}
			}
			for _, p := range vexc {
				excl[strings.TrimPrefix(p, dst)] = struct{}{}
			}
		}
	}

	// keep if incl and not excl
	return filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if path == dst {
			return nil
		}

		s := strings.TrimPrefix(path, dst)
		_, inc := incl[s]
		_, exc := excl[s]
		lg := log.WithField("inc", inc).WithField("exc", exc).WithField("s", s).WithField("dst", dst)
		if inc && !exc {
			lg.Debug("not deleting file")
			return nil
		}
		lg.Debug("deleting file")

		return os.RemoveAll(path)
	})
}
