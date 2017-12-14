package main

import (
	"github.com/yannh/r10k-go/puppetfileparser"
	"log"
	"path"
	"sync"
)

func installPuppetFiles(puppetFiles []*puppetFile, numWorkers int, cache *cache, withDeps bool, limitToModules ...string) int {
	drs := make(chan downloadRequest)

	var wg sync.WaitGroup
	errorCount := make(chan int)

	for w := 1; w <= numWorkers; w++ {
		go downloadModules(drs, cache, withDeps, &wg, errorCount)
	}

	for _, pf := range puppetFiles {
		wg.Add(1)
		go func(pf *puppetFile, drs chan downloadRequest) {
			if err := pf.Process(drs, limitToModules...); err != nil {
				if serr, ok := err.(puppetfileparser.ErrMalformedPuppetfile); ok {
					log.Fatal(serr)
				} else {
					log.Printf("failed parsing %s: %v\n", pf.filename, err)
				}
			}

			pf.Close()
			wg.Done()
		}(pf, drs)
	}

	wg.Wait()
	close(drs)

	nErr := 0
	for w := 1; w <= numWorkers; w++ {
		nErr += <-errorCount
	}
	close(errorCount)
	return nErr
}

func getPuppetFileForEnvironment(env environment, moduledir string, cache *cache) *puppetFile {
	if env.fetch(cache) != nil {
		log.Fatal("Failed fetching environment " + env.branch)
	}

	puppetfile := path.Join(env.source.Basedir, env.branch, "Puppetfile")

	pf := newPuppetFile(puppetfile, environment{env.source, env.branch, moduledir})
	if pf == nil {
		log.Fatalf("no such file or directory %s", puppetfile)
	}
	return pf
}
