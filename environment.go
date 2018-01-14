package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"path"

	"github.com/yannh/r10k-go/git"
	"github.com/yannh/r10k-go/puppetsource"
)

type environment struct {
	source        puppetsource.Source
	branch        string
	modulesFolder string
}

func newEnvironment(s puppetsource.Source, branch string) environment {
	return environment{
		s, branch, "modules",
	}
}

func getEnvironments(envNames []string, sources []puppetsource.Source) []environment {
	envs := make([]environment, 0)

	for _, envName := range envNames {
		// Find in which source the environment is
		// TODO: make deterministic
		found := false
		for _, source := range sources {
			if git.RepoHasRemoteBranch(source.Remote(), envName) {
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
	folder := path.Join(e.source.Basedir(), e.branch, e.modulesFolder)

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
	s := env.source.(*puppetsource.GitSource) // FIXME should use interface instead of checkout/clone
	if err := s.Fetch(cache.folder); err != nil {
		return err
	}

	if err := git.Checkout(s.Location(), git.NewRef(git.TypeBranch, env.branch)); err != nil {
		return err
	}
	if err := git.Clone(s.Location(), path.Join(s.Basedir(), env.branch)); err != nil {
		return err
	}

	return nil
}

func DeployedEnvironments(s puppetsource.Source) []environment {
	folder := path.Join(s.Basedir())

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	envs := make([]environment, 0)

	for _, f := range files {
		envs = append(envs, newEnvironment(s, f.Name()))
	}

	return envs
}
