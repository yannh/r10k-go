package main

// Todo: Remove duplication between modules
// TODO: Move Git wrappers to own module
// TODO: Split downloadmodules in 2 to simplify lock management

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
	Download() DownloadError
	Folder() string
	SetModulesFolder(to string)
	ModulesFolder() string
	Hash() string
	Processed()
}

type CacheableModule interface {
	SetCacheFolder(string)
	IsUpToDate() bool
}

// Can be a PuppetFile or a metadata.json file
type moduleFile interface {
	Filename() string
	Process(modules chan<- PuppetModule, done func()) error
	Close()
}

type DownloadError struct {
	error
	retryable bool
}

type DownloadResult struct {
	err       DownloadError
	skipped   bool
	willRetry bool
	m         PuppetModule
}

func downloadModule(m PuppetModule, results chan DownloadResult, cache *Cache) {
	maxTries := 3
	retryDelay := 5 * time.Second

	m.SetCacheFolder(path.Join(cache.Folder, m.Hash()))
	derr := DownloadError{nil, false}

	cache.LockModule(m.Hash())
	defer cache.UnlockModule(m.Hash())

	if m.IsUpToDate() {
		go func(m PuppetModule) {
			results <- DownloadResult{err: DownloadError{nil, false}, skipped: true, willRetry: false, m: m}
		}(m)
		return
	}

	if err := os.RemoveAll(m.Folder()); err != nil {
		log.Fatalf("Error removing folder: %s", m.Folder())
	}

	derr = m.Download()
	for i := 0; derr.error != nil && i < maxTries-1 && derr.retryable; i++ {
		go func(derr DownloadError, m PuppetModule) {
			results <- DownloadResult{err: derr, skipped: false, willRetry: true, m: m}
		}(derr, m)
		time.Sleep(retryDelay)
		derr = m.Download()
	}

	if derr.error != nil {
		go func(derr DownloadError, m PuppetModule) {
			results <- DownloadResult{err: derr, skipped: false, willRetry: false, m: m}
		}(derr, m)
		return
	}

	// Success
	go func(m PuppetModule) {
		results <- DownloadResult{err: DownloadError{nil, false}, skipped: false, willRetry: false, m: m}
	}(m)
}

// Simplify
func downloadModules(c chan PuppetModule, results chan DownloadResult, cache *Cache) {
	for m := range c {
		downloadModule(m, results, cache)
	}
}

func processModuleFiles(moduleFiles <-chan moduleFile, modules chan PuppetModule, wg *sync.WaitGroup, done chan bool) {
	for mf := range moduleFiles {
		if err := mf.Process(modules, func() { wg.Done() }); err != nil {
			if serr, ok := err.(ErrMalformedPuppetfile); ok {
				log.Fatal(serr)
			} else {
				log.Printf("failed parsing %s: %v\n", mf.Filename(), err)
			}
		}
		mf.Close()
	}

	done <- true
}

func ParseDownloadResults(results <-chan DownloadResult, downloadDeps bool, metadataFiles chan<- moduleFile, wg *sync.WaitGroup, errorsCount chan<- int) {
	downloadErrors := 0

	for res := range results {
		if res.err.error != nil {
			if res.err.retryable == true && res.willRetry == true {
				log.Printf("failed downloading %s: %v... Retrying\n", res.m.Name(), res.err)
			} else {
				log.Printf("failed downloading %s: %v. Giving up!\n", res.m.Name(), res.err)
				downloadErrors++
				res.m.Processed()
			}
			continue
		}

		if res.skipped != true {
			log.Println("Downloaded " + res.m.Name() + " to " + res.m.ModulesFolder())
		}

		// This should not be here
		if downloadDeps {
			mf := NewMetadataFile(res.m.ModulesFolder(), path.Join(res.m.Folder(), "metadata.json"))
			if mf != nil {
				wg.Add(1)
				go func() { metadataFiles <- mf }()
			}
		}

		res.m.Processed()
	}

	errorsCount <- downloadErrors
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

	environmentRootFolder := "."
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
			if err := git.RevParse(path.Join(cacheDir, environmentSource)); err != nil {
				if err := git.Clone(r10kConfig.Sources[environmentSource].Remote, git.Ref{Branch: envName}, sourceCacheFolder); err != nil {
					log.Fatalf("failed downloading environment: %v", err)
				}
			} else {
				git.Fetch(path.Join(cacheDir, environmentSource))
			}

			git.WorktreeAdd(path.Join(cacheDir, environmentSource), git.Ref{Branch: envName}, path.Join(r10kConfig.Sources[environmentSource].Basedir, envName))
			puppetfile := path.Join(r10kConfig.Sources[environmentSource].Basedir, envName, "Puppetfile")
			if pf := NewPuppetFile(path.Join(path.Dir(puppetfile), "modules"), puppetfile); pf != nil {
				puppetFiles = append(puppetFiles, pf)
			}
		}
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		if cliOpts["--puppetfile"] == nil && len(puppetFiles) == 0 {
			puppetfile := path.Join(environmentRootFolder, "Puppetfile")
			if pf := NewPuppetFile(path.Join(path.Dir(puppetfile), "modules"), puppetfile); pf != nil {
				puppetFiles = append(puppetFiles, pf)
			}
		} else if cliOpts["--puppetfile"] != nil {
			puppetfile := cliOpts["--puppetfile"].(string)
			if pf := NewPuppetFile(path.Join(path.Dir(puppetfile), "modules"), puppetfile); pf != nil {
				puppetFiles = append(puppetFiles, pf)
			}
		}

		results := make(chan DownloadResult)
		modules := make(chan PuppetModule)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(modules, results, cache)
		}

		var wg sync.WaitGroup
		moduleFiles := make(chan moduleFile)

		done := make(chan bool)
		errorCount := make(chan int)

		go processModuleFiles(moduleFiles, modules, &wg, done)
		go ParseDownloadResults(results, !cliOpts["--no-deps"].(bool), moduleFiles, &wg, errorCount)

		for _, pf := range puppetFiles {
			wg.Add(1)
			moduleFiles <- pf
		}

		wg.Wait()
		close(modules)
		close(moduleFiles)
		close(results)

		<-done
		nErr := <-errorCount
		close(errorCount)

		os.Exit(nErr)
	}
}
