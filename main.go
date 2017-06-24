package main

import "sync"
import "os"
import "path"
import "log"
import "io"
import "time"
import "fmt"
import "strconv"

type PuppetModule interface {
	Name() string
	Download(Cache) (string, error)
	SetTargetFolder(string)
	TargetFolder() string
	Hash() string
}

type Parser interface {
	parse(puppetFile io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup) error
}

type DownloadError interface {
	error
	Retryable() bool
}

type Modules struct {
	*sync.Mutex
	m map[string]bool
}

func cloneWorker(c chan PuppetModule, modules *Modules, cache Cache, wg *sync.WaitGroup) {
	var err error
	var derr DownloadError
	var ok bool

	parser := MetadataParser{}
	maxTries := 3
	retryDelay := 3 * time.Second

	for m := range c {
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
			if _, err = m.Download(cache); err != nil {
				derr, ok = err.(DownloadError)
				for i := 0; i < maxTries-1 && ok && derr.Retryable(); i++ {
					time.Sleep(retryDelay)
					_, err = m.Download(cache)
					derr, ok = err.(DownloadError)
				}
			}

			if derr == nil {
				fmt.Println("Downloaded " + m.Name() + " to " + m.TargetFolder())
			} else {
				fmt.Println("Failed downloading " + m.Name() + ": " + derr.Error())
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
	var puppetfile string

	cliOpts := cli()
	cache, err := NewCache(".tmp/")
	if err != nil {
		log.Fatal(err)
	}

	if cliOpts["install"] == true {
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

		for w := 1; w <= numWorkers; w++ {
			go cloneWorker(modulesChan, &modules, cache, &wg)
		}

		parser := PuppetFileParser{}
		parser.parse(file, modulesChan, &wg)

		wg.Wait()
		close(modulesChan)
	}
}
