package main

// TODO Fix installpath
// TODO ParseDownloadREsults is a mess, nedds simplifying

import (
	"github.com/yannh/r10k-go/git"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

// ForgeModule, GitModule, GithubTarballModule, ....
type PuppetModule interface {
	CacheableModule
	Name() string
	Download(string, *Cache) DownloadError
	Folder() string
	SetModulesFolder(to string)
	ModulesFolder() string
	Hash() string
	Processed()
}

type CacheableModule interface {
	IsUpToDate() bool
}

// Can be a PuppetFile or a metadata.json file
type moduleFile interface {
	Filename() string
	Process(modules chan<- PuppetModule) error
	Close()
}

type DownloadError struct {
	error
	retryable bool
}

type DownloadResult struct {
	err       DownloadError
	skipped   bool
	retryable bool
}

func downloadModule(m PuppetModule, cache *Cache) DownloadResult {
	if m.IsUpToDate() {
		return DownloadResult{err: DownloadError{nil, false}, skipped: true, retryable: false}
	}

	if err := os.RemoveAll(m.Folder()); err != nil {
		log.Fatalf("Error removing folder: %s", m.Folder())
	}

	if derr := m.Download(m.Folder(), cache); derr.error != nil {
		return DownloadResult{err: derr, skipped: false, retryable: true}
	}

	return DownloadResult{err: DownloadError{nil, false}, skipped: false, retryable: false}
}

func downloadModules(modules chan PuppetModule, cache *Cache, downloadDeps bool, wg *sync.WaitGroup, errorsCount chan<- int) {
	maxTries := 3
	retryDelay := 5 * time.Second
	errors := 0

	for m := range modules {
		cache.LockModule(m.Hash())

		dres := downloadModule(m, cache)
		for i := 1; dres.err.error != nil && dres.err.retryable && i < maxTries; i++ {
			log.Printf("failed downloading %s: %v... Retrying\n", m.Name(), dres.err)
			time.Sleep(retryDelay)
			dres = downloadModule(m, cache)
		}

		if dres.err.error == nil {
			if downloadDeps {
				if mf := NewMetadataFile(path.Join(m.Folder(), "metadata.json"), m.ModulesFolder()); mf != nil {
					wg.Add(1)
					go func() {
						processModuleFile(mf, modules)
						mf.Close()
						wg.Done()
					}()
				}
			}

			if !dres.skipped {
				log.Println("Downloaded " + m.Name() + " to " + m.ModulesFolder())
			}
		} else {
			log.Printf("failed downloading %s: %v. Giving up!\n", m.Name(), dres.err)
			errors++
		}

		m.Processed()
		cache.UnlockModule(m.Hash())
	}

	errorsCount <- errors
}

func processModuleFile(mf moduleFile, modules chan PuppetModule) {
	if err := mf.Process(modules); err != nil {
		if serr, ok := err.(ErrMalformedPuppetfile); ok {
			log.Fatal(serr)
		} else {
			log.Printf("failed parsing %s: %v\n", mf.Filename(), err)
		}
	}
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

		// Find in which source the environment is
		// TODO: render deterministic
		for _, envName := range cliOpts["<env>"].([]string) {
			environmentSource := ""

			for sourceName, sourceOpts := range r10kConfig.Sources {
				if git.RepoHasBranch(sourceOpts.Remote, envName) {
					environmentSource = sourceName
					break
				}
			}

			sourceCacheFolder := path.Join(cacheDir, environmentSource)

			// Clone if environment doesnt exist, fetch otherwise
			if err := git.RevParse(sourceCacheFolder); err != nil {
				if err := git.Clone(r10kConfig.Sources[environmentSource].Remote, git.Ref{Branch: envName}, sourceCacheFolder); err != nil {
					log.Fatalf("failed downloading environment: %v", err)
				}
			} else {
				git.Fetch(sourceCacheFolder)
			}

			git.WorktreeAdd(sourceCacheFolder, git.Ref{Branch: envName}, path.Join(r10kConfig.Sources[environmentSource].Basedir, envName))
			puppetfile := path.Join(r10kConfig.Sources[environmentSource].Basedir, envName, "Puppetfile")

			pf := NewPuppetFile(puppetfile, path.Join(path.Dir(puppetfile), "modules"))
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

		pf := NewPuppetFile(puppetfile, path.Join(path.Dir(puppetfile), "modules"))
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}

		puppetFiles = append(puppetFiles, pf)
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		modules := make(chan PuppetModule)

		var wg sync.WaitGroup
		errorCount := make(chan int)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(modules, cache, !cliOpts["--no-deps"].(bool), &wg, errorCount)
		}

		for _, pf := range puppetFiles {
			wg.Add(1)
			go func(pf moduleFile, modules chan PuppetModule) {
				processModuleFile(pf, modules)
				pf.Close()
				wg.Done()
			}(pf, modules)
		}

		wg.Wait()
		close(modules)

		nErr := 0
		for w := 1; w <= numWorkers; w++ {
			nErr += <-errorCount
		}

		close(errorCount)

		os.Exit(nErr)
	}
}
