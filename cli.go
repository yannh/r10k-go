package main

import "github.com/docopt/docopt-go"

func cli() map[string]interface{} {

	usage := `librarian-go

  Usage:
    librarian-go install [--puppetfile=<PUPPETFILE>] [--workers=<workers>]
    librarian-go deploy environment <ENV> [--puppetfile=<PUPPETFILE>] [--workers=<workers>]

  Options:
    --modulesPath=<PATH>     Path to the modules folder
    -h --help                Show this screen.`

	opts, _ := docopt.Parse(usage, nil, true, "r10k-go", false)

	return opts
}
