package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

type dependency struct {
	name               string
	versionRequirement string
}

type Metadata struct {
	name         string
	dependencies []dependency
}

type MetadataFile struct {
	*os.File
	filename string
	env      environment
}

func NewMetadataFile(metadataFile string, env environment) *MetadataFile {
	// We just ignore if the file doesn't exist'
	f, err := os.Open(metadataFile)
	if err != nil {
		return nil
	}

	return &MetadataFile{File: f, filename: metadataFile, env: env}
}

func (m *MetadataFile) Close() { m.File.Close() }

func (m *MetadataFile) Process(drs chan<- downloadRequest) error {
	var meta Metadata
	var wg sync.WaitGroup

	metadataFile, err := ioutil.ReadAll(m.File)
	if err != nil {
		return fmt.Errorf("could not read JSON file %v", err)
	}

	if err = json.Unmarshal(metadataFile, &meta); err != nil {
		return fmt.Errorf("JSON file malformed: %v", err)
	}

	for _, req := range meta.dependencies {
		wg.Add(1)
		done := make(chan bool)

		go func(req dependency) {
			drs <- downloadRequest{
				m: &ForgeModule{
					name: req.name,
				},
				env:  m.env,
				done: done,
			}
			<-done
			wg.Done()
		}(req)
	}

	wg.Wait()
	return nil
}
