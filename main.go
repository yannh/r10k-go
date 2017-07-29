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

func cloneWorker(c chan PuppetModule, cache Cache, environmentRootFolder string, resultsChan chan DownloadResult) {
	var err error
	var derr DownloadError
	var ok bool

	maxTries := 3
	retryDelay := 5 * time.Second

	for m := range c {
		dr := DownloadResult{err: nil, willRetry: false, m: m}

		m.SetCacheFolder(path.Join(cache.folder, m.Hash()))
		m.SetTargetFolder(path.Join(environmentRootFolder, m.TargetFolder()))

		derr = nil
		if !m.IsUpToDate() {
			cwd, _ := os.Getwd()
			os.RemoveAll(path.Join(cwd, m.TargetFolder()))

			if err = m.Download(); err != nil {
				derr, ok = err.(DownloadError)
				for i := 0; ok && derr != nil && i < maxTries-1 && derr.Retryable(); i++ {
					dr.err = derr
					dr.willRetry = true
					resultsChan <- dr

					time.Sleep(retryDelay)
					err = m.Download()
					derr, ok = err.(DownloadError)
				}
			}

			if derr != nil {
				dr.willRetry = false
				dr.err = derr
			}

			resultsChan <- dr
		}
	}
}

func main() {
	var err error
	var numWorkers int
	var puppetfile, environmentRootFolder string
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

		if cliOpts["--puppetfile"] == nil {
			puppetfile = "Puppetfile"
		} else {
			puppetfile = cliOpts["--puppetfile"].(string)
		}

		var wg sync.WaitGroup

		if cliOpts["environment"] == false {
			environmentRootFolder = "."
		} else {
			environmentRootFolder = path.Join("environment", cliOpts["<ENV>"].(string))
		}

		results := make(chan DownloadResult)

		modulesUnfiltered := make(chan PuppetModule)
		modulesFiltered := make(chan PuppetModule)

		for w := 1; w <= numWorkers; w++ {
			go cloneWorker(modulesFiltered, cache, environmentRootFolder, results)
		}

		puppetFiles := make(chan *PuppetFile)
		metadataFiles := make(chan *MetadataFile)
		resultParsingFinished := make(chan bool)
		downloadErrors := 0
		go func() {
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

					if !cliOpts["--no-deps"].(bool) {
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
			resultParsingFinished <- true
			log.Println("Parsing results finished")
		}()


		modulesFilteringFinished := make(chan bool)
		go func() {
			modules := make(map[string]bool)

			for {
				select {
				case m, ok := <-modulesUnfiltered:
					if ok {
						if _, ok := modules[m.Name()]; !ok {
							modules[m.Name()] = true
							wg.Add(1)
							modulesFiltered <- m
						}
					} else {
						modulesUnfiltered = nil
					}
				}

				if modulesUnfiltered == nil {
					break
				}
			}
			log.Println("Finished filtering modules")
			modulesFilteringFinished <- true
		}()

		fileParsingFinished := make(chan bool)
		go func() {
			for {
				select {
				case f, ok := <-puppetFiles:
					if ok {
						(&PuppetFile{f}).parse(modulesUnfiltered)
						wg.Done()
					} else {
						puppetFiles = nil
					}
				case f, ok := <-metadataFiles:
					if ok {
						(&MetadataFile{f}).parse(modulesUnfiltered)
						wg.Done()
					} else {
						metadataFiles = nil
					}
				}

				if puppetFiles == nil && metadataFiles == nil {
					break
				}
			}
			log.Println("Finished parsing files")
			fileParsingFinished <- true
		}()

		file, err := os.Open(puppetfile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		wg.Add(1)
		puppetFiles <- &PuppetFile{file}

		wg.Wait()

		close(modulesUnfiltered)
		close(modulesFiltered)
		close(puppetFiles)
		close(metadataFiles)
		close(results)

		<-resultParsingFinished
		<-fileParsingFinished
		<-modulesFilteringFinished

		os.Exit(downloadErrors)
	}
}
