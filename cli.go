package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
)

func cli() map[string]interface{} {

	usage := `r10k-go

Usage:
  r10k-go install [--modulePath=<PATH>] [--puppetfile=<PUPPETFILE>] [--workers=<n>] [--no-deps]
  r10k-go deploy environment <ENV> [--puppetfile=<PUPPETFILE>] [--workers=<WORKERS>]
  r10k-go -h | --help
  r10k-go --version

Options:
  --modulesPath=<PATH>        Path to the modules folder
  --no-deps                   Skip downloading modules dependencies
  --puppetFile=<PUPPETFILE>   Path to the modules folder
  --workers=<n>               Number of modules to download in parallel
  -h --help                   Show this screen.
`

	opts, _ := docopt.Parse(usage, nil, true, "0.1", true)
	fmt.Println(opts)
	return opts
}
