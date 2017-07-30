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
  wg *sync.WaitGroup
}

func NewMetadataFile(r io.Reader) *MetadataFile {
	var wg sync.WaitGroup

	return &MetadataFile{
		Reader: r,
		wg: &wg,
	}
}

func (m *MetadataFile) moduleProcessedCallback() {
  m.wg.Done()
}

func (m *MetadataFile) process(modulesChan chan<- PuppetModule, done func()) error {
	var meta Metadata

	metadataFile, err := ioutil.ReadAll(m.Reader)
	if err != nil {
		return err
	}

	json.Unmarshal(metadataFile, &meta)

	for _, req := range meta.Dependencies {
		// modulesChan <- p.compute(&ForgeModule{name: req.Name, version_requirement: req.Version_requirement})
		m.wg.Add(1)
		modulesChan <- m.compute(&ForgeModule{name: req.Name, processed: m.moduleProcessedCallback})
	}

	go func() {
		m.wg.Wait()
		done()
	}()

	return nil
}

func (p *MetadataFile) compute(m PuppetModule) PuppetModule {
	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder(path.Join("modules", folderName))
	return m
}
