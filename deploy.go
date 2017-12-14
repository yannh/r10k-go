package main

import (
	"fmt"
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

func getEnvironments(envNames []string, sources []gitSource) []environment {
	envs := make([]environment, 0)

	for _, envName := range envNames {
		// Find in which gitSource the environment is
		// TODO: make deterministic
		found := false
		for _, source := range sources {
			if git.RepoHasRemoteBranch(source.Remote, envName) {
				envs = append(envs, newEnvironment(source, envName))
				found = true
				break
			}
		}
		if found == false {
			log.Printf("failed to find source for environment %s", envName)
		}
	}

	fmt.Printf("%+v\n", envs)
	return envs
}

func getPuppetFilesForEnvironments(envs []environment, moduledir string, cache *cache) []*puppetFile {
	puppetFiles := make([]*puppetFile, 0)
	for _, env := range envs {
		sourceCacheFolder := path.Join(cache.folder, env.source.Name)
		env.source.fetch(cache)

		if err := git.Checkout(sourceCacheFolder, env.branch); err != nil {
			log.Fatal(err)
		}
		if err := git.Clone(sourceCacheFolder, git.Ref{Branch: env.branch}, path.Join(env.source.Basedir, env.branch)); err != nil {
			log.Fatal(err)
		}
		puppetfile := path.Join(env.source.Basedir, env.branch, "Puppetfile")

		pf := newPuppetFile(puppetfile, environment{env.source, env.branch, moduledir})
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}
		puppetFiles = append(puppetFiles, pf)
	}

	return puppetFiles
}
