package main

import (
	"io/ioutil"
	"log"
	"path"
)

type environment struct {
	source        source
	branch        string
	modulesFolder string
}

func NewEnvironment(s source, branch string) environment {
	return environment{
		s, branch, "modules",
	}
}

func (e *environment) InstalledModules() []string {
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
