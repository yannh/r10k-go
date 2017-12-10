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

type metadata struct {
	name         string
	dependencies []dependency
}

type metadataFile struct {
	*os.File
	filename string
	env      environment
}

func NewMetadataFile(mf string, env environment) *metadataFile {
	// We just ignore if the file doesn't exist'
	f, err := os.Open(mf)
	if err != nil {
		return nil
	}

	return &metadataFile{File: f, filename: mf, env: env}
}

func (m *metadataFile) Close() { m.File.Close() }

func (m *metadataFile) Process(drs chan<- downloadRequest) error {
	var meta metadata
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
				m: &forgeModule{
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
