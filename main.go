package main

// TODO: Return 1 if at least one module failed to download

import (
	"fmt"
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
}

type Parser interface {
	parse(puppetFile io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup, environment string) error
}

type DownloadError interface {
	error
	Retryable() bool
}

type Modules struct {
	*sync.Mutex
	m map[string]bool
}

func cloneWorker(c chan PuppetModule, modules *Modules, cache Cache, wg *sync.WaitGroup, environmentRootFolder string) {
	var err error
	var derr DownloadError
	var ok bool

	parser := MetadataParser{}
	maxTries := 3
	retryDelay := 3 * time.Second

	for m := range c {
		m.SetCacheFolder(path.Join(cache.folder, m.Hash()))
		m.SetTargetFolder(path.Join(environmentRootFolder, m.TargetFolder()))

		modules.Lock()
		if _, ok := modules.m[m.Name()]; ok {
			wg.Done()
			modules.Unlock()
			continue
		}
		modules.m[m.Name()] = true
		modules.Unlock()

		derr = nil
		if _, err = os.Stat(m.TargetFolder()); err != nil {
			if err = m.Download(); err != nil {
				derr, ok = err.(DownloadError)
				for i := 0; err != nil && i < maxTries-1 && ok && derr.Retryable(); i++ {
					fmt.Println("Failed downloading " + m.Name() + ": " + derr.Error() + ". Retrying...")
					time.Sleep(retryDelay)
					err = m.Download()
					derr, ok = err.(DownloadError)
				}
			}

			if derr == nil {
				fmt.Println("Downloaded " + m.Name() + " to " + m.TargetFolder())
			} else {
				fmt.Println("Failed downloading " + m.Name() + ": " + derr.Error() + ". Giving up")
			}
		}

		if file, err := os.Open(path.Join(m.TargetFolder(), "metadata.json")); err == nil {
			wg.Add(1)
			go func() {
				parser.parse(file, c, wg)
				file.Close()
			}()
		}

		wg.Done()
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

	fmt.Println(cliOpts)

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

		file, err := os.Open(puppetfile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		modulesChan := make(chan PuppetModule)
		var wg sync.WaitGroup

		modules := Modules{
			&sync.Mutex{},
			make(map[string]bool)}

		if cliOpts["environment"] == false {
			environmentRootFolder = "."
		} else {
			environmentRootFolder = path.Join("environment", cliOpts["<ENV>"].(string))
		}

		for w := 1; w <= numWorkers; w++ {
			go cloneWorker(modulesChan, &modules, cache, &wg, environmentRootFolder)
		}

		parser := PuppetFileParser{}
		parser.parse(file, modulesChan, &wg)

		wg.Wait()
		close(modulesChan)
	}
}
