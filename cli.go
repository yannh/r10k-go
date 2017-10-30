package main

import (
	"github.com/docopt/docopt-go"
)

func cli() map[string]interface{} {

	usage := `r10k-go

Usage:
  r10k-go install [--modulePath=<PATH>] [--no-deps] [--puppetfile=<PUPPETFILE>] [--workers=<n>]
  r10k-go deploy environment <env>... [--workers=<n>]
  r10k-go -h | --help
  r10k-go --version

Options:
  -h --help                   Show this screen.
  --modulesPath=<PATH>        Path to the modules folder
  --no-deps                   Skip downloading modules dependencies
  --puppetFile=<PUPPETFILE>   Path to the modules folder
  --version                   Displays the version.
  --workers=<n>               Number of modules to download in parallel
`

	opts, _ := docopt.Parse(usage, nil, true, "0.1", false)
	return opts
}
