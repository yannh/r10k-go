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

type MetadataParser struct {
}

func (p *MetadataParser) parse(r io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup, environment string) error {
	var meta Metadata

	defer wg.Done()
	metadataFile, _ := ioutil.ReadAll(r)

	json.Unmarshal(metadataFile, &meta)
	for _, req := range meta.Dependencies {
		wg.Add(1)
		modulesChan <- p.compute(&ForgeModule{name: req.Name, version_requirement: req.Version_requirement}, environment)
	}

	return nil
}

func (p *MetadataParser) compute(m PuppetModule, environment string) PuppetModule {
	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder(path.Join(environment, "modules", folderName))
	return m
}
