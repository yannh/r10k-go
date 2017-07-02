package main

import (
	"bytes"
	"gopkg.in/yaml.v2"
	"io"
)

type source struct {
	Basedir string
	Prefix  string
	Remote  string
}

type config struct {
	Cachedir string
	Sources  map[string]source
}

func parseConfig(file io.Reader) (config, error) {
	t := config{}

	buf := new(bytes.Buffer)
	buf.ReadFrom(file)

	yaml.Unmarshal(buf.Bytes(), &t)

	return t, nil
}
