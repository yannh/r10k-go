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
	wg       *sync.WaitGroup
	filename string
	env      environment
}

func NewMetadataFile(metadataFile string, env environment) *MetadataFile {
	// We just ignore if the file doesn't exist'
	f, err := os.Open(metadataFile)
	if err != nil {
		return nil
	}

	return &MetadataFile{File: f, filename: metadataFile, wg: &sync.WaitGroup{}, env: env}
}

func (m *MetadataFile) moduleProcessedCallback() { m.wg.Done() }
func (m *MetadataFile) Close()                   { m.File.Close() }
func (m *MetadataFile) Filename() string         { return m.filename }

func (m *MetadataFile) Process(drs chan<- downloadRequest) error {
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
		done := make(chan bool)

		go func() {
			drs <- downloadRequest{
				m: &ForgeModule{
					name: req.Name,
				},
				env:  m.env,
				done: done,
			}
			<-done
			m.wg.Done()
		}()
	}

	m.wg.Wait()
	return nil
}
