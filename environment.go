package main

import (
	"fmt"
	"github.com/yannh/r10k-go/git"
	"io/ioutil"
	"log"
	"path"
)

type environment struct {
	source        gitSource
	branch        string
	modulesFolder string
}

func newEnvironment(s gitSource, branch string) environment {
	return environment{
		s, branch, "modules",
	}
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

func (e *environment) installedModules() []string {
	folder := path.Join(e.source.Basedir, e.branch, e.modulesFolder)

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	modules := make([]string, 5)
	for _, f := range files {
		modules = append(modules, f.Name())
	}

	return modules
}

func (env *environment) fetch(cache *cache) error {
	if err := env.source.fetch(cache); err != nil {
		return err
	}

	if err := git.Checkout(env.source.location, git.NewRef(git.TypeBranch, env.branch)); err != nil {
		return err
	}
	if err := git.Clone(env.source.location, path.Join(env.source.Basedir, env.branch)); err != nil {
		return err
	}

	return nil
}
