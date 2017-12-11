package main

import (
	"bufio"
	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"os"
)

type puppetFile struct {
	*os.File // Make that a io.Reader
	filename string
	env      environment
}

func newPuppetFile(pf string, env environment) *puppetFile {
	f, err := os.Open(pf)
	if err != nil {
		return nil
	}

	return &puppetFile{File: f, filename: pf, env: env}
}

func (p *puppetFile) toTypedModule(module map[string]string) puppetModule {
	switch module["type"] {
	case "git":
		return &gitModule{
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
		return &githubTarballModule{
			name:     module["name"],
			repoName: module["repoName"],
			version:  module["version"],
		}

	default:
		return &forgeModule{
			name:    module["name"],
			version: module["version"],
		}
	}
}

func (p *puppetFile) Close() { p.File.Close() }

// Will download all modules in the Puppetfile
// limitToModules is a list of module names - if set, only those will be downloaded
func (p *puppetFile) Process(drs chan<- downloadRequest, limitToModules ...string) error {
	done := make(chan bool)

	parsedModules, _, err := puppetfileparser.Parse(bufio.NewScanner(p.File))
	if err != nil {
		return puppetfileparser.ErrMalformedPuppetfile{S: err.Error()}
	}

	nDownloadRequests := 0
	for _, module := range parsedModules {
		if len(limitToModules) > 0 {
			for _, moduleName := range limitToModules {
				if module["name"] != moduleName && folderFromModuleName(module["name"]) != moduleName {
					continue
				}
			}
		}

		dr := downloadRequest{
			m:    p.toTypedModule(module),
			env:  p.env,
			done: done,
		}

		nDownloadRequests++
		go func() {
			drs <- dr
		}()
	}

	for i := 0; i < nDownloadRequests; i++ {
		<-done
	}

	return nil
}
