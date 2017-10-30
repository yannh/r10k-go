package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
	*os.File
	wg          *sync.WaitGroup
	filename    string
	ModulesPath string
}

func NewMetadataFile(modulesPath string, metadataFile string) *MetadataFile {
	// We just ignore if the file doesn't exist'
	f, err := os.Open(metadataFile)
	if err != nil {
		return nil
	}

	return &MetadataFile{File: f, filename: metadataFile, wg: &sync.WaitGroup{}, ModulesPath: modulesPath}
}

func (m *MetadataFile) moduleProcessedCallback() { m.wg.Done() }
func (m *MetadataFile) Close()                   { m.File.Close() }
func (m *MetadataFile) Filename() string         { return m.filename }

func (m *MetadataFile) Process(modulesChan chan<- PuppetModule, done func()) error {
	var meta Metadata

	metadataFile, err := ioutil.ReadAll(m.File)
	if err != nil {
		done()
		return fmt.Errorf("could not read JSON file %v", err)
	}

	if err = json.Unmarshal(metadataFile, &meta); err != nil {
		done()
		return fmt.Errorf("JSON file malformed: %v", err)
	}

	for _, req := range meta.Dependencies {
		// modulesChan <- p.compute(&ForgeModule{name: req.Name, version_requirement: req.Version_requirement})

		m.wg.Add(1)
		modulesChan <- &ForgeModule{
			name:          req.Name,
			processed:     m.moduleProcessedCallback,
			modulesFolder: path.Join(m.ModulesPath),
		}
	}

	go func() {
		m.wg.Wait()
		done()
	}()

	return nil
}
