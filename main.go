package main

import (
	"github.com/yannh/r10k-go/git"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ForgeModule, GitModule, GithubTarballModule, ....
type PuppetModule interface {
	IsUpToDate(folder string) bool
	Name() string
	Download(to string, cache *Cache) *DownloadError
	Hash() string
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

type source struct {
	Basedir string
	Prefix  string
	Remote  string
}

type environment struct {
	source
	branch        string
	modulesFolder string
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
	maxTries := 3
	retryDelay := 5 * time.Second
	errors := 0

	for dr := range drs {
		cache.LockModule(dr.m.Hash())

		modulesFolder := path.Join(dr.env.Basedir, dr.env.branch, "modules")
		if dr.env.modulesFolder != "" {
			modulesFolder = path.Join(dr.env.Basedir, dr.env.branch, dr.env.modulesFolder)
		}
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
		cache.UnlockModule(dr.m.Hash())
	}

	errorsCount <- errors
}

func main() {
	var err error
	var numWorkers int
	var cache *Cache

	cliOpts := cli()

	if cliOpts["--workers"] == nil {
		numWorkers = 4
	} else {
		numWorkers, err = strconv.Atoi(cliOpts["--workers"].(string))
		if err != nil {
			log.Fatalf("Parameter --workers should be an integer")
		}
	}

	cacheDir := ".cache"
	var puppetFiles []*PuppetFile

	if cache, err = NewCache(cacheDir); err != nil {
		log.Fatal(err)
	}

	if cliOpts["deploy"] == true {
		r10kFile := "r10k.yml"
		r10kConfig, err := NewR10kConfig(r10kFile)
		if err != nil {
			log.Fatalf("Error parsing r10k configuration file %s: %v", r10kFile, err)
		}

		if r10kConfig.Cachedir != "" {
			cacheDir = r10kConfig.Cachedir
		}

		var s source

		// Find in which source the environment is
		// TODO: render deterministic
		for _, envName := range cliOpts["<env>"].([]string) {
			sourceName := ""

			for name, source := range r10kConfig.Sources {
				if git.RepoHasBranch(source.Remote, envName) {
					sourceName = name
					s = source
					break
				}
			}

			sourceCacheFolder := path.Join(cacheDir, sourceName)
			// Clone if environment doesnt exist, fetch otherwise
			if err := git.RevParse(sourceCacheFolder); err != nil {
				if err := git.Clone(r10kConfig.Sources[sourceName].Remote, git.Ref{Branch: envName}, sourceCacheFolder); err != nil {
					log.Fatalf("failed downloading environment: %v", err)
				}
			} else {
				git.Fetch(sourceCacheFolder)
			}

			git.WorktreeAdd(sourceCacheFolder, git.Ref{Branch: envName}, path.Join(r10kConfig.Sources[sourceName].Basedir, envName))
			puppetfile := path.Join(r10kConfig.Sources[sourceName].Basedir, envName, "Puppetfile")

			moduledir := "modules"
			if cliOpts["--moduledir"] != nil {
				moduledir = cliOpts["--moduledir"].(string)
			}
			pf := NewPuppetFile(puppetfile, environment{s, envName, moduledir})
			if pf == nil {
				log.Fatalf("no such file or directory %s", puppetfile)
			}
			puppetFiles = append(puppetFiles, pf)
		}
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
		pf := NewPuppetFile(puppetfile, environment{source{Basedir: path.Dir(puppetfile), Prefix: "", Remote: ""}, "", moduledir})
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}

		puppetFiles = append(puppetFiles, pf)
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		drs := make(chan downloadRequest)

		var wg sync.WaitGroup
		errorCount := make(chan int)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(drs, cache, !cliOpts["--no-deps"].(bool), &wg, errorCount)
		}

		for _, pf := range puppetFiles {
			wg.Add(1)
			go func(pf *PuppetFile, drs chan downloadRequest) {
				if err := pf.Process(drs); err != nil {
					if serr, ok := err.(ErrMalformedPuppetfile); ok {
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

		os.Exit(nErr)
	}
}
