package main

import "io"
import "encoding/json"
import "io/ioutil"
import "sync"
import "strings"

type Metadata struct {
	Name         string
	Dependencies []struct {
		Name                string
		Version_requirement string
	}
}

type MetadataParser struct {
}

func (p *MetadataParser) parse(r io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup) error {
	var meta Metadata

	defer wg.Done()
	metadataFile, _ := ioutil.ReadAll(r)

	json.Unmarshal(metadataFile, &meta)
	for _, req := range meta.Dependencies {
		wg.Add(1)
		modulesChan <- p.compute(&ForgeModule{name: req.Name, version_requirement: req.Version_requirement})
	}

	return nil
}

func (p *MetadataParser) compute(m PuppetModule) PuppetModule {
	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder("./modules/" + folderName)
	return m
}
