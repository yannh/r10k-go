package main

import "sync"
import "os"
import "log"
import "io"
import "time"
import "fmt"
import "strconv"

type PuppetModule interface {
	Name() string
	Download() (string, error)
	SetTargetFolder(string)
	TargetFolder() string
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

func cloneWorker(c chan PuppetModule, modules *Modules, wg *sync.WaitGroup) {
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
		} else {
			modules.m[m.Name()] = true
		}
		modules.Unlock()

		derr = nil
		if _, err = os.Stat(m.TargetFolder()); err != nil {
			if _, err = m.Download(); err != nil {
				derr, ok = err.(DownloadError)
				for i := 0; i < maxTries-1 && ok && derr.Retryable(); i++ {
					time.Sleep(retryDelay)
					_, err = m.Download()
					derr, ok = err.(DownloadError)
				}
			}

			if derr == nil {
				fmt.Println("Downloaded " + m.Name() + " to " + m.TargetFolder())
			} else {
				fmt.Println("Failed downloading " + m.Name() + ": " + derr.Error())
			}
		}

		if file, err := os.Open(m.TargetFolder() + "/metadata.json"); err == nil {
			defer file.Close()
			wg.Add(1)
			go parser.parse(file, c, wg)
		}

		wg.Done()
	}
}

func main() {
	var err error
	var numWorkers int
	var puppetfile string

	opts := cli()

	if opts["install"] == true {
		if opts["--workers"] == nil {
			numWorkers = 4
		} else {
			numWorkers, err = strconv.Atoi(opts["--workers"].(string))
			if err != nil {
				fmt.Println("Parameter --workers should be an integer")
			}
		}

		if opts["--puppetfile"] == nil {
			puppetfile = "Puppetfile"
		} else {
			puppetfile = opts["--puppetfile"].(string)
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
			go cloneWorker(modulesChan, &modules, &wg)
		}

		parser := PuppetFileParser{}
		parser.parse(file, modulesChan, &wg)

		wg.Wait()
		close(modulesChan)
	}
}
