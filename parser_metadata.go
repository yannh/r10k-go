package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"path"
	"strings"
	"sync"
)

type Metadata struct {
	Name         string
	Dependencies []struct {
		Name                string
		Version_requirement string
	}
}

type MetadataFile struct {
	io.Reader
}

func (m *MetadataFile) parse(modulesChan chan<- PuppetModule, wg *sync.WaitGroup) (int, error) {
	var meta Metadata

	metadataFile, err := ioutil.ReadAll(m.Reader)
	if err != nil {
		return 0, err
	}

	json.Unmarshal(metadataFile, &meta)
	for _, req := range meta.Dependencies {
		// modulesChan <- p.compute(&ForgeModule{name: req.Name, version_requirement: req.Version_requirement})
		wg.Add(1)
		modulesChan <- m.compute(&ForgeModule{name: req.Name})
	}

	return len(meta.Dependencies), nil
}

func (p *MetadataFile) compute(m PuppetModule) PuppetModule {
	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder(path.Join("modules", folderName))
	return m
}
