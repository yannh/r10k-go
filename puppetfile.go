package main

import (
	"bufio"
	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"os"
	"sync"
)

type PuppetFile struct {
	*os.File // Make that a io.Reader
	filename string
	env      environment
}

func NewPuppetFile(puppetfile string, env environment) *PuppetFile {
	f, err := os.Open(puppetfile)
	if err != nil {
		return nil
	}

	return &PuppetFile{File: f, filename: puppetfile, env: env}
}

func (p *PuppetFile) ToTypedModule(module map[string]string) PuppetModule {
	switch module["type"] {
	case "git":
		return &GitModule{
			name:        module["name"],
			repoURL:     module["repoUrl"],
			installPath: module["installPath"],
			want: git.Ref{
				Ref:    module["ref"],
				Tag:    module["tag"],
				Branch: module["branch"],
			},
		}

	case "github_tarball":
		return &GithubTarballModule{
			name:     module["name"],
			repoName: module["repoName"],
			version:  module["version"],
		}

	default:
		return &ForgeModule{
			name:    module["name"],
			version: module["version"],
		}
	}
}

func (p *PuppetFile) Close() { p.File.Close() }

// Will download all modules in the Puppetfile
// limitToModules is a list of module names - if set, only those will be downloaded
func (p *PuppetFile) Process(drs chan<- downloadRequest, limitToModules ...string) error {
	var wg sync.WaitGroup

	parsedModules, _, err := puppetfileparser.Parse(bufio.NewScanner(p.File))
	if err != nil {
		return puppetfileparser.ErrMalformedPuppetfile{S: err.Error()}
	}

	for _, module := range parsedModules {
		if len(limitToModules) > 0 {
			for _, moduleName := range limitToModules {
				if module["name"] != moduleName && folderFromModuleName(module["name"]) != moduleName {
					continue
				}
			}
		}

		dr := downloadRequest{
			m:    p.ToTypedModule(module),
			env:  p.env,
			done: make(chan bool),
		}

		wg.Add(1)
		go func() {
			drs <- dr
			<-dr.done
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}
