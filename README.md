# r10k-go - Fast &amp; resilient Puppet module deployments  [![Build Status](https://travis-ci.org/yannh/r10k-go.svg?branch=master)](https://travis-ci.org/yannh/r10k-go) [![Go Report card](https://goreportcard.com/badge/github.com/yannh/r10k-go)](https://goreportcard.com/report/github.com/yannh/r10k-go)

WARNING: Under heavy development. Not ready for wide use.

Deployments using r10k/librarian on large Puppetfiles (>100 modules) can end up taking a very long time, and sometimes fail. The goal of this project is to parallelize module download, and retry failed downloads a few times before giving up.

It tries to improve on https://github.com/xorpaul/g10k/ by limitting the number of downloads than can run in parallel, trying to be closer to the behaviour of r10k/librarian, and implementing a retry mechanism.

```
r10k-go

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
  --modulesPath=<PATH>        Path to the modules folder
  --no-deps                   Skip downloading modules dependencies
  --puppetFile=<PUPPETFILE>   Path to the modules folder
  --version                   Displays the version.
  --workers=<n>               Number of modules to download in parallel
```

## What works

The following Puppetfile should download correctly:

```
forge "https://forgeapi.puppetlabs.com"

mod 'puppetlabs-razor'
mod 'puppetlabs-ntp', "0.0.3"

mod 'puppetlabs-apt',
  :git => "git://github.com/puppetlabs/puppetlabs-apt.git"

mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
  :install_path => "test_install_path"

mod 'puppetlabs-apache', '0.6.0',
  :github_tarball => 'puppetlabs/puppetlabs-apache'
```

A cache is maintained in .cache, git worktrees are used to deploy git repository to limit disk usage.

## Not yet implemented

* Complex version requirements for forge modules (can only give a specific version) - although only librarian respects this.
* r10k deploy display
* r10k puppetfile purge
* SVN or local sources
* probably a lot more...

## How to build

Given a correctly setup Go environment, you can go get r10k-go and use the makefile to build it.

```
~/$ go get github.com/yannh/r10k-go
~/$ cd ~/go/src/github.com/yannh/r10k-go/
~/go/src/github.com/yannh/r10k-go$ make
```
