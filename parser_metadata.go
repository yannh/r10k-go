package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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
	*os.File
	wg       *sync.WaitGroup
	filename string
}

func NewMetadataFile(metadataFile string) *MetadataFile {
	// We just ignore if the file doesn't exist'
	f, err := os.Open(metadataFile)
	if err != nil {
		return nil
	}

	return &MetadataFile{File: f, filename: metadataFile, wg: &sync.WaitGroup{}}
}

func (m *MetadataFile) moduleProcessedCallback() { m.wg.Done() }

func (m *MetadataFile) process(modulesChan chan<- PuppetModule, done func()) error {
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

		splitPath := strings.Split(req.Name, "/")
		folderName := splitPath[len(splitPath)-1]
		targetFolder := path.Join("modules", folderName)

		modulesChan <- &ForgeModule{name: req.Name, processed: m.moduleProcessedCallback, targetFolder: targetFolder}
	}

	go func() {
		m.wg.Wait()
		done()
	}()

	return nil
}
