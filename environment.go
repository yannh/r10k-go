package main

import (
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
