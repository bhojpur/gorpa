# Bhojpur GoRPA - Builder, Packager, Assembler
An efficient Go-based Rapid Product Assembly software tool used within the Bhojpur.NET Platform ecosystem. It is used for validation, building, and packaging of different applications and/or services hosted in the SaaS platform. It has a heavy caching build system for Go, Yarn, and Docker software projects.

Some of the features of Bhojpur GoRPA are:
- **source dependent versions**: the Bhojpur GoRPA computes the version of a package based on the sources, dependencies, and configuration that makes up this package. There's no need (or means) to manually version packages.
- **two-level package cache**: the Bhojpur GoRPA caches its build results locally and remotely. The remote cache (e.g., a Google Cloud Storage bucket) means builds can share their results and thus become drastically faster.
- **parallel builds**: since the Bhojpur GoRPA understands all the dependencies of your packages, it can build them as parallel as possible.
- **built-in support for Yarn and Go**: the Bhojpur GoRPA knows how to link, build, and test Yarn and Go packages and applications. This makes building software written in those programming languages very straight forward.
- **build arguments**: the Bhojpur GoRPA supports build arguments which can parametrize packages at build time. We support version dependent arguments (where the version depends on the argument value), component-wide constants and application-level defaults.
- **rich CLI**: the Bhojpur GoRPA's CLI supports deep inspection of the application and its structure. Its output is easy to understand and looks good.

The Bhojpur GoRPA structures a repository at three different levels:
- The **application** is the root of all operations. All component names are relative to this path. No relevant file must be placed outside the application. The application root is marked with a `APPLICATION.yaml` file.
- A **components** is single piece of standalone software. Each folder in the application, which contains a `BUILD.yaml` file, is a component. The Components are identifed by their path relative to the application root.
- **Packages** are the buildable unit in the Bhojpur GoRPA. Each component can define multiple packages in its build file. The Packages are identified by their name prefixed with the component name, e.g. some-component:pkg.

