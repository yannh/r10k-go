package main

import (
	"github.com/yannh/r10k-go/git"
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

func getPuppetfilesForEnvironments(envs []string, sources map[string]source, cache *cache, moduledir string) []*puppetFile {
	var puppetFiles []*puppetFile
	var s source

	//for _, envName := range cliOpts["<env>"].([]string) {
	for _, envName := range envs {
		sourceName := ""

		// Find in which source the environment is
		// TODO: make deterministic
		//for name, source := range r10kConfig.Sources {
		for name, source := range sources {
			if git.RepoHasBranch(source.Remote, envName) {
				sourceName = name
				s = source
				break
			}
		}

		sourceCacheFolder := path.Join(cache.folder, sourceName)

		// Clone if environment doesnt exist, fetch otherwise
		if err := git.RevParse(sourceCacheFolder); err != nil {
			log.Printf("%v", sources["enviro1"])
			if err := git.Clone(sources[sourceName].Remote, git.Ref{Branch: envName}, sourceCacheFolder); err != nil {
				log.Fatalf("failed downloading environment: %v", err)
			}
		} else {
			git.Fetch(sourceCacheFolder)
		}

		git.Clone(sourceCacheFolder, git.Ref{Branch: envName}, path.Join(sources[sourceName].Basedir, envName))
		puppetfile := path.Join(sources[sourceName].Basedir, envName, "Puppetfile")

		pf := newPuppetFile(puppetfile, environment{s, envName, moduledir})
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}
		puppetFiles = append(puppetFiles, pf)
	}
	return puppetFiles
}
