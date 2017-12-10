package main

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"io"
	"log"
	"os"
)

type R10kConfig struct {
	Cachedir string
	Sources  map[string]source
}

func newR10kConfig(filename string) (*R10kConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("could not open %s: %v", filename, err)
	}

	return parseR10kConfig(f)
}

func parseR10kConfig(r io.Reader) (*R10kConfig, error) {
	c := &R10kConfig{}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	err := yaml.Unmarshal(buf.Bytes(), c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
