package main

import "github.com/docopt/docopt-go"

func cli() map[string]interface{} {

	usage := `librarian-go

  Usage:
    librarian-go install [--path=<PATH>] [--puppetfile=<PUPPETFILE>] [--workers=<workers>]
    librarian-go git_status
    librarian-go update

  Options:
    -h --help                Show this screen.`

	opts, _ := docopt.Parse(usage, nil, true, "r10k-go", false)

	return opts
}
