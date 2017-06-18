package main

import "io"
import "encoding/json"
import "io/ioutil"
import "sync"

type Metadata struct {
	Name         string
	Dependencies []struct {
		Name                string
		Version_requirement string
	}
}

type MetadataParser struct {
}

func (p MetadataParser) parse(r io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup) error {
	var m Metadata

	defer wg.Done()
	metadataFile, _ := ioutil.ReadAll(r)

	json.Unmarshal(metadataFile, &m)
	for _, req := range m.Dependencies {
		wg.Add(1)
		modulesChan <- &ForgeModule{name: req.Name, version_requirement: req.Version_requirement}
	}

	return nil
}
