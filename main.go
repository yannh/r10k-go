package main

// TODO make sure branch names / folder name conversion is clean everywhere
// TODO move more functionality to environment / source

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

// PuppetModule is implemented by ForgeModule, GitModule, GithubTarballModule, ....
type PuppetModule interface {
	IsUpToDate(folder string) bool
	Name() string
	Download(to string, cache *Cache) *DownloadError
	InstallPath() string
}

type DownloadError struct {
	error
	retryable bool
}

type DownloadResult struct {
	err     *DownloadError
	skipped bool
}

type downloadRequest struct {
	m    PuppetModule
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

func downloadModule(m PuppetModule, to string, cache *Cache) DownloadResult {
	if m.IsUpToDate(to) {
		return DownloadResult{err: nil, skipped: true}
	}

	if err := os.RemoveAll(to); err != nil {
		log.Fatalf("Error removing folder: %s", to)
	}

	if derr := m.Download(to, cache); derr != nil {
		return DownloadResult{err: derr, skipped: false}
	}

	return DownloadResult{err: nil, skipped: false}
}

func downloadModules(drs chan downloadRequest, cache *Cache, downloadDeps bool, wg *sync.WaitGroup, errorsCount chan<- int) {
	maxTries := 1
	retryDelay := 5 * time.Second
	errors := 0

	for dr := range drs {
		cache.LockModule(dr.m)

		modulesFolder := path.Join(dr.env.Basedir, dr.env.branch, dr.env.modulesFolder)
		if dr.m.InstallPath() != "" {
			modulesFolder = path.Join(dr.env.Basedir, dr.env.branch, dr.m.InstallPath())
		}

		to := path.Join(modulesFolder, folderFromModuleName(dr.m.Name()))

		dres := downloadModule(dr.m, to, cache)
		for i := 1; dres.err != nil && dres.err.retryable && i < maxTries; i++ {
			log.Printf("failed downloading %s: %v... Retrying\n", dr.m.Name(), dres.err)
			time.Sleep(retryDelay)
			dres = downloadModule(dr.m, to, cache)
		}

		if dres.err == nil {
			if downloadDeps && !dres.skipped {
				metadataFilename := path.Join(to, "metadata.json")
				if mf := NewMetadataFile(metadataFilename, dr.env); mf != nil {
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
		cache.UnlockModule(dr.m)
	}

	errorsCount <- errors
}

func main() {
	var err error
	var numWorkers int
	var cache *Cache
	var puppetFiles []*PuppetFile

	cliOpts := cli()

	numWorkers = 4
	if cliOpts["--workers"] != nil {
		if numWorkers, err = strconv.Atoi(cliOpts["--workers"].(string)); err != nil {
			log.Fatalf("Parameter --workers should be an integer")
		}
	}

	r10kFile := "r10k.yml"
	r10kConfig, err := NewR10kConfig(r10kFile)
	if err != nil {
		log.Fatalf("Error parsing r10k configuration file %s: %v", r10kFile, err)
	}

	cacheDir := ".cache"
	if r10kConfig.Cachedir != "" {
		cacheDir = r10kConfig.Cachedir
	}

	if cache, err = NewCache(cacheDir); err != nil {
		log.Fatal(err)
	}

	if cliOpts["check"] != false {
		puppetfile := "./Puppetfile"
		pf := NewPuppetFile(puppetfile, environment{})
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
		pf := NewPuppetFile(puppetfile, environment{source{Basedir: path.Dir(puppetfile), prefix: "", Remote: ""}, "", moduledir})
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
		puppetFiles = getPuppetfilesForEnvironments(cliOpts["<env>"].([]string), r10kConfig.Sources, cache, moduledir)
		os.Exit(installPuppetFiles(puppetFiles, 4, cache, !cliOpts["--no-deps"].(bool)))
	}

	if cliOpts["deploy"] == true && cliOpts["module"] == true {
		drs := make(chan downloadRequest)
		var wg sync.WaitGroup
		errorCount := make(chan int)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(drs, cache, false, &wg, errorCount)
		}

		if cache, err = NewCache(r10kConfig.Cachedir); err != nil {
			log.Fatal(err)
		}

		for sourceName, s := range r10kConfig.Sources { // TODO verify sourceName is usable as a directory name
			sourceCacheFolder := path.Join(cache.folder, sourceName)
			git.Fetch(sourceCacheFolder)

			for _, env := range s.deployedEnvironments() {
				git.Fetch(env.Basedir)
				puppetFilePath := path.Join(s.Basedir, env.branch, "Puppetfile")
				pf := NewPuppetFile(puppetFilePath, *env)
				if pf != nil {
					limit := cliOpts["<module>"].([]string)
					pf.Process(drs, limit...)
				}
			}
		}
		os.Exit(1)
	}
}
