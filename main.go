package main

// TODO make sure branch names / folder name conversion is clean everywhere
// TODO move more functionality to environment / gitSource

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"github.com/yannh/r10k-go/puppetmodule"
)

type downloadResult struct {
	err     *puppetmodule.DownloadError
	skipped bool
}

type downloadRequest struct {
	m    puppetmodule.PuppetModule
	env  environment
	done chan bool
}

func installPuppetFiles(puppetFiles []*puppetFile, numWorkers int, cache *cache, withDeps bool, limitToModules ...string) int {
	drs := make(chan downloadRequest)

	var wg sync.WaitGroup
	errorCount := make(chan int)

	for w := 1; w <= numWorkers; w++ {
		go downloadModules(drs, cache, withDeps, &wg, errorCount)
	}

	for _, pf := range puppetFiles {
		wg.Add(1)
		go func(pf *puppetFile, drs chan downloadRequest) {
			if err := pf.Process(drs, limitToModules...); err != nil {
				if serr, ok := err.(puppetfileparser.ErrMalformedPuppetfile); ok {
					log.Fatal(serr)
				} else {
					log.Printf("failed parsing %s: %v\n", pf.filename, err)
				}
			}

			pf.Close()
			wg.Done()
		}(pf, drs)
	}

	wg.Wait()
	close(drs)

	nErr := 0
	for w := 1; w <= numWorkers; w++ {
		nErr += <-errorCount
	}
	close(errorCount)
	return nErr
}

func getPuppetFileForEnvironment(env environment, moduledir string, cache *cache) *puppetFile {
	if env.fetch(cache) != nil {
		log.Fatal("Failed fetching environment " + env.branch)
	}

	puppetfile := path.Join(env.source.Basedir, env.branch, "Puppetfile")

	pf := newPuppetFile(puppetfile, environment{env.source, env.branch, moduledir})
	if pf == nil {
		log.Fatalf("no such file or directory %s", puppetfile)
	}
	return pf
}

// If a module is called puppetlabs-stdlib, or puppetlabs/stdlib,
// the target folder should be stdlib
func folderFromModuleName(moduleName string) string {
	splitPath := strings.FieldsFunc(moduleName, func(r rune) bool {
		return r == '/' || r == '-'
	})

	return splitPath[len(splitPath)-1]
}

func downloadModule(m puppetmodule.PuppetModule, to string, cache *cache) downloadResult {
	if m.IsUpToDate(to) {
		return downloadResult{err: nil, skipped: true}
	}

	if err := os.RemoveAll(to); err != nil {
		log.Fatalf("Error removing folder: %s", to)
	}

	if derr := m.Download(to, cache.folder); derr != nil {
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
		if dr.m.GetInstallPath() != "" {
			modulesFolder = path.Join(dr.env.source.Basedir, dr.env.branch, dr.m.GetInstallPath())
		}

		to := path.Join(modulesFolder, folderFromModuleName(dr.m.Name()))

		dres := downloadModule(dr.m, to, cache)
		for i := 1; dres.err != nil && dres.err.Retryable && i < maxTries; i++ {
			log.Printf("failed downloading %s: %v... Retrying\n", dr.m.Name(), dres.err)
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
				log.Println("Downloaded " + dr.m.Name() + " to " + to)
			}
		} else {
			log.Printf("failed downloading %s to %s: %v. Giving up!\n", dr.m.Name(), to, dres.err)
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
	rf, err := os.Open(r10kFile)
	if err != nil {
		log.Fatalf("could not open %s: %v", r10kFile, err)
	}

	r10kConfig, err := parseR10kConfig(rf)
	if err != nil {
		log.Fatalf("Error parsing r10k configuration file %s: %v", r10kFile, err)
	}
	rf.Close()

	cacheDir := ".cache"
	if r10kConfig.Cachedir != "" {
		cacheDir = r10kConfig.Cachedir
	}

	if cliOpts["check"] != false {
		puppetfile := "./Puppetfile"
		if cliOpts["--puppetfile"] != nil {
			puppetfile = cliOpts["--puppetfile"].(string)
		}

		pf := newPuppetFile(puppetfile, environment{})
		if pf == nil {
			log.Fatalf("could not open file: %s", puppetfile)

		}
		if _, _, err := puppetfileparser.Parse(bufio.NewScanner(pf.File)); err != nil {
			log.Fatalf("failed parsing %s: %v", puppetfile, err)
		} else {
			log.Printf("Syntax OK: %s", puppetfile)
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
		puppetFiles := make([]*puppetFile, 0)
		for _, env := range envs {
			puppetFiles = append(puppetFiles, getPuppetFileForEnvironment(env, moduledir, cache))
		}

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
