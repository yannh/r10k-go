package main

import (
	"github.com/docopt/docopt-go"
)

func cli() map[string]interface{} {

	usage := `r10k-go

Usage:
  r10k-go puppetfile install [--moduledir=<PATH>] [--no-deps] [--puppetfile=<PUPPETFILE>] [--workers=<n>]
  r10k-go puppetfile check [--puppetfile=<PUPPETFILE>]
  r10k-go deploy environment <env>... [--workers=<n>]
  r10k-go deploy module <module>... [--environment=<env>] [--workers=<n>]
  r10k-go version
  r10k-go -h | --help
  r10k-go --version

Options:
  -h --help                   Show this screen.
  --modulesdir=<PATH>        Path to the modules folder
  --no-deps                   Skip downloading modules dependencies
  --puppetFile=<PUPPETFILE>   Path to the modules folder
  --version                   Displays the version.
  --workers=<n>               Number of modules to download in parallel
`

	opts, _ := docopt.Parse(usage, nil, true, "0.1", false)
	return opts
}
