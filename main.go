package main

// TODO Fix installpath
// Pass a DownloadRequest to DownloadModules instead of Module: Module + Environment

import (
	"github.com/yannh/r10k-go/git"
	"log"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

// ForgeModule, GitModule, GithubTarballModule, ....
type PuppetModule interface {
	IsUpToDate(string) bool
	Name() string
	Download(string, *Cache) *DownloadError
	Folder() string
	Hash() string
	Processed()
	InstallPath() string
}

type DownloadError struct {
	error
	retryable bool
}

type DownloadResult struct {
	err     *DownloadError
	skipped bool
}

type source struct {
	Basedir string
	Prefix  string
	Remote  string
}

type environment struct {
	source
	branch string
}

type downloadRequest struct {
	m    PuppetModule
	env  environment
	done chan bool
}

func downloadModule(m PuppetModule, to string, cache *Cache) DownloadResult {
	if m.IsUpToDate(to) {
		return DownloadResult{err: nil, skipped: true}
	}

	if err := os.RemoveAll(to); err != nil {
		log.Fatalf("Error removing folder: %s", m.Folder())
	}

	if derr := m.Download(to, cache); derr != nil {
		return DownloadResult{err: derr, skipped: false}
	}

	return DownloadResult{err: nil, skipped: false}
}

func downloadModules(drs chan downloadRequest, cache *Cache, downloadDeps bool, wg *sync.WaitGroup, errorsCount chan<- int) {
	maxTries := 3
	retryDelay := 5 * time.Second
	errors := 0

	for dr := range drs {
		cache.LockModule(dr.m.Hash())

		modulesFolder := path.Join(dr.env.Basedir, "modules")
		if dr.m.InstallPath() != "" {
			modulesFolder = path.Join(dr.env.Basedir, dr.m.InstallPath())
		}

		to := path.Join(modulesFolder, dr.m.Folder())

		dres := downloadModule(dr.m, to, cache)
		for i := 1; dres.err != nil && dres.err.retryable && i < maxTries; i++ {
			log.Printf("failed downloading %s: %v... Retrying\n", dr.m.Name(), dres.err)
			time.Sleep(retryDelay)
			dres = downloadModule(dr.m, to, cache)
		}

		if dres.err == nil {
			if downloadDeps && !dres.skipped {
				if mf := NewMetadataFile(path.Join(to, "metadata.json"), dr.env); mf != nil {
					wg.Add(1)
					go func() {
						if err := mf.Process(drs); err != nil {
							log.Printf("failed parsing %s: %v\n", mf.Filename(), err)
						}

						mf.Close()
						wg.Done()
					}()
				}
			}

			if !dres.skipped {
				log.Println("Downloaded " + dr.m.Name() + " to " + to)
			}
		} else {
			log.Printf("failed downloading %s: %v. Giving up!\n", dr.m.Name(), dres.err)
			errors++
		}

		dr.m.Processed()
		cache.UnlockModule(dr.m.Hash())
	}

	errorsCount <- errors
}

func main() {
	var err error
	var numWorkers int
	var cache *Cache

	cliOpts := cli()

	if cliOpts["--workers"] == nil {
		numWorkers = 4
	} else {
		numWorkers, err = strconv.Atoi(cliOpts["--workers"].(string))
		if err != nil {
			log.Fatalf("Parameter --workers should be an integer")
		}
	}

	cacheDir := ".cache"
	var puppetFiles []*PuppetFile

	if cache, err = NewCache(cacheDir); err != nil {
		log.Fatal(err)
	}

	if cliOpts["deploy"] == true {
		r10kFile := "r10k.yml"
		r10kConfig, err := NewR10kConfig(r10kFile)
		if err != nil {
			log.Fatalf("Error parsing r10k configuration file %s: %v", r10kFile, err)
		}

		if r10kConfig.Cachedir != "" {
			cacheDir = r10kConfig.Cachedir
		}

		var s source

		// Find in which source the environment is
		// TODO: render deterministic
		for _, envName := range cliOpts["<env>"].([]string) {
			environmentSource := ""

			for sourceName, source := range r10kConfig.Sources {
				if git.RepoHasBranch(source.Remote, envName) {
					environmentSource = sourceName
					s = source
					break
				}
			}

			sourceCacheFolder := path.Join(cacheDir, environmentSource)
			// Clone if environment doesnt exist, fetch otherwise
			if err := git.RevParse(sourceCacheFolder); err != nil {
				if err := git.Clone(r10kConfig.Sources[environmentSource].Remote, git.Ref{Branch: envName}, sourceCacheFolder); err != nil {
					log.Fatalf("failed downloading environment: %v", err)
				}
			} else {
				git.Fetch(sourceCacheFolder)
			}

			git.WorktreeAdd(sourceCacheFolder, git.Ref{Branch: envName}, path.Join(r10kConfig.Sources[environmentSource].Basedir, envName))
			puppetfile := path.Join(r10kConfig.Sources[environmentSource].Basedir, envName, "Puppetfile")

			pf := NewPuppetFile(puppetfile, environment{s, envName})
			if pf == nil {
				log.Fatalf("no such file or directory %s", puppetfile)
			}
			puppetFiles = append(puppetFiles, pf)
		}
	}

	if cliOpts["install"] == true {
		puppetfile := ""
		if cliOpts["--puppetfile"] == nil {
			wd, _ := os.Getwd()
			puppetfile = path.Join(wd, "Puppetfile")
		} else {
			puppetfile = cliOpts["--puppetfile"].(string)
		}

		pf := NewPuppetFile(puppetfile, environment{source{Basedir: path.Dir(puppetfile), Prefix: "", Remote: ""}, ""})
		if pf == nil {
			log.Fatalf("no such file or directory %s", puppetfile)
		}

		puppetFiles = append(puppetFiles, pf)
	}

	if cliOpts["install"] == true || cliOpts["deploy"] == true {
		drs := make(chan downloadRequest)

		var wg sync.WaitGroup
		errorCount := make(chan int)

		for w := 1; w <= numWorkers; w++ {
			go downloadModules(drs, cache, !cliOpts["--no-deps"].(bool), &wg, errorCount)
		}

		for _, pf := range puppetFiles {
			wg.Add(1)
			go func(pf *PuppetFile, drs chan downloadRequest) {
				if err := pf.Process(drs); err != nil {
					if serr, ok := err.(ErrMalformedPuppetfile); ok {
						log.Fatal(serr)
					} else {
						log.Printf("failed parsing %s: %v\n", pf.Filename(), err)
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

		os.Exit(nErr)
	}
}