# Installation
The Bhojpur GoRPA assumes that it is running on a Linux or macOS operating system. It is very very unlikely that this runs on Windows out-of-the-box.
To install, just download and unpack a [release](https://github.com/bhojpur/gorpa/releases).

# Build setup

## Application
Place a file named `APPLICATION.yaml` in the root of your working folder. For convenience sake, you should set the `GORPA_APPLICATION_ROOT` environment variable to the path of that application.
For example:
```
touch APPLICATION.yaml
export GORPA_APPLICATION_ROOT=$PWD
```

The `APPLICATION.yaml` may contain some default settings for the application:
```YAML
# defaultTarget is package we build when just running `gorpa build`
defaultTarget: some/package:name
#defaultArgs are key=value pairs setting default values for build arguments
defaultArgs:
  key: value
```

## Component
Place a `BUILD.yaml` in a folder somewhere in your Application to make that folder a component. A `BUILD.yaml` primarily contains the packages of that component, but can also contain constant values (think of them as metadata). For example:
```YAML
# const defines component-wide constants which can be used much like build arguments. Only string keys and values are supported.
const:
  internalName: example
  someRandomProperty: value
packages:
- ...
scripts:
- ...
```

## Package
A package is an entry in a `BUILD.yaml` in the `packages` section. All packages share the following fields:
```YAML
# name is the component-wide unique name of this package
name: must-not-contain-spaces
# Package type must be one of: go, yarn, docker, generic
type: generic
# Sources list all sources of this package. Entries can be double-star globs and are relative to the component root.
# Avoid listing sources outside the component folder.
srcs:
- "**/*.yaml"
- "glob/**/path"
# Deps list dependencies to other packages which must be built prior to building this package. How these dependencies are made
# available during build depends on the package type.
deps:
- some/other:package
# Argdeps makes build arguments version relevant. i.e. if the value of a build arg listed here changes, so does the package version.
argdeps:
- someBuildArg
# Env is a list of key=value pair environment variables available during package build
env:
- CGO_ENABLED=0
# Config configures the package build depending on the package type. See below for details
config:
  ...
```

## Script
Scripts are a great way to automate the tasks during development time (think [`yarn scripts`](https://classic.yarnpkg.com/en/docs/package-json#toc-scripts)).
Unlike packages they do not run in isolation, by default, but do have access to the original application.
What makes scripts special is that they can dependent on packages, which become available to a script in the PATH and as environment variables.

Under the `scripts` key in the component's `BUILD.yaml` add:
```YAML
# name is the component-wide unique name of script. Packages and scripts do NOT share a namespace.
# You can have a package called foo and a script called foo within the same component.
name: some-script-name
# description provides a short synopsis of the script. Shown when running `gorpa collect scripts`.
description: A sentence describing what the script is good for.
# Deps list dependencies to packages (NOT scripts) which must be built prior to running this script.
# All built dependencies get added to the PATH environment variable. This is handy if your application
# contains tools you want to use in a script.
deps:
- some/other:package
# Env sets environment variables which are present during script execution.
env:
- MESSAGE=hello
# Workdir changes the workdir location/layout of working folder of the script. The following choices are available:
# - origin (default): execute the script in the directory of the containing component in the original application.
#                     This is the default mode and handy if one wants to automate tasks in the development application.
# - packages:         produces a filesystem layout much like during a generic package build where all deps are
#                     found by their name in a temporary directory. This provides some isolation from the original
#                     application, while giving full access to the built dependencies.
workdir: origin
# The actual script. For now, only bash scripts are supported. The shebang is added automatically.
script: |
  echo $MESSAGE, this is where the script goes
  if [ "A$(ps -o comm= -p $$)" = "Abash" ]; then
    echo "it's the bash alright"
  fi
  echo "build args work to: ${myBuildArg}"
```

### Build arguments

In a package definition one can use _build arguments_. Build args have the form of `${argumentName}` and are string-replaced when the package is loaded.
**It's advisable to use build args only within the `config` section of packages**. Constants and built-in build args do not even work outside of the config section.

The Bhojpur GoRPA supports built-in build arguments:
- `__pkg_version` resolves to the Bhojpur GoRPA version hash of a component.

### Go packages
```YAML
config:
  # Packaging method. See https://godoc.org/github.com/bhojpur/gorpa/pkg/gorpa#GoPackaging for details. Defaults to library.
  packaging: library
  # If true Bhojpur GoRPA runs `go generate -v ./...` prior to testing/building. Defaults to false.
  generate: false
  # If true disables `go test -v ./...`
  dontTest: false
  # If true disables the enforcement of `go fmt`. By default, if the code is not go formatted, the build fails.
  dontCheckGoFmt: false
  # If true disables the linting stage.
  dontLint: false
  # Overrides the `go build .` command. Supersedes buildFlags.
  buildCommand: []
  # [DEPRECATED: use buildCommand instead] A list of flags passed to `go build`. Useful for passing `ldflags`.
  buildFlags: []
  # Command that's executed to lint the code
  lintCommand: ["golangci-lint", "run"]
  # GoKart is a static security analysis tool for Go (https://github.com/praetorian-inc/gokart). The Bhojpur GoRPA supports the construction
  # of analayzer.yaml file for GoKart based on the package dependencies. This is useful for detecting unsanitised input from API surfaces.
  gokart:
    enabled: false
    apiDepsPattern: 'reg-exp\/matching-go-package\/import-names'
```

### Yarn packages
```YAML
config:
  # yarnlock is the path to the yarn.lock used to build this package. Defaults to `yarn.lock`. Useful when building packages in a Yarn application setup.
  # Automatically added to the package sources.
  yarnlock: "yarn.lock"
  # tsconfig is the path to the tsconfig.json used to build this package. Detauls to `tsconfig.json`
  # Automatically added to the package sources.
  tsconfig: "tsconfig.json"
  # packaging method. See https://godoc.org/github.com/bhojpur/gorpa/pkg/gorpa#YarnPackaging for details.
  # Defaults to library
  packaging: library
  # If true disables `yarn test`
  dontTest: false
  # commands overrides the default commands executed during build
  commands:
    install: ["yarn", "install"]
    build: ["yarn", "build"]
    test: ["yarn", "test"]
```

### Docker packages
Docker packages have a default "retagging" behaviour: even when a Docker package is built already, i.e. it's GoRPA build version didn't change,
then the Bhojpur GoRPA will ensure that an image exists with the names specified in the package config. For example, if a Docker package has `gorpa/some-package:${version}` specified,
and `${version}` changes, but otherwise the package has been built before, then Bhojpur GoRPA will "re-tag" the previously built image to be available under `gorpa/some-package:${version}`.
This behaviour can be disabled using `--dont-retag`.
```YAML
config:
  # Dockerfile is the name of the Dockerfile to be built. Automatically added to the package sources.
  dockerfile: "Dockerfile"
  # Metadata produces a metadata.yaml file in the resulting package tarball.
  metadata:
    foo: bar
  # build args are Docker build arguments. Often we just pass the Bhojpur GoRPA build arguments along here.
  buildArgs:
  - arg=value
  - other=${someBuildArg}
  # image lists the Docker tags the Bhojpur GoRPA will use and push to
  image:
  - bhojpur/gorpa:latest
  - bhojpur/gorpa:${__pkg_version}
```

### Generic packages
```YAML
config:
  # A list of commands to execute. Beware that the commands are not executed in a shell. If you need shell features (e.g. wildcards or pipes),
  # wrap your command in `sh -c`. Generic packages without commands result in an empty tar file.
  commands:
  - ["echo", "hello world"]
  - ["sh", "-c", "ls *"]
```

## Package Variants
The Bhojpur GoRPA supports build-time variance through "package variants". Those variants are defined at the application-level and can modify the list of sources, environment variables and config of packages.
For example consider an `APPLICATION.yaml` with this variants section:
```YAML
variants:
- name: nogo
  srcs:
    exclude:
    - "**/*.go"
  config:
    go:
      buildFlags:
        - tags: foo
```

This application has a (non-sensical) `nogo` variant that, if enabled, excludes all the Go source code files from all the packages.
It also changes the config of all Go packages to include the `-tags foo` flag. You can explore the effects of a variant using `collect` and `describe`, e.g. `gorpa --variant nogo collect files` vs `gorpa collect files`.
You can list all variants in an Application using `gorpa collect variants`.

## Environment Manifest
The Bhojpur GoRPA does not control the environment in which it builds the packages, but assumes that all required tools are available already (e.g. `go` or `yarn`).
This however can lead to subtle failure modes where a package built in one enviroment ends up being used in another, because no matter whihc of the environment they were built in, they get the same version.

To prevent such issues, the Bhojpur GoRPA computes an _environment manifest_ which contains the versions of the tools used, as well as some platform information.
The entries in that manifest depend on the package types used by that application, e.g. if only `Go` packages exist in the application, only `go version`, [GOOS and GOARCH](https://golang.org/pkg/runtime/#pkg-constants) will be part of the manifest.
You can inspect an Application's environment manifest using `gorpa describe environment-manifest`.

You can add your own entries to an Application's environment manifest in the `APPLICATION.yaml` like so:
```YAML
environmentManifest:
  - name: gcc
    command: ["gcc", "--version"]
```

Using this mechanism you can also overwrite the default manifest entries, e.g. "go" or "yarn".

## Nested Applications
The Bhojpur GoRPA has some experimental support for nested applications, e.g. a structure like this one:
```
/application
/application/APPLICATION.yaml
/application/comp1/BUILD.yaml
/application/otherApplication/APPLICATION.yaml
/application/otherApplication/comp2/BUILD.yaml
```

By default, the Bhojpur GoRPA would just ignore the nested `otherApplication/` folder and everything below, because of `otherApplication/APPLICATION.yaml`. When nested application support is enabled though, the `otherApplication/` would be loaded as if it stood alone and merged into `/application`. For example:
```
$ export GORPA_NESTED_APPLICATION=true
$ gorpa collect
comp1:app
otherApplication/comp2:app
otherApplication/comp2:lib
```

- **inner applications are loaded as if they stood alone**: when the Bhojpur GoRPA loads any nested application, it does so as if that application stood for itself, i.e. were not nested. This means that all components are relative to that application root. In particular, dependencies remain stable no matter if an application is nested or not. e.g. `comp2:app` depending on `comp2:lib` works irregardless of application nesting.
- **nested dependencies**: dependencies from another application into a nested one is possible and behave as if all packages were in the same application, e.g. `comp1:app` could depend on `otherApplication/comp2:app`. Dependencies out of a nested application are not allowed, e.g. `otherApplication/comp2:app` cannot depend on `comp1:app`.
- **default arguments**: there is one exception to the "standalone", that is `defaultArgs`. The `defaultArgs` of the root application override the defaults of the nested applications. This is demonstrated by the Bhojpur GoRPA's test fixtures, where the message changes depending on the application that's loaded:
  ```
  $ export GORPA_NESTED_APPLICATION=true
  $ gorpa run fixtures/nested-ws/wsa/pkg1:echo
  hello world

  $ gorpa run -w fixtures/nested-ws wsa/pkg1:echo
  hello root
  ```
- **variants**: only the root Application's variants matter. Even if the nested application defined any, they'd simply be ignored.

# Configuration
The Bhojpur GoRPA is configured exclusively through the APPLICATION.yaml/BUILD.yaml files and environment variables. The following environment
variables have an effect on the Bhojpur GoRPA:
- `GORPA_APPLICATION_ROOT`: contains the path where to look for a APPLICATION .yaml file. Can also be set using --application.
- `GORPA_REMOTE_CACHE_BUCKET`: enables remote caching using GCP buckets. Set this variable to the bucket name used for caching. When this variable is set, the Bhojpur GoRPA expects "gsutil" in the path configured and authenticated so that it can work with the bucket.
- `GORPA_CACHE_DIR`: location of the local build cache. The directory does not have to exist yet.
- `GORPA_BUILD_DIR`: working location of the Bhojpur GoRPA (i.e. where the actual builds happen). This location will see heavy I/O, which makes it advisable to place this on a fast SSD or in RAM.
- `GORPA_YARN_MUTEX`: configures the mutex flag the Bhojpur GoRPA will pass to Yarn. Defaults to "network". See https://yarnpkg.com/lang/en/docs/cli/#toc-concurrency-and-mutex for possible values.
- `GORPA_EXPERIMENTAL`: enables some of the experimental features
- `GORPA_NESTED_APPLICATION`: enables nested applications. By default, the Bhojpur GoRPA ignores everything below another `APPLICATION.yaml`, but if this environment variable is set, then the Bhojpur GoRPA will try and link packages from the other application as if they were part of the parent one. This does not work for scripts yet.

# Provenance (SLSA) - EXPERIMENTAL
The Bhojpur GoRPA can produce provenance information as part of a build. At the moment only [SLSA](https://slsa.dev/spec/v0.1/) is supported. This supoprt is **experimental**.

Provenance generation is enabled in the `APPLICATION.yaml` file.
```YAML
provenance:
  enabled: true
  slsa: true
```

Once enabled, all packages carry an [attestation bundle](https://github.com/in-toto/attestation/blob/main/spec/bundle.md) which is compliant with the [SLSA v0.2 spec](https://slsa.dev/provenance/v0.2) in their cached archive. The bundle is complete, i.e. not only contains the attestation for the package build, but also those of its dependencies.

## Dirty vs clean Git working copy
When building from a clean Git working copy, the Bhojpur GoRPA will use a reference to the Git remote origin as [material](https://github.com/in-toto/in-toto-golang/blob/26b6a96f8a7537f27b7483e19dd68e022b179ea6/in_toto/model.go#L360) (part of the SLSA [link](https://github.com/slsa-framework/slsa/blob/main/controls/attestations.md)).

## Signing attestations
To support SLSA level 2, the Bhojpur GoRPA can sign the attestations it produces. To this end, you can provide the filepath to a key either as part of the `APPLICATION.yaml` or through the `GORPA_PROVENANCE_KEYPATH` environment variable.

## Inspecting provenance
You can inspect the generated attestation bundle by extracting it from the built and cached archive. For example:
```bash
# run a build
gorpa build --save /tmp/build.tar.gz
# extract bundle
tar xf /tmp/build.tar.gz ./provenance-bundle.json
# inspect the bundle
cat provenance-bundle.json | jq -r .payload | base64 -d | jq
```

## Caveats
- provenance is part of the Bhojpur GoRPA package version, i.e. when you enable provenance that will naturally invalidate previously built packages.
- provenance is not supported for nested workspaces. The presence of `GORPA_NESTED_APPLICATION` will make the build fail.

# Debugging
When a build fails, or to get an idea of how the Bhojpur GoRPA assembles dependencies, run your build with `gorpa build -c local` (local cache only) and inspect your `$GORPA_BUILD_DIR`.

# Bhojpur GoRPA CLI tips

### How can I build a package in the current component/folder?
```bash
gorpa build .:package-name
```

### Is there bash autocompletion?
Yes, run `. <(gorpa bash-completion)` to enable it. If you place this line in `.bashrc` you'll have autocompletion every time.

### How can I find all packages in an Application?
```bash
# list all packages in the application
gorpa collect
# list all package names using Go templates
gorpa collect -t '{{ range $n := . }}{{ $n.Metadata.FullName }}{{"\n"}}{{end}}'
# list all package names using jq
gorpa collect -o json | jq -r '.[].metadata.name'
```

### How can I find out more about a package?
```bash
# print package description on the console
gorpa describe some/components:package
# dump package description as json
gorpa describe some/components:package -o json
```

### How can I inspect a packages depdencies?
```bash
# print the dependency tree on the console
gorpa describe dependencies some/components:package
# print the denendency graph as Graphviz dot
gorpa describe dependencies --dot some/components:package
# serve an interactive version of the dependency graph
gorpa describe dependencies --serve=:8080 some/components:package
```

### How can I print a component constant?
```bash
# print all constants of the component in the current working directory
gorpa describe const .
# print all constants of a component
gorpa describe const some/component/name
# print the value of the `someName` constant of `some/component/name`
gorpa describe const some/component/name -o json | jq -r '.[] | select(.name=="foo").value'
```

### How can I find all components with a particular constant?
```bash
gorpa collect components -l someConstant
```

### How can I export only an Application the way Bhojpur GoRPA sees it, i.e. based on the packages?
```bash
GORPA_EXPERIMENTAL=true gorpa export --strict /some/destination
``
