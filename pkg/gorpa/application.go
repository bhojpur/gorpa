package gorpa

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/trace"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/imdario/mergo"
	"github.com/minio/highwayhash"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"

	"github.com/bhojpur/gorpa/pkg/doublestar"
)

// Application is the root container of all compoments. All components are named relative
// to the origin of this application.
type Application struct {
	DefaultTarget       string              `yaml:"defaultTarget,omitempty"`
	ArgumentDefaults    map[string]string   `yaml:"defaultArgs,omitempty"`
	DefaultVariant      *PackageVariant     `yaml:"defaultVariant,omitempty"`
	Variants            []*PackageVariant   `yaml:"variants,omitempty"`
	EnvironmentManifest EnvironmentManifest `yaml:"environmentManifest,omitempty"`

	Origin          string                `yaml:"-"`
	Components      map[string]*Component `yaml:"-"`
	Packages        map[string]*Package   `yaml:"-"`
	Scripts         map[string]*Script    `yaml:"-"`
	SelectedVariant *PackageVariant       `yaml:"-"`
	GitCommit       string                `yaml:"-"`

	ignores []string
}

// EnvironmentManifest is a collection of environment manifest entries
type EnvironmentManifest []EnvironmentManifestEntry

// Write writes the manifest to the writer
func (mf EnvironmentManifest) Write(out io.Writer) error {
	for _, e := range mf {
		_, err := fmt.Fprintf(out, "%s: %s\n", e.Name, e.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Hash produces the hash of this manifest
func (mf EnvironmentManifest) Hash() (string, error) {
	key, err := hex.DecodeString(contentHashKey)
	if err != nil {
		return "", err
	}

	hash, err := highwayhash.New(key)
	if err != nil {
		return "", err
	}

	err = mf.Write(hash)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// EnvironmentManifestEntry represents an entry in the environment manifest
type EnvironmentManifestEntry struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`

	Value   string `yaml:"-"`
	Builtin bool   `yaml:"-"`
}

const (
	builtinEnvManifestGOOS   = "goos"
	builtinEnvManifestGOARCH = "goarch"
)

var defaultEnvManifestEntries = map[PackageType]EnvironmentManifest{
	"": []EnvironmentManifestEntry{
		{Name: "os", Command: []string{builtinEnvManifestGOOS}, Builtin: true},
		{Name: "arch", Command: []string{builtinEnvManifestGOARCH}, Builtin: true},
	},
	GenericPackage: []EnvironmentManifestEntry{},
	DockerPackage:  []EnvironmentManifestEntry{
		// We do not pull the docker version here as that would make package versions dependent on a connection
		// to a Docker daemon. As the environment manifest is resolved on application load one would always need
		// a connection to a Docker daemon just to run e.g. gorpa collect.
		//
		// If you want the behaviour described above, add the following to your APPLICATION.yaml:
		//   environmentManifest:
		//     - name: docker
		//       command: ["docker", "version", "--format", "{{.Client.Version}} {{.Server.Version}}"]
		//
	},
	GoPackage: []EnvironmentManifestEntry{
		{Name: "go", Command: []string{"go", "version"}},
	},
	YarnPackage: []EnvironmentManifestEntry{
		{Name: "yarn", Command: []string{"yarn", "-v"}},
		{Name: "node", Command: []string{"node", "--version"}},
	},
}

// ShouldIgnoreComponent returns true if a file should be ignored for a component listing
func (ws *Application) ShouldIgnoreComponent(path string) bool {
	return ws.ShouldIgnoreSource(path)
}

// ShouldIgnoreSource returns true if a file should be ignored for a source listing
func (ws *Application) ShouldIgnoreSource(path string) bool {
	for _, ptn := range ws.ignores {
		if strings.Contains(path, ptn) {
			return true
		}
	}
	return false
}

// FindNestedApplications loads nested applications
func FindNestedApplications(path string, args Arguments, variant string) (res Application, err error) {
	rootWS, err := loadApplicationYAML(path)
	if err != nil {
		return Application{}, err
	}

	var ignore doublestar.IgnoreFunc
	if fc, _ := ioutil.ReadFile(filepath.Join(path, ".gorpaignore")); len(fc) > 0 {
		ignore = doublestar.IgnoreStrings(strings.Split(string(fc), "\n"))
	}

	wss, err := doublestar.Glob(path, "**/APPLICATION.yaml", ignore)
	if err != nil {
		return
	}

	// deepest applications first
	sort.Slice(wss, func(i, j int) bool {
		return strings.Count(wss[i], string(os.PathSeparator)) > strings.Count(wss[j], string(os.PathSeparator))
	})

	loadedApplications := make(map[string]*Application)
	for _, wspath := range wss {
		wspath = strings.TrimSuffix(strings.TrimSuffix(wspath, "APPLICATION.yaml"), "/")
		log := log.WithField("wspath", wspath)
		log.Debug("loading (possibly nested) application")

		sws, err := loadApplication(context.Background(), wspath, args, variant, &loadApplicationOpts{
			PrelinkModifier: func(packages map[string]*Package) {
				for otherloc, otherws := range loadedApplications {
					relativeOrigin := filepathTrimPrefix(otherloc, wspath)

					for k, p := range otherws.Packages {
						var otherKey string
						if strings.HasPrefix(k, "//") {
							otherKey = fmt.Sprintf("%s%s", relativeOrigin, strings.TrimPrefix(k, "//"))
						} else {
							otherKey = fmt.Sprintf("%s/%s", relativeOrigin, k)
						}
						p.fullNameOverride = otherKey
						packages[otherKey] = p

						log.WithField("relativeOrigin", relativeOrigin).WithField("package", otherKey).Debug("prelinking previously loaded application")
					}
				}
			},
			ArgumentDefaults: rootWS.ArgumentDefaults,
		})
		if err != nil {
			return res, err
		}
		loadedApplications[wspath] = &sws
		res = sws
	}

	// now that we've loaded and linked the main application, we need to fix the location names and indices
	var (
		newComps   = make(map[string]*Component)
		newScripts = make(map[string]*Script)
	)
	for _, pkg := range res.Packages {
		name := filepathTrimPrefix(pkg.C.Origin, res.Origin)
		if name == "" {
			name = "//"
		}
		pkg.C.Name = name
		newComps[name] = pkg.C
		log.WithField("origin", pkg.C.Origin).WithField("name", name).Debug("renamed component")
	}
	for otherloc, otherws := range loadedApplications {
		relativeOrigin := filepathTrimPrefix(otherloc, path)

		for k, p := range otherws.Scripts {
			var otherKey string
			if strings.HasPrefix(k, "//") {
				otherKey = fmt.Sprintf("%s%s", relativeOrigin, strings.TrimPrefix(k, "//"))
			} else if relativeOrigin == "" {
				otherKey = k
			} else {
				otherKey = fmt.Sprintf("%s/%s", relativeOrigin, k)
			}
			newScripts[otherKey] = p
			log.WithField("k", otherKey).WithField("otherloc", otherloc).Debug("new script")
		}
	}
	res.Components = newComps
	res.Scripts = newScripts

	return
}

func filepathTrimPrefix(path, prefix string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, prefix), string(os.PathSeparator))
}

// loadApplicationYAML loads a application's YAML file only - does not linking or processing of any kind.
// Probably you want to use loadApplication instead.
func loadApplicationYAML(path string) (Application, error) {
	root := filepath.Join(path, "APPLICATION.yaml")
	fc, err := ioutil.ReadFile(root)
	if err != nil {
		return Application{}, err
	}
	var application Application
	err = yaml.Unmarshal(fc, &application)
	if err != nil {
		return Application{}, err
	}
	application.Origin, err = filepath.Abs(filepath.Dir(root))
	if err != nil {
		return Application{}, err
	}
	return application, nil
}

type loadApplicationOpts struct {
	PrelinkModifier  func(map[string]*Package)
	ArgumentDefaults map[string]string
}

func loadApplication(ctx context.Context, path string, args Arguments, variant string, opts *loadApplicationOpts) (Application, error) {
	ctx, task := trace.NewTask(ctx, "loadApplication")
	defer task.End()

	application, err := loadApplicationYAML(path)
	if err != nil {
		return Application{}, err
	}

	if variant != "" {
		for _, vnt := range application.Variants {
			if vnt.Name == variant {
				application.SelectedVariant = vnt
				break
			}
		}
	} else if application.DefaultVariant != nil {
		application.SelectedVariant = application.DefaultVariant
		log.WithField("defaults", *application.SelectedVariant).Debug("applying default variant")
	}

	var ignores []string
	ignoresFile := filepath.Join(application.Origin, ".gorpaignore")
	if _, err := os.Stat(ignoresFile); !os.IsNotExist(err) {
		fc, err := ioutil.ReadFile(ignoresFile)
		if err != nil {
			return Application{}, err
		}
		ignores = strings.Split(string(fc), "\n")
	}
	otherWS, err := doublestar.Glob(application.Origin, "**/APPLICATION.yaml", application.ShouldIgnoreSource)
	if err != nil {
		return Application{}, err
	}
	for _, ows := range otherWS {
		dir := filepath.Dir(ows)
		if dir == application.Origin {
			continue
		}

		ignores = append(ignores, dir)
	}
	application.ignores = ignores
	log.WithField("ignores", application.ignores).Debug("computed application ignores")

	if len(opts.ArgumentDefaults) > 0 {
		if application.ArgumentDefaults == nil {
			application.ArgumentDefaults = make(map[string]string)
		}
		for k, v := range opts.ArgumentDefaults {
			application.ArgumentDefaults[k] = v
		}
		log.WithField("rootDefaultArgs", opts.ArgumentDefaults).Debug("installed root application defaults")
	}

	log.WithField("defaultArgs", application.ArgumentDefaults).Debug("applying application defaults")
	for key, val := range application.ArgumentDefaults {
		if args == nil {
			args = make(map[string]string)
		}

		_, alreadySet := args[key]
		if alreadySet {
			continue
		}

		args[key] = val
	}

	comps, err := discoverComponents(ctx, &application, args, application.SelectedVariant, opts)
	if err != nil {
		return application, err
	}
	application.Components = make(map[string]*Component)
	application.Packages = make(map[string]*Package)
	application.Scripts = make(map[string]*Script)
	packageTypesUsed := make(map[PackageType]struct{})
	for _, comp := range comps {
		application.Components[comp.Name] = comp

		for _, pkg := range comp.Packages {
			application.Packages[pkg.FullName()] = pkg
			packageTypesUsed[pkg.Type] = struct{}{}
		}
		for _, script := range comp.Scripts {
			application.Scripts[script.FullName()] = script
		}
	}

	// with all packages loaded we can compute the env manifest, becuase now we know which package types are actually
	// used, hence know the default env manifest entries.
	application.EnvironmentManifest, err = buildEnvironmentManifest(application.EnvironmentManifest, packageTypesUsed)
	if err != nil {
		return Application{}, err
	}

	// if this application has a Git repo at its root, resolve its commit hash
	application.GitCommit, err = getGitCommit(application.Origin)
	if err != nil {
		log.WithField("application", application.Origin).WithError(err).Warn("cannot get Git commit")
		err = nil
	}

	// now that we have all components/packages, we can link things
	if opts != nil && opts.PrelinkModifier != nil {
		opts.PrelinkModifier(application.Packages)
	}
	for _, pkg := range application.Packages {
		err := pkg.link(application.Packages)
		if err != nil {
			return application, xerrors.Errorf("linking error in package %s: %w", pkg.FullName(), err)
		}
	}
	for _, script := range application.Scripts {
		err := script.link(application.Packages)
		if err != nil {
			return application, xerrors.Errorf("linking error in script %s: %w", script.FullName(), err)
		}
	}

	// dependency cycles break the version computation and are not allowed
	for _, p := range application.Packages {
		c, err := p.findCycle()
		if err != nil {
			log.WithError(err).WithField("pkg", p.FullName()).Warn("internal error - skipping cycle detection")
			continue
		}
		if len(c) == 0 {
			continue
		}

		return application, xerrors.Errorf("dependency cycle found: %s", strings.Join(c, " -> "))
	}

	// at this point all packages are fully loaded and we can compute the version, as well as resolve builtin variables
	for _, pkg := range application.Packages {
		err = pkg.resolveBuiltinVariables()
		if err != nil {
			return application, xerrors.Errorf("cannot resolve builtin variables %s: %w", pkg.FullName(), err)
		}
	}

	return application, nil
}

func getGitCommit(loc string) (string, error) {
	gitfc := filepath.Join(loc, ".git")
	stat, err := os.Stat(gitfc)
	if err != nil || !stat.IsDir() {
		return "", nil
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = gitfc
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// buildEnvironmentManifest executes the commands of an env manifest and updates the values
func buildEnvironmentManifest(entries EnvironmentManifest, pkgtpes map[PackageType]struct{}) (res EnvironmentManifest, err error) {
	t0 := time.Now()

	envmf := make(map[string]EnvironmentManifestEntry, len(entries))
	for _, e := range defaultEnvManifestEntries[""] {
		envmf[e.Name] = e
	}
	for tpe := range pkgtpes {
		for _, e := range defaultEnvManifestEntries[tpe] {
			envmf[e.Name] = e
		}
	}
	for _, e := range entries {
		e := e
		envmf[e.Name] = e
	}

	for k, e := range envmf {
		if e.Builtin {
			switch e.Command[0] {
			case builtinEnvManifestGOARCH:
				e.Value = runtime.GOARCH
			case builtinEnvManifestGOOS:
				e.Value = runtime.GOOS
			}
			res = append(res, e)
			continue
		}

		out := bytes.NewBuffer(nil)
		cmd := exec.Command(e.Command[0], e.Command[1:]...)
		cmd.Stdout = out
		err := cmd.Run()
		if err != nil {
			return nil, xerrors.Errorf("cannot resolve env manifest entry %v: %w", k, err)
		}
		e.Value = strings.TrimSpace(out.String())

		res = append(res, e)
	}

	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })

	log.WithField("time", time.Since(t0).String()).WithField("res", res).Debug("built environment manifest")

	return
}

// FindApplication looks for a APPLICATION.yaml file within the path. If multiple such files are found,
// an error is returned.
func FindApplication(path string, args Arguments, variant string) (Application, error) {
	return loadApplication(context.Background(), path, args, variant, &loadApplicationOpts{})
}

// discoverComponents discovers components in an application
func discoverComponents(ctx context.Context, application *Application, args Arguments, variant *PackageVariant, opts *loadApplicationOpts) ([]*Component, error) {
	defer trace.StartRegion(context.Background(), "discoverComponents").End()

	path := application.Origin
	pths, err := doublestar.Glob(path, "**/BUILD.yaml", application.ShouldIgnoreSource)
	if err != nil {
		return nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	cchan := make(chan *Component, 20)

	for _, pth := range pths {
		if application.ShouldIgnoreComponent(pth) {
			continue
		}

		pth := pth
		eg.Go(func() error {
			comp, err := loadComponent(ctx, application, pth, args, variant)
			if err != nil {
				return err
			}
			cchan <- &comp
			return nil
		})
	}
	var (
		comps []*Component
		wg    sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for c := range cchan {
			// filter variant-excluded components and all their packages
			if filterExcludedComponents(variant, c) {
				continue
			}

			comps = append(comps, c)
		}
	}()
	err = eg.Wait()
	close(cchan)
	if err != nil {
		return nil, err
	}
	wg.Wait()

	return comps, nil
}

// filterExcludedComponents returns true if the component is excluded by the variant.
// This function also removes all dependencies to excluded components.
func filterExcludedComponents(variant *PackageVariant, c *Component) (ignoreComponent bool) {
	if variant == nil {
		return false
	}
	if variant.ExcludeComponent(c.Name) {
		log.WithField("component", c.Name).Debug("selected variant excludes this component")
		return true
	}

	for _, p := range c.Packages {
		for i, dep := range p.Dependencies {
			segs := strings.Split(dep, ":")
			if len(segs) != 2 {
				continue
			}

			if variant.ExcludeComponent(segs[0]) {
				p.Dependencies[i] = p.Dependencies[len(p.Dependencies)-1]
				p.Dependencies = p.Dependencies[:len(p.Dependencies)-1]
			}
		}
	}
	return false
}

// loadComponent loads a component from a BUILD.yaml file
func loadComponent(ctx context.Context, application *Application, path string, args Arguments, variant *PackageVariant) (c Component, err error) {
	defer trace.StartRegion(context.Background(), "loadComponent").End()
	trace.Log(ctx, "component", path)
	defer func() {
		if err != nil {
			err = xerrors.Errorf("%s: %w", path, err)
		}
	}()

	fc, err := ioutil.ReadFile(path)
	if err != nil {
		return Component{}, err
	}

	// we attempt to load the constants of a component first so that we can add it to the args
	var compconst struct {
		Constants Arguments `yaml:"const"`
	}
	err = yaml.Unmarshal(fc, &compconst)
	if err != nil {
		return Component{}, err
	}
	compargs := make(Arguments)
	for k, v := range args {
		compargs[k] = v
	}
	for k, v := range compconst.Constants {
		// constants overwrite args
		compargs[k] = v
	}

	// replace build args
	var rfc []byte = fc
	if len(args) > 0 {
		rfc = replaceBuildArguments(fc, compargs)
	}

	var (
		comp    Component
		rawcomp struct {
			Packages []yaml.Node
		}
	)
	err = yaml.Unmarshal(rfc, &comp)
	if err != nil {
		return comp, err
	}
	err = yaml.Unmarshal(fc, &rawcomp)
	if err != nil {
		return comp, err
	}

	name := strings.TrimPrefix(strings.TrimPrefix(filepath.Dir(path), application.Origin), "/")
	if name == "" {
		name = "//"
	}

	comp.W = application
	comp.Name = name
	comp.Origin = filepath.Dir(path)

	// if this component has a Git repo at its root, resolve its commit hash
	comp.gitCommit, err = getGitCommit(comp.Origin)
	if err != nil {
		log.WithField("comp", comp.Name).WithError(err).Warn("cannot get Git commit")
		err = nil
	}

	for i, pkg := range comp.Packages {
		pkg.C = &comp
		if pkg.Type == "typescript" {
			log.WithField("pkg", pkg.FullName()).Warn("package uses deprecated \"typescript\" type - use \"yarn\" instead")
			pkg.Type = YarnPackage
		}

		pkg.Definition, err = yaml.Marshal(&rawcomp.Packages[i])
		if err != nil {
			return comp, xerrors.Errorf("%s: %w", comp.Name, err)
		}

		pkg.originalSources = pkg.Sources
		pkg.Sources, err = resolveSources(pkg.C.W, pkg.C.Origin, pkg.Sources, false)
		if err != nil {
			return comp, xerrors.Errorf("%s: %w", comp.Name, err)
		}

		// add additional sources to package sources
		completeSources := make(map[string]struct{})
		for _, src := range pkg.Sources {
			completeSources[src] = struct{}{}
		}
		for _, src := range pkg.Config.AdditionalSources() {
			fn, err := filepath.Abs(filepath.Join(comp.Origin, src))
			if err != nil {
				return comp, xerrors.Errorf("%s: %w", comp.Name, err)
			}
			if _, err := os.Stat(fn); os.IsNotExist(err) {
				return comp, xerrors.Errorf("%s: %w", comp.Name, err)
			}
			if _, found := completeSources[fn]; found {
				continue
			}

			completeSources[fn] = struct{}{}
		}
		if vnt := pkg.C.W.SelectedVariant; vnt != nil {
			incl, excl, err := vnt.ResolveSources(pkg.C.W, pkg.C.Origin)
			if err != nil {
				return comp, xerrors.Errorf("%s: %w", comp.Name, err)
			}
			for _, i := range incl {
				completeSources[i] = struct{}{}
			}
			for _, i := range excl {
				delete(completeSources, i)
			}
			log.WithField("pkg", pkg.Name).WithField("variant", variant).WithField("excl", excl).WithField("incl", incl).WithField("package", pkg.FullName()).Debug("applying variant")
		}
		pkg.Sources = make([]string, len(completeSources))
		i := 0
		for src := range completeSources {
			pkg.Sources[i] = src
			i++
		}

		// re-set the version relevant arguments to <name>: <value>
		for i, argdep := range pkg.ArgumentDependencies {
			val, ok := args[argdep]
			if !ok {
				val = "<not-set>"
			}
			pkg.ArgumentDependencies[i] = fmt.Sprintf("%s: %s", argdep, val)
		}

		// make all dependencies fully qualified
		for idx, dep := range pkg.Dependencies {
			if !strings.HasPrefix(dep, ":") {
				continue
			}

			pkg.Dependencies[idx] = comp.Name + dep
		}
		// make all layout entries full qualified
		if pkg.Layout == nil {
			pkg.Layout = make(map[string]string)
		}
		for dep, loc := range pkg.Layout {
			if !strings.HasPrefix(dep, ":") {
				continue
			}

			delete(pkg.Layout, dep)
			pkg.Layout[comp.Name+dep] = loc
		}

		// apply variant config
		if vnt := pkg.C.W.SelectedVariant; vnt != nil {
			if vntcfg, ok := vnt.Config(pkg.Type); ok {
				err = mergeConfig(pkg, vntcfg)
				if err != nil {
					return comp, xerrors.Errorf("%s: %w", comp.Name, err)
				}
			}

			err = mergeEnv(pkg, vnt.Environment)
			if err != nil {
				return comp, xerrors.Errorf("%s: %w", comp.Name, err)
			}
		}
	}

	for _, scr := range comp.Scripts {
		scr.C = &comp

		// fill in defaults
		if scr.Type == "" {
			scr.Type = BashScript
		}
		if scr.WorkdirLayout == "" {
			scr.WorkdirLayout = WorkdirOrigin
		}

		// make all dependencies fully qualified
		for idx, dep := range scr.Dependencies {
			if !strings.HasPrefix(dep, ":") {
				continue
			}

			scr.Dependencies[idx] = comp.Name + dep
		}
	}

	return comp, nil
}

func mergeConfig(pkg *Package, src PackageConfig) error {
	if src == nil {
		return nil
	}

	switch pkg.Config.(type) {
	case YarnPkgConfig:
		dst := pkg.Config.(YarnPkgConfig)
		in, ok := src.(YarnPkgConfig)
		if !ok {
			return xerrors.Errorf("cannot merge %s onto %s", reflect.TypeOf(src).String(), reflect.TypeOf(dst).String())
		}
		err := mergo.Merge(&dst, in)
		if err != nil {
			return err
		}
		pkg.Config = dst
	case GoPkgConfig:
		dst := pkg.Config.(GoPkgConfig)
		in, ok := src.(GoPkgConfig)
		if !ok {
			return xerrors.Errorf("cannot merge %s onto %s", reflect.TypeOf(src).String(), reflect.TypeOf(dst).String())
		}
		err := mergo.Merge(&dst, in)
		if err != nil {
			return err
		}
		pkg.Config = dst
	case DockerPkgConfig:
		dst := pkg.Config.(DockerPkgConfig)
		in, ok := src.(DockerPkgConfig)
		if !ok {
			return xerrors.Errorf("cannot merge %s onto %s", reflect.TypeOf(src).String(), reflect.TypeOf(dst).String())
		}
		err := mergo.Merge(&dst, in)
		if err != nil {
			return err
		}
		pkg.Config = dst
	case GenericPkgConfig:
		dst := pkg.Config.(GenericPkgConfig)
		in, ok := src.(GenericPkgConfig)
		if !ok {
			return xerrors.Errorf("cannot merge %s onto %s", reflect.TypeOf(src).String(), reflect.TypeOf(dst).String())
		}
		err := mergo.Merge(&dst, in)
		if err != nil {
			return err
		}
		pkg.Config = dst
	default:
		return xerrors.Errorf("unknown config type %s", reflect.ValueOf(pkg.Config).Elem().Type().String())
	}
	return nil
}

func mergeEnv(pkg *Package, src []string) error {
	env := make(map[string]string, len(pkg.Environment))
	for _, set := range [][]string{pkg.Environment, src} {
		for _, kv := range set {
			segs := strings.Split(kv, "=")
			if len(segs) < 2 {
				return xerrors.Errorf("environment variable must have format ENV=VAR: %s", kv)
			}

			env[segs[0]] = strings.Join(segs[1:], "=")
		}

	}

	pkg.Environment = make([]string, 0, len(env))
	for k, v := range env {
		pkg.Environment = append(pkg.Environment, fmt.Sprintf("%s=%s", k, v))
	}
	return nil
}
