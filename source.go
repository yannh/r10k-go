package main

import (
	"io/ioutil"
	"log"
	"path"
)

type source struct {
	Basedir string
	prefix  string
	Remote  string
}

func (s *source) deployedEnvironments() []environment {
	folder := path.Join(s.Basedir)

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	envs := make([]environment, 0)

	for _, f := range files {
		envs = append(envs, NewEnvironment(*s, f.Name()))
	}

	return envs
}
