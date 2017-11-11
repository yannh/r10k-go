package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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

func NewMetadataFile(metadataFile string, modulesPath string) *MetadataFile {
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

func (m *MetadataFile) Process(modulesChan chan<- PuppetModule) error {
	var meta Metadata

	metadataFile, err := ioutil.ReadAll(m.File)
	if err != nil {
		return fmt.Errorf("could not read JSON file %v", err)
	}

	if err = json.Unmarshal(metadataFile, &meta); err != nil {
		return fmt.Errorf("JSON file malformed: %v", err)
	}

	for _, req := range meta.Dependencies {
		m.wg.Add(1)
		modulesChan <- &ForgeModule{
			name:          req.Name,
			processed:     m.moduleProcessedCallback,
			modulesFolder: m.ModulesPath,
		}
	}

	m.wg.Wait()
	return nil
}
