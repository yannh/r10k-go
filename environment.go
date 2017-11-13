package main

import (
	"io/ioutil"
	"log"
	"path"
)

type environment struct {
	source
	branch        string
	modulesFolder string
}

func (e *environment) InstalledModules() []string {
	folder := path.Join(e.basedir, e.branch, e.modulesFolder)

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
