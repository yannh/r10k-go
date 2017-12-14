package main

// TODO make sure branch names / folder name conversion is clean everywhere
// TODO move more functionality to environment / gitSource

import (
	"bufio"
	"fmt"
	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// puppetModule is implemented by forgeModule, gitModule, githubTarballModule, ....
type puppetModule interface {
	isUpToDate(folder string) bool
	getName() string
	download(to string, cache *cache) *downloadError
	getInstallPath() string
}

type downloadError struct {
	error
	retryable bool
}

type downloadResult struct {
	err     *downloadError
	skipped bool
}

type downloadRequest struct {
	m    puppetModule
	env  environment
	done chan bool
}

// If a module is called puppetlabs-stdlib, or puppetlabs/stdlib,
// the target folder should be stdlib
func folderFromModuleName(moduleName string) string {
	splitPath := strings.FieldsFunc(moduleName, func(r rune) bool {
		return r == '/' || r == '-'
	})

	return splitPath[len(splitPath)-1]
}

func downloadModule(m puppetModule, to string, cache *cache) downloadResult {
	if m.isUpToDate(to) {
		return downloadResult{err: nil, skipped: true}
	}

	if err := os.RemoveAll(to); err != nil {
		log.Fatalf("Error removing folder: %s", to)
	}

	if derr := m.download(to, cache); derr != nil {
		return downloadResult{err: derr, skipped: false}
	}

	return downloadResult{err: nil, skipped: false}
}

func downloadModules(drs chan downloadRequest, cache *cache, downloadDeps bool, wg *sync.WaitGroup, errorsCount chan<- int) {
	maxTries := 1
	retryDelay := 5 * time.Second
	errors := 0

	for dr := range drs {
		cache.lockModule(dr.m)

		modulesFolder := path.Join(dr.env.source.Basedir, dr.env.branch, dr.env.modulesFolder)
		if dr.m.getInstallPath() != "" {
			modulesFolder = path.Join(dr.env.source.Basedir, dr.env.branch, dr.m.getInstallPath())
		}

		to := path.Join(modulesFolder, folderFromModuleName(dr.m.getName()))

		dres := downloadModule(dr.m, to, cache)
		for i := 1; dres.err != nil && dres.err.retryable && i < maxTries; i++ {
			log.Printf("failed downloading %s: %v... Retrying\n", dr.m.getName(), dres.err)
			time.Sleep(retryDelay)
			dres = downloadModule(dr.m, to, cache)
		}

		if dres.err == nil {
			if downloadDeps && !dres.skipped {
				metadataFilename := path.Join(to, "metadata.json")
				if mf := newMetadataFile(metadataFilename, dr.env); mf != nil {
					wg.Add(1)
					go func() {
						if err := mf.Process(drs); err != nil {
							log.Printf("failed parsing %s: %v\n", metadataFilename, err)
						}

						mf.Close()
						wg.Done()
					}()
				}
			}

			if !dres.skipped {
				log.Println("Downloaded " + dr.m.getName() + " to " + to)
			}
		} else {
			log.Printf("failed downloading %s to %s: %v. Giving up!\n", dr.m.getName(), to, dres.err)
			errors++
		}

		dr.done <- true
		cache.unlockModule(dr.m)
	}

	errorsCount <- errors
}

func main() {
	var err error
	var numWorkers int
	var cache *cache
	var puppetFiles []*puppetFile

	cliOpts := cli()

	numWorkers = 4
	if cliOpts["--workers"] != nil {
		if numWorkers, err = strconv.Atoi(cliOpts["--workers"].(string)); err != nil {
			log.Fatalf("Parameter --workers should be an integer")
		}
	}

	r10kFile := "r10k.yml"
	r10kConfig, err := newR10kConfig(r10kFile)
	if err != nil {
		log.Fatalf("Error parsing r10k configuration file %s: %v", r10kFile, err)
	}

	cacheDir := ".cache"
	if r10kConfig.Cachedir != "" {
		cacheDir = r10kConfig.Cachedir
	}

	if cliOpts["check"] != false {
		puppetfile := "./Puppetfile"
		pf := newPuppetFile(puppetfile, environment{})
		if _, _, err := puppetfileparser.Parse(bufio.NewScanner(pf.File)); err != nil {
			log.Fatalf("failed parsing %s: %v", puppetfile, err)
		} else {
			log.Printf("file parsed correctly: %s", puppetfile)
			os.Exit(0)
		}
	}

	if cliOpts["version"] != false {
		fmt.Println("0.0.1")
		os.Exit(0)
	}

	if cache, err = newCache(cacheDir); err != nil {
		log.Fatal(err)
	}

	if cliOpts["install"] == true {
		puppetfile := ""
		if cliOpts["--puppetfile"] == nil {
			wd, _ := os.Getwd()
			puppetfile = path.Join(wd, "Puppetfile")
		} else {
			puppetfile = cliOpts["--puppetfile"].(string)
		}

		moduledir := "modules"
		if cliOpts["--moduledir"] != nil {
			moduledir = cliOpts["--moduledir"].(string)
		}
		pf := newPuppetFile(puppetfile, environment{gitSource{Basedir: path.Dir(puppetfile), prefix: "", Remote: ""}, "", moduledir})
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}

		puppetFiles = append(puppetFiles, pf)
		os.Exit(installPuppetFiles(puppetFiles, 4, cache, !cliOpts["--no-deps"].(bool)))
	}

	if cliOpts["deploy"] == true && cliOpts["environment"] == true {
		moduledir := "modules"
		if cliOpts["--moduledir"] != nil {
			moduledir = cliOpts["--moduledir"].(string)
		}

		sources := make([]gitSource, 0)
		for sName, s := range r10kConfig.Sources {
			s.Name = sName
			sources = append(sources, s)
		}

		envs := getEnvironments(cliOpts["<env>"].([]string), sources)
		puppetFiles := getPuppetFilesForEnvironments(envs, moduledir, cache)

		os.Exit(installPuppetFiles(puppetFiles, 4, cache, !cliOpts["--no-deps"].(bool)))
	}

	if cliOpts["deploy"] == true && cliOpts["module"] == true {
		for sourceName, s := range r10kConfig.Sources { // TODO verify sourceName is usable as a directory name
			git.Fetch(path.Join(cache.folder, sourceName))

			for _, env := range s.deployedEnvironments() {
				if pf := newPuppetFile(path.Join(s.Basedir, env.branch, "Puppetfile"), env); pf != nil {
					puppetFiles = append(puppetFiles, pf)
				}
			}
		}

		limit := cliOpts["<module>"].([]string)
		os.Exit(installPuppetFiles(puppetFiles, numWorkers, cache, false, limit...))
	}

	os.Exit(1)
}
