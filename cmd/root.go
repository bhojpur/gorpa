package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"strings"

	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"

	"github.com/bhojpur/gorpa/pkg/gorpa"
)

const (
	// EnvvarApplicationRoot names the environment variable we check for the application root path
	EnvvarApplicationRoot = "GORPA_APPLICATION_ROOT"

	// EnvvarRemoteCacheBucket configures a bucket name. This enables the use of RemoteStorage
	EnvvarRemoteCacheBucket = "GORPA_REMOTE_CACHE_BUCKET"

	// EnvvarRemoteCacheStorage configures a Remote Storage Provider. Default is GCP
	EnvvarRemoteCacheStorage = "GORPA_REMOTE_CACHE_STORAGE"
)

const (
	bashCompletionFunc = `__gorpa_parse_get()
{
    local gorpa_output out
    if gorpa_output=$(gorpa collect 2>/dev/null); then
        out=($(echo "${gorpa_output}" | awk '{print $1}'))
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}
__gorpa_get_resource()
{
    __gorpa_parse_get
    if [[ $? -eq 0 ]]; then
        return 0
    fi
}
__gorpa_custom_func() {
    case ${last_command} in
        gorpa_build | gorpa_describe)
            __gorpa_get_resource
            return
            ;;
        *)
            ;;
    esac
}
`
)

var (
	// version is set during the build using ldflags
	version string = "unknown"

	application string
	buildArgs []string
	verbose   bool
	variant   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gorpa",
	Short: "A caching meta-build system",
	Long: color.Render(`<light_yellow>The GoRPA is a heavily caching build system</> for Go, Yarn, and Docker projects. It knows three core concepts:
  Application: the application is the root of all operations. All component names are relative to this path. No relevant
             file must be placed outside the application. The application root is marked with an APPLICATION file.
  Component: a component is single piece of standalone software. Every folder in the application which contains a BUILD file
             is a component. Components are identifed by their path relative to the application root.
  Package:   packages are the buildable unit in GoRPA. Every component can define multiple packages in its build file.
             Packages are identified by their name prefixed with the component name, e.g. some-component:pkg
<white>Configuration</>
The GoRPA is configured exclusively through the APPLICATION/BUILD files and environment variables. The following environment
variables have an effect on GoRPA:
       <light_blue>GORPA_APPLICATION_ROOT</>  Contains the path where to look for a APPLICATION file. Can also be set using --application.
     <light_blue>GORPA_NESTED_APPLICATION</>  Enables (experimental) support for nested applications.
  <light_blue>GORPA_REMOTE_CACHE_BUCKET</>  Enables remote caching using GCP buckets. Set this variable to the bucket name used for caching.
                              When this variable is set, GoRPA expects "gsutil" in the path configured and authenticated so
                              that it can work with the bucket.
            <light_blue>GORPA_CACHE_DIR</>  Location of the local build cache. The directory does not have to exist yet.
            <light_blue>GORPA_BUILD_DIR</>  Working location of GoRPA (i.e. where the actual builds happen). This location will see heavy I/O
                              which makes it advisable to place this on a fast SSD or in RAM.
           <light_blue>GORPA_YARN_MUTEX</>  Configures the mutex flag GoRPA will pass to yarn. Defaults to "network".
                              See https://yarnpkg.com/lang/en/docs/cli/#toc-concurrency-and-mutex for possible values.
  <light_blue>GORPA_DEFAULT_CACHE_LEVEL</>  Sets the default cache level for builds. Defaults to "remote".
         <light_blue>GORPA_EXPERIMENTAL</>  Enables experimental GoRPA features and commands.
`),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			log.SetLevel(log.DebugLevel)
		}
	},
	BashCompletionFunction: bashCompletionFunc,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	tp := os.Getenv("GORPA_TRACE")
	if tp != "" {
		f, err := os.OpenFile(tp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			log.WithError(err).Fatal("cannot start trace but GORPA_TRACE is set")
			return
		}
		defer f.Close()
		err = trace.Start(f)
		if err != nil {
			log.WithError(err).Fatal("cannot start trace but GORPA_TRACE is set")
			return
		}
		defer trace.Stop()

		defer trace.StartRegion(context.Background(), "main").End()
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	applicationRoot := os.Getenv(EnvvarApplicationRoot)
	if applicationRoot == "" {
		applicationRoot = "."
	}

	rootCmd.PersistentFlags().StringVarP(&application, "application", "w", applicationRoot, "Application root")
	rootCmd.PersistentFlags().StringArrayVarP(&buildArgs, "build-arg", "D", []string{}, "pass arguments to BUILD files")
	rootCmd.PersistentFlags().StringVar(&variant, "variant", "", "selects a package variant")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enables verbose logging")
	rootCmd.PersistentFlags().Bool("dut", false, "used for testing only - doesn't actually do anything")
}

func getApplication() (gorpa.Application, error) {
	args, err := getBuildArgs()
	if err != nil {
		return gorpa.Application{}, err
	}

	if os.Getenv("GORPA_NESTED_APPLICATION") != "" {
		return gorpa.FindNestedApplications(application, args, variant)
	}

	return gorpa.FindApplication(application, args, variant)
}

func getBuildArgs() (gorpa.Arguments, error) {
	if len(buildArgs) == 0 {
		return nil, nil
	}

	res := make(gorpa.Arguments)
	for _, arg := range buildArgs {
		segs := strings.Split(arg, "=")
		if len(segs) < 2 {
			return nil, xerrors.Errorf("invalid build argument (format is key=value): %s", arg)
		}
		res[segs[0]] = strings.Join(segs[1:], "=")
	}
	return res, nil
}

func getRemoteCache() gorpa.RemoteCache {
	remoteCacheBucket := os.Getenv(EnvvarRemoteCacheBucket)
	remoteStorage := os.Getenv(EnvvarRemoteCacheStorage)
	if remoteCacheBucket != "" {
		switch remoteStorage {
		case "GCP":
			return gorpa.GSUtilRemoteCache{
				BucketName: remoteCacheBucket,
			}
		case "MINIO":
			return gorpa.MinioRemoteCache{
				BucketName: remoteCacheBucket,
			}
		default:
			return gorpa.GSUtilRemoteCache{
				BucketName: remoteCacheBucket,
			}
		}

	}

	return gorpa.NoRemoteCache{}
}

func addExperimentalCommand(parent, child *cobra.Command) {
	if os.Getenv("GORPA_EXPERIMENTAL") != "true" {
		return
	}

	parent.AddCommand(child)
}
