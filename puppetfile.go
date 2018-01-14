package main

import (
	"bufio"
	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"github.com/yannh/r10k-go/puppetmodule"
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

func (p *puppetFile) toTypedModule(module map[string]string) puppetmodule.PuppetModule {
	switch module["type"] {
	case "git":
		var ref *git.Ref

		switch {
		case module["ref"] != "":
			ref = git.NewRef(git.TypeRef, module["ref"])
		case module["tag"] != "":
			ref = git.NewRef(git.TypeTag, module["tag"])
		case module["branch"] != "":
			ref = git.NewRef(git.TypeRef, module["branch"])
		}

		return &puppetmodule.GitModule{
			Name:        module["name"],
			RepoURL:     module["repoUrl"],
			InstallPath: module["installPath"],
			Want:        ref,
		}

	case "github_tarball":
		return &puppetmodule.GithubTarballModule{
			Name:     module["name"],
			RepoName: module["repoName"],
			Version:  module["version"],
		}

	default:
		return &puppetmodule.ForgeModule{
			Name:    module["name"],
			Version: module["version"],
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
		go func(dr downloadRequest) {
			drs <- dr
		}(dr)
	}

	for i := 0; i < nDownloadRequests; i++ {
		<-done
	}

	return nil
}
