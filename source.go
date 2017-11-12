package main

import (
	"io/ioutil"
	"log"
	"path"
)

type source struct {
	Basedir string
	Prefix  string
	Remote  string
}

func (s *source) deployedEnvironments() []string {
	folder := path.Join(s.Basedir)

	files, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal(err)
	}

	envs := make([]string, 5)
	for _, f := range files {
		envs = append(envs, f.Name())
	}

	return envs
}
