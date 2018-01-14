package main

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"io"
)

type r10kConfig struct {
	Cachedir string
	Sources  map[string]gitSource
}

func parseR10kConfig(r io.Reader) (*r10kConfig, error) {
	c := &r10kConfig{}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	err := yaml.Unmarshal(buf.Bytes(), c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
