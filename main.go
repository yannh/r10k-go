package main

// TODO: Return 1 if at least one module failed to download

import (
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

type PuppetModule interface {
	Name() string
	Download() error
	SetTargetFolder(string)
	TargetFolder() string
	SetCacheFolder(string)
	Hash() string
	IsUpToDate() bool
}

type Parser interface {
	parse(r io.Reader, modulesChan chan<- PuppetModule) (int, error)
}

type DownloadResult struct {
	err       DownloadError
	willRetry bool
	m         PuppetModule
}

type DownloadError interface {
	error
	Retryable() bool
}

func downloadModules(c chan PuppetModule, results chan DownloadResult) {
	var derr DownloadError
	var ok bool

	maxTries := 3
	retryDelay := 5 * time.Second

	for m := range c {
		dr := DownloadResult{err: nil, willRetry: false, m: m}

		derr = nil
		if !m.IsUpToDate() {
			cwd, err := os.Getwd()
			if err != nil {
				log.Fatal("Error getting current folder: %v", err)
			}
			os.RemoveAll(path.Join(cwd, m.TargetFolder()))

			if err = m.Download(); err != nil {
				derr, ok = err.(DownloadError)
				for i := 0; ok && derr != nil && i < maxTries-1 && derr.Retryable(); i++ {
					dr.err = derr
					dr.willRetry = true
					results <- dr

					time.Sleep(retryDelay)
					err = m.Download()
					derr, ok = err.(DownloadError)
				}
			}

			if derr != nil {
				dr.willRetry = false
				dr.err = derr
			}
		}

		results <- dr
	}
}

func deduplicate(in <-chan PuppetModule, out chan<- PuppetModule, cache *Cache, environmentRootFolder string, wg *sync.WaitGroup, done chan<- bool) {
	modules := make(map[string]bool)

	for m := range in {
		if _, ok := modules[m.Name()]; !ok {
			modules[m.Name()] = true
			wg.Add(1)
			m.SetTargetFolder(path.Join(environmentRootFolder, m.TargetFolder()))
			m.SetCacheFolder(path.Join(cache.folder, m.Hash()))
			out <- m
		}
	}
	done <- true
}

func parseModuleFiles(puppetFiles <-chan *PuppetFile, metadataFiles <-chan *MetadataFile, modules chan PuppetModule, wg *sync.WaitGroup, done chan<- bool) {
	for {
		select {
		case f, ok := <-puppetFiles:
			if ok {
				(&PuppetFile{f}).parse(modules)
				wg.Done()
			} else {
				puppetFiles = nil
			}
		case f, ok := <-metadataFiles:
			if ok {
				(&MetadataFile{f}).parse(modules)
				wg.Done()
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

func parseResults(results <-chan DownloadResult, downloadDeps bool, metadataFiles chan<- *MetadataFile, wg *sync.WaitGroup, done chan<- bool) {
	downloadErrors := 0

	for res := range results {
		if res.err != nil {
			if res.err.Retryable() == true && res.willRetry == true {
				log.Println("Failed downloading " + res.m.Name() + ": " + res.err.Error() + ". Retrying...")
			} else {
				log.Println("Failed downloading " + res.m.Name() + ". Giving up!")
				downloadErrors += 1
				wg.Done()
			}
		} else {
			log.Println("Downloaded " + res.m.Name())

			if downloadDeps {
				if file, err := os.Open(path.Join(res.m.TargetFolder(), "metadata.json")); err == nil {
					wg.Add(1)
					go func() {
						metadataFiles <- &MetadataFile{file}
					}()
				}
			}
			wg.Done()
		}
	}

	done <- true
}

func main() {
	var err error
	var numWorkers int
	var cache Cache

	cliOpts := cli()
	if cache, err = NewCache(".tmp"); err != nil {
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
		downloadErrors := 0

		done := make(chan bool)
		go parseModuleFiles(puppetFiles, metadataFiles, modules, &wg, done)
		go deduplicate(modules, modulesDeduplicated, &cache, environmentRootFolder, &wg, done)
		go parseResults(results, !cliOpts["--no-deps"].(bool), metadataFiles, &wg, done)

		file, err := os.Open(puppetfile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		wg.Add(1)
		puppetFiles <- &PuppetFile{file}
		wg.Wait()

		close(modules)
		close(modulesDeduplicated)
		close(puppetFiles)
		close(metadataFiles)
		close(results)

		for i := 0; i < 3; i++ {
			<-done
		}

		os.Exit(downloadErrors)
	}
}
