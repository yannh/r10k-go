package main

import (
	"bytes"
	"io"

	"github.com/yannh/r10k-go/puppetsource"
	"gopkg.in/yaml.v2"
)

type r10kConfigSource struct {
	Basedir string
	Prefix  string
	Remote  string
}

type r10kConfigBase struct {
	Cachedir string
	Sources  map[string]r10kConfigSource
}

type r10kConfig struct {
	Cachedir string
	Sources  []puppetsource.Source
}

func parseR10kConfig(r io.Reader) (*r10kConfig, error) {
	cb := &r10kConfigBase{}
	c := &r10kConfig{}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	err := yaml.Unmarshal(buf.Bytes(), cb)
	if err != nil {
		return nil, err
	}

	c.Cachedir = cb.Cachedir
	c.Sources = make([]puppetsource.Source, 0)
	for sName, s := range cb.Sources {
		c.Sources = append(c.Sources, puppetsource.NewGitSource(sName, "", s.Basedir, s.Prefix, s.Remote))
	}

	return c, nil
}
