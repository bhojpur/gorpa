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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	gorpa "github.com/bhojpur/gorpa/pkg/engine"
	"github.com/bhojpur/gorpa/pkg/version"
	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [targetPackage]",
	Short: "Builds a package",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, pkg, _, _ := getTarget(args, false)
		if pkg == nil {
			log.Fatal("build needs a package")
		}
		opts, localCache := getBuildOpts(cmd)

		var (
			watch, _ = cmd.Flags().GetBool("watch")
			save, _  = cmd.Flags().GetString("save")
			serve, _ = cmd.Flags().GetString("serve")
		)
		if watch {
			err := gorpa.Build(pkg, opts...)
			if err != nil {
				log.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			if save != "" {
				saveBuildResult(ctx, save, localCache, pkg)
			}
			if serve != "" {
				go serveBuildResult(ctx, serve, localCache, pkg)
			}

			evt, errs := gorpa.WatchSources(context.Background(), append(pkg.GetTransitiveDependencies(), pkg))
			for {
				select {
				case <-evt:
					_, pkg, _, _ := getTarget(args, false)
					err := gorpa.Build(pkg, opts...)
					if err == nil {
						cancel()
						ctx, cancel = context.WithCancel(context.Background())
						if save != "" {
							saveBuildResult(ctx, save, localCache, pkg)
						}
						if serve != "" {
							go serveBuildResult(ctx, serve, localCache, pkg)
						}
					} else {
						log.Error(err)
					}
				case err = <-errs:
					log.Fatal(err)
				}
			}
		}

		err := gorpa.Build(pkg, opts...)
		if err != nil {
			log.Fatal(err)
		}
		if save != "" {
			saveBuildResult(context.Background(), save, localCache, pkg)
		}
		if serve != "" {
			serveBuildResult(context.Background(), serve, localCache, pkg)
		}
	},
}

func serveBuildResult(ctx context.Context, addr string, localCache *gorpa.FilesystemCache, pkg *gorpa.Package) {
	br, exists := localCache.Location(pkg)
	if !exists {
		log.Fatal("build result is not in local cache despite just being built. Something's wrong with the cache.")
	}

	tmp, err := ioutil.TempDir("", "gorpa_serve")
	if err != nil {
		log.WithError(err).Fatal("cannot serve build result")
	}

	cmd := exec.Command("tar", "xzf", br)
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).WithField("output", string(out)).Fatal("cannot serve build result")
	}

	if ctx.Err() != nil {
		return
	}

	fmt.Printf("\n????  serving build result on %s\n", color.Cyan.Render(addr))
	server := &http.Server{Addr: addr, Handler: http.FileServer(http.Dir(tmp))}
	go func() {
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	err = server.Close()
	if err != nil {
		log.WithError(err).Error("cannot close server")
	}
}

func saveBuildResult(ctx context.Context, loc string, localCache *gorpa.FilesystemCache, pkg *gorpa.Package) {
	br, exists := localCache.Location(pkg)
	if !exists {
		log.Fatal("build result is not in local cache despite just being built. Something's wrong with the cache.")
	}

	fout, err := os.OpenFile(loc, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Fatal("cannot open result file for writing")
	}
	fin, err := os.OpenFile(br, os.O_RDONLY, 0644)
	if err != nil {
		fout.Close()
		log.WithError(err).Fatal("cannot copy build result")
	}

	_, err = io.Copy(fout, fin)
	fout.Close()
	fin.Close()
	if err != nil {
		log.WithError(err).Fatal("cannot copy build result")
	}

	fmt.Printf("\n????  saving build result to %s\n", color.Cyan.Render(loc))
}

func init() {
	rootCmd.AddCommand(buildCmd)

	addBuildFlags(buildCmd)
	buildCmd.Flags().String("serve", "", "After a successful build this starts a webserver on the given address serving the build result (e.g. --serve localhost:8080)")
	buildCmd.Flags().String("save", "", "After a successful build this saves the build result as tar.gz file in the local filesystem (e.g. --save build-result.tar.gz)")
	buildCmd.Flags().Bool("watch", false, "Watch source files and re-build on change")
}

func addBuildFlags(cmd *cobra.Command) {
	cacheDefault := os.Getenv("GORPA_DEFAULT_CACHE_LEVEL")
	if cacheDefault == "" {
		cacheDefault = "remote"
	}

	cmd.Flags().StringP("cache", "c", cacheDefault, "Configures the caching behaviour: none=no caching, local=local caching only, remote-pull=download from remote but never upload, remote-push=push to remote cache only but don't download, remote=use all configured caches")
	cmd.Flags().StringSlice("add-remote-cache", []string{}, "Configures additional (pull-only) remote caches")
	cmd.Flags().Bool("dry-run", false, "Don't actually build but stop after showing what would need to be built")
	cmd.Flags().String("dump-plan", "", "Writes the build plan as JSON to a file. Use \"-\" to write the build plan to stderr.")
	cmd.Flags().Bool("gorpa", false, "Produce GoRPA CI compatible output")
	cmd.Flags().Bool("dont-test", false, "Disable all package-level tests (defaults to false)")
	cmd.Flags().Bool("dont-retag", false, "Disable Docker image re-tagging (defaults to false)")
	cmd.Flags().UintP("max-concurrent-tasks", "j", uint(runtime.NumCPU()), "Limit the number of max concurrent build tasks - set to 0 to disable the limit")
	cmd.Flags().String("coverage-output-path", "", "Output path where test coverage file will be copied after running tests")
	cmd.Flags().StringToString("docker-build-options", nil, "Options passed to all 'docker build' commands")

}

