package main

import (
	"bufio"
	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetfileparser"
	"os"
	"path"
	"sync"
)

type PuppetFile struct {
	*os.File // Make that a io.Reader
	wg       *sync.WaitGroup
	filename string
	env      environment
}

func NewPuppetFile(puppetfile string, env environment) *PuppetFile {
	f, err := os.Open(puppetfile)
	if err != nil {
		return nil
	}

	return &PuppetFile{File: f, wg: &sync.WaitGroup{}, filename: puppetfile, env: env}
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

func (p *PuppetFile) Process(drs chan<- downloadRequest) error {
	parsedModules, opts, err := puppetfileparser.Parse(bufio.NewScanner(p.File))
	if err != nil {
		return puppetfileparser.ErrMalformedPuppetfile{S: err.Error()}
	}

	modulesDir := path.Join(p.env.Basedir, "modules")
	// The moduledir option in the Puppetfile overrides the default
	if _, ok := opts["moduledir"]; ok {
		modulesDir = opts["moduledir"]
		if !path.IsAbs(modulesDir) {
			modulesDir = path.Join(path.Dir(p.filename), modulesDir)
		}
	}

	for _, module := range parsedModules {
		dr := downloadRequest{
			m:    p.ToTypedModule(module),
			env:  p.env,
			done: make(chan bool),
		}

		p.wg.Add(1)
		go func() {
			drs <- dr
			<-dr.done
			p.wg.Done()
		}()
	}

	p.wg.Wait()
	return nil
}

func (p *PuppetFile) ProcessSingleModule(drs chan<- downloadRequest, moduleName string) error {
	parsedModules, opts, err := puppetfileparser.Parse(bufio.NewScanner(p.File))
	if err != nil {
		return puppetfileparser.ErrMalformedPuppetfile{S: err.Error()}
	}

	modulesDir := path.Join(p.env.Basedir, "modules")
	// The moduledir option in the Puppetfile overrides the default
	if _, ok := opts["moduledir"]; ok {
		modulesDir = opts["moduledir"]
		if !path.IsAbs(modulesDir) {
			modulesDir = path.Join(path.Dir(p.filename), modulesDir)
		}
	}

	for _, module := range parsedModules {
		if module["name"] == moduleName || folderFromModuleName(module["name"]) == moduleName {
			dr := downloadRequest{
				m:    p.ToTypedModule(module),
				env:  p.env,
				done: make(chan bool),
			}

			p.wg.Add(1)
			go func() {
				drs <- dr
				<-dr.done
				p.wg.Done()
			}()
		}
	}

	p.wg.Wait()
	return nil
}
