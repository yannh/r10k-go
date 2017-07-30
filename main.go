package main

// TODO: Handle Signals
// TODO: Pagination for forge and github tarballs
// Todo: Extract outpud handling / support quiet, debug, json, ...
// Todo: Remove duplication between github_tarball_module & forge_module

import (
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

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

type DownloadError struct {
	error
	retryable bool
}

func (de *DownloadError) Retryable() bool {
	return de.retryable
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
			log.Fatal("Error getting current folder: %v", err)
		}

		if err = os.RemoveAll(path.Join(cwd, m.TargetFolder())); err != nil {
			log.Fatal("Error removing folder: %s", path.Join(cwd, m.TargetFolder()))
		}

		derr = m.Download()
		for i := 0; derr.error != nil && i < maxTries-1 && derr.Retryable(); i++ {
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
		if _, ok := modules[m.Name()]; !ok {
			modules[m.Name()] = true
			m.SetTargetFolder(path.Join(environmentRootFolder, m.TargetFolder()))
			m.SetCacheFolder(path.Join(cache.folder, m.Hash()))
			out <- m
		} else {
			m.Processed()
		}
	}
	done <- true
}

func processModuleFiles(puppetFiles <-chan *PuppetFile, metadataFiles <-chan *MetadataFile, modules chan PuppetModule, wg *sync.WaitGroup, done chan bool) {
	for {
		select {
		case f, ok := <-puppetFiles:
			if ok {
				NewPuppetFile(f).process(modules, func() { wg.Done() })
			} else {
				puppetFiles = nil
			}
		case f, ok := <-metadataFiles:
			if ok {
				NewMetadataFile(f).process(modules, func() { wg.Done() })
			} else {
				metadataFiles = nil
			}
		}
		if puppetFiles == nil && metadataFiles == nil {
			break
		}
	}
	done <- true
}

func parseResults(results <-chan DownloadResult, downloadDeps bool, metadataFiles chan<- *MetadataFile, wg *sync.WaitGroup, errorsCount chan<- int) {
	downloadErrors := 0

	for res := range results {
		if res.err.error != nil {
			if res.err.Retryable() == true && res.willRetry == true {
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
			if file, err := os.Open(path.Join(res.m.TargetFolder(), "metadata.json")); err == nil {
				wg.Add(1)
				go func() {
					metadataFiles <- NewMetadataFile(file)
				}()
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
		} else {
			environmentRootFolder = path.Join("environment", cliOpts["<ENV>"].(string))
		}

		results := make(chan DownloadResult)
		modules := make(chan PuppetModule)
		modulesDeduplicated := make(chan PuppetModule)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(modulesDeduplicated, results)
		}

		var wg sync.WaitGroup
		puppetFiles := make(chan *PuppetFile)
		metadataFiles := make(chan *MetadataFile)

		done := make(chan bool)
		errorCount := make(chan int)

		go processModuleFiles(puppetFiles, metadataFiles, modules, &wg, done)
		go deduplicate(modules, modulesDeduplicated, &cache, environmentRootFolder, done)
		go parseResults(results, !cliOpts["--no-deps"].(bool), metadataFiles, &wg, errorCount)

		file, err := os.Open(puppetfile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		wg.Add(1)
		// TODO: file handler get leaked
		puppetFiles <- NewPuppetFile(file)

		// +1 For every file being processed or module in the queue
		wg.Wait()

		close(modules)
		close(modulesDeduplicated)
		close(puppetFiles)
		close(metadataFiles)
		close(results)

		<-done
		<-done
		nErr := <-errorCount
		close(errorCount)

		os.Exit(nErr)
	}
}