func getBuildOpts(cmd *cobra.Command) ([]gorpa.BuildOption, *gorpa.FilesystemCache) {
	cm, _ := cmd.Flags().GetString("cache")
	log.WithField("cacheMode", cm).Debug("configuring caches")
	cacheLevel := gorpa.CacheLevel(cm)

	remoteCache := getRemoteCache()
	switch cacheLevel {
	case gorpa.CacheNone, gorpa.CacheLocal:
		remoteCache = gorpa.NoRemoteCache{}
	case gorpa.CacheRemotePull:
		remoteCache = &pullOnlyRemoteCache{C: remoteCache}
	case gorpa.CacheRemotePush:
		remoteCache = &pushOnlyRemoteCache{C: remoteCache}
	case gorpa.CacheRemote:
	default:
		log.Fatalf("invalid cache level: %s", cacheLevel)
	}

	var (
		localCacheLoc string
		err           error
	)
	if cacheLevel == gorpa.CacheNone {
		localCacheLoc, err = ioutil.TempDir("", "gorpa")
		if err != nil {
			log.Fatal(err)
		}
	} else {
		localCacheLoc = os.Getenv(gorpa.EnvvarCacheDir)
		if localCacheLoc == "" {
			localCacheLoc = filepath.Join(os.TempDir(), "cache")
		}
	}
	log.WithField("location", localCacheLoc).Debug("set up local cache")
	localCache, err := gorpa.NewFilesystemCache(localCacheLoc)
	if err != nil {
		log.Fatal(err)
	}

	var arcs []gorpa.RemoteCache
	additionalRemoteCaches, _ := cmd.Flags().GetStringSlice("add-remote-cache")
	for _, arc := range additionalRemoteCaches {
		arcs = append(arcs, &pullOnlyRemoteCache{
			C: gorpa.GSUtilRemoteCache{
				BucketName: arc,
			},
		})
	}

	dryrun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		log.Fatal(err)
	}

	log.Debugf("Bhojpur GoRPA version %s", version.Version)

	var planOutlet io.Writer
	if plan, _ := cmd.Flags().GetString("dump-plan"); plan != "" {
		if plan == "-" {
			planOutlet = os.Stderr
		} else {
			f, err := os.OpenFile(plan, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			planOutlet = f
		}
	}

	gorpalog, err := cmd.Flags().GetBool("gorpa")
	if err != nil {
		log.Fatal(err)
	}
	var reporter gorpa.Reporter
	if gorpalog {
		reporter = gorpa.NewGorpaReporter()
	} else {
		reporter = gorpa.NewConsoleReporter()
	}

	dontTest, err := cmd.Flags().GetBool("dont-test")
	if err != nil {
		log.Fatal(err)
	}

	dontRetag, err := cmd.Flags().GetBool("dont-retag")
	if err != nil {
		log.Fatal(err)
	}

	maxConcurrentTasks, err := cmd.Flags().GetUint("max-concurrent-tasks")
	if err != nil {
		log.Fatal(err)
	}

	coverageOutputPath, _ := cmd.Flags().GetString("coverage-output-path")
	if coverageOutputPath != "" {
		_ = os.MkdirAll(coverageOutputPath, 0644)
	}

	var dockerBuildOptions gorpa.DockerBuildOptions
	dockerBuildOptions, err = cmd.Flags().GetStringToString("docker-build-options")
	if err != nil {
		log.Fatal(err)
	}

	return []gorpa.BuildOption{
		gorpa.WithLocalCache(localCache),
		gorpa.WithRemoteCache(remoteCache),
		gorpa.WithAdditionalRemoteCaches(arcs),
		gorpa.WithDryRun(dryrun),
		gorpa.WithBuildPlan(planOutlet),
		gorpa.WithReporter(reporter),
		gorpa.WithDontTest(dontTest),
		gorpa.WithMaxConcurrentTasks(int64(maxConcurrentTasks)),
		gorpa.WithCoverageOutputPath(coverageOutputPath),
		gorpa.WithDontRetag(dontRetag),
		gorpa.WithDockerBuildOptions(&dockerBuildOptions),
	}, localCache
}

type pushOnlyRemoteCache struct {
	C gorpa.RemoteCache
}

func (c *pushOnlyRemoteCache) Download(dst gorpa.Cache, pkgs []*gorpa.Package) error {
	return nil
}

func (c *pushOnlyRemoteCache) Upload(src gorpa.Cache, pkgs []*gorpa.Package) error {
	return c.C.Upload(src, pkgs)
}

type pullOnlyRemoteCache struct {
	C gorpa.RemoteCache
}

func (c *pullOnlyRemoteCache) Download(dst gorpa.Cache, pkgs []*gorpa.Package) error {
	return c.C.Download(dst, pkgs)
}

func (c *pullOnlyRemoteCache) Upload(src gorpa.Cache, pkgs []*gorpa.Package) error {
	return nil
}
