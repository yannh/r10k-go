package main

// TODO: Handle Signals
// TODO: Pagination for forge and github tarballs
// Todo: Extract outpud handling / support quiet, debug, json, ...
// Todo: Remove duplication between github_tarball_module & forge_module
// TODO: fix name/target folder with dashes or slashes in name

import (
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

// ForgeModule, GitModule, GithubTarballModule, ....
type PuppetModule interface {
	Name() string
	Download() DownloadError
	SetTargetFolder(string)
	TargetFolder() string
	SetCacheFolder(string)
	Hash() string
	IsUpToDate() bool
	Processed()
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

func downloadModules(c chan PuppetModule, results chan DownloadResult) {
	maxTries := 3
	retryDelay := 5 * time.Second

	for m := range c {
		derr := DownloadError{nil, false}

		if m.IsUpToDate() {
			results <- DownloadResult{err: DownloadError{nil, false}, skipped: true, willRetry: false, m: m}
			continue
		}

		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current folder: %v", err)
		}

		if err = os.RemoveAll(path.Join(cwd, m.TargetFolder())); err != nil {
			log.Fatalf("Error removing folder: %s", path.Join(cwd, m.TargetFolder()))
		}

		derr = m.Download()
		for i := 0; derr.error != nil && i < maxTries-1 && derr.retryable; i++ {
			results <- DownloadResult{err: derr, skipped: false, willRetry: true, m: m}
			time.Sleep(retryDelay)
			derr = m.Download()
		}

		if derr.error != nil {
			results <- DownloadResult{err: derr, skipped: false, willRetry: false, m: m}
			continue
		}

		// Success
		results <- DownloadResult{err: DownloadError{nil, false}, skipped: false, willRetry: false, m: m}
	}
}

func deduplicate(in <-chan PuppetModule, out chan<- PuppetModule, cache *Cache, environmentRootFolder string, done chan<- bool) {
	modules := make(map[string]bool)

	for m := range in {
		if _, ok := modules[m.Name()]; ok {
			m.Processed()
			continue
		}

		modules[m.Name()] = true
		m.SetTargetFolder(path.Join(environmentRootFolder, m.TargetFolder()))
		m.SetCacheFolder(path.Join(cache.folder, m.Hash()))
		out <- m
	}

	done <- true
}

func processModuleFiles(moduleFiles <-chan moduleFile, modules chan PuppetModule, wg *sync.WaitGroup, done chan bool) {
	for mf := range moduleFiles {
		if err := mf.Process(modules, func() { wg.Done() }); err != nil {
			log.Printf("failed parsing puppetfile %s: %v\n", mf.Filename(), err)
		}
		mf.Close()
	}

	done <- true
}

func parseResults(results <-chan DownloadResult, downloadDeps bool, metadataFiles chan<- moduleFile, wg *sync.WaitGroup, errorsCount chan<- int) {
	downloadErrors := 0

	for res := range results {
		if res.err.error != nil {
			if res.err.retryable == true && res.willRetry == true {
				log.Println("Failed downloading " + res.m.Name() + ": " + res.err.Error() + ". Retrying...")
			} else {
				log.Println("Failed downloading " + res.m.Name() + ". Giving up!")
				downloadErrors += 1
				res.m.Processed()
			}
			continue
		}

		if res.skipped != true {
			log.Println("Downloaded " + res.m.Name())
		}

		if downloadDeps {
			mf := NewMetadataFile(path.Join(res.m.TargetFolder(), "metadata.json"))
			if mf != nil {
				wg.Add(1)
				metadataFiles <- mf
			}
		}

		res.m.Processed()
	}

	errorsCount <- downloadErrors
}

func main() {
	var err error
	var numWorkers int
	var cache Cache

	cliOpts := cli()
	if cache, err = NewCache(".cache"); err != nil {
		log.Fatal(err)
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		if cliOpts["--workers"] == nil {
			numWorkers = 4
		} else {
			numWorkers, err = strconv.Atoi(cliOpts["--workers"].(string))
			if err != nil {
				log.Fatalf("Parameter --workers should be an integer")
			}
		}

		var puppetfile, environmentRootFolder string

		if cliOpts["--puppetfile"] == nil {
			puppetfile = "Puppetfile"
		} else {
			puppetfile = cliOpts["--puppetfile"].(string)
		}

		if cliOpts["environment"] == false {
			environmentRootFolder = "."
		}

		results := make(chan DownloadResult)
		modules := make(chan PuppetModule)
		modulesDeduplicated := make(chan PuppetModule)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(modulesDeduplicated, results)
		}

		var wg sync.WaitGroup
		moduleFiles := make(chan moduleFile)

		done := make(chan bool)
		errorCount := make(chan int)

		go processModuleFiles(moduleFiles, modules, &wg, done)
		go deduplicate(modules, modulesDeduplicated, &cache, environmentRootFolder, done)
		go parseResults(results, !cliOpts["--no-deps"].(bool), moduleFiles, &wg, errorCount)

		if pf := NewPuppetFile(puppetfile); pf != nil {
			wg.Add(1)
			moduleFiles <- pf
		}

		// +1 For every file being processed or module in the queue
		wg.Wait()
		close(modules)
		close(modulesDeduplicated)
		close(moduleFiles)
		close(results)

		<-done
		<-done
		nErr := <-errorCount
		close(errorCount)

		os.Exit(nErr)
	}
}
