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

func downloadModule(m PuppetModule, cache *Cache) DownloadResult {
	derr := DownloadError{nil, false}
	m.SetCacheFolder(path.Join(cache.Folder, m.Hash())) // TODO: This is a mess

	if m.IsUpToDate() {
		return DownloadResult{err: DownloadError{nil, false}, skipped: true, willRetry: false, m: m}
	}

	if err := os.RemoveAll(m.Folder()); err != nil {
		log.Fatalf("Error removing folder: %s", m.Folder())
	}

	if derr = m.Download(); derr.error != nil {
		return DownloadResult{err: derr, skipped: false, willRetry: true, m: m}
	}

	return DownloadResult{err: DownloadError{nil, false}, skipped: false, willRetry: false, m: m}
}

func downloadModules(modules chan PuppetModule, results chan DownloadResult, cache *Cache) {
	maxTries := 3
	retryDelay := 5 * time.Second

	for m := range modules {
		cache.LockModule(m.Hash())

		dres := downloadModule(m, cache)
		for i := 0; dres.err.error != nil && i < maxTries-1 && dres.err.retryable; i++ {
			go func(dres DownloadResult) {
				results <- dres
			}(dres)
			time.Sleep(retryDelay)
			dres = downloadModule(m, cache)
		}

		go func(dres DownloadResult) {
			results <- dres
		}(dres)

		cache.UnlockModule(m.Hash())
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
			}
			res.m.Processed()
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
		if cliOpts["--puppetfile"] == nil {
			wd, _ := os.Getwd()
			puppetfile := path.Join(wd, "Puppetfile")
			pf := NewPuppetFile(puppetfile, path.Join(path.Dir(puppetfile), "modules"))
			if pf == nil {
				log.Fatalf("no such file or directory %s", puppetfile)
			}
			puppetFiles = append(puppetFiles, pf)
		} else {
			puppetfile := cliOpts["--puppetfile"].(string)
			pf := NewPuppetFile(puppetfile, path.Join(path.Dir(puppetfile), "modules"))
			if pf == nil {
				log.Fatalf("no such file or directory %s", puppetfile)
			}
			puppetFiles = append(puppetFiles, pf)
		}
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		if len(puppetFiles) == 0 {
			wd, _ := os.Getwd()
			puppetfile := path.Join(wd, "Puppetfile")
			pf := NewPuppetFile(path.Join(wd, "Puppetfile"), path.Join(wd, "modules"))
			if pf == nil {
				log.Fatalf("no such file or directory %s", puppetfile)
			}
			puppetFiles = append(puppetFiles, pf)
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
			go func(file *PuppetFile) { moduleFiles <- file }(pf)
		}

		wg.Wait()
		close(modules)
		close(moduleFiles)
		<-done
		close(results)
		nErr := <-errorCount
		close(errorCount)

		os.Exit(nErr)
	}
}
