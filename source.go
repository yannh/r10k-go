package main

import (
	"fmt"
	"github.com/yannh/r10k-go/git"
	"io/ioutil"
	"log"
	"path"
)

type source interface {
	deployedEnvironments() []environment
	fetchTo(c *cache) error
}

type gitSource struct {
	Name     string
	location string
	Basedir  string
	prefix   string
	Remote   string
}

func (s *gitSource) deployedEnvironments() []environment {
	folder := path.Join(s.Basedir)

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	envs := make([]environment, 0)

	for _, f := range files {
		envs = append(envs, newEnvironment(*s, f.Name()))
	}

	return envs
}

func (s *gitSource) fetch(c *cache) error {
	if c == nil || c.folder == "" {
		return fmt.Errorf("can not fetch source without cache")
	}
	s.location = path.Join(c.folder, s.Name)

	// Clone if gitSource doesnt exist, fetch otherwise
	if err := git.RevParse(s.location); err != nil {
		if err := git.Clone(s.Remote, git.Ref{}, s.location); err != nil {
			log.Fatalf("%s", err)
		}
	} else {
		git.Fetch(s.location)
	}

	return nil
}
