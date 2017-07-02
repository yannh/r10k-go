# r10k-go - Fast &amp; resilient Puppet module deployments  [![Build Status](https://travis-ci.org/yannh/r10k-go.svg?branch=master)](https://travis-ci.org/yannh/r10k-go)

WARNING: Under heavy development. Not ready for wide use.

Deployments using r10k/librarian on large Puppetfiles (>100 modules) can end up taking a very long time, and sometimes fail. The goal of this project is to parallelize module download, and retry failed downloads a few times before giving up.

It tries to improve on https://github.com/xorpaul/g10k/ by limitting the number of downloads than can run in parallel, trying to be closer to the behaviour of r10k/librarian, and implementing a retry mechanism.

```
Usage:
    librarian-go install [--puppetfile=<PUPPETFILE>] [--workers=<workers>]
    librarian-go deploy environment <ENV> [--puppetfile=<PUPPETFILE>] [--workers=<workers>]
```

# Currently implemented

* Caches GIt repositories, and uses Git worktrees to make them available to different environments
* Will download the right tag/ref/branch of GIT modules if specified
* Will download the right version of Forge modules

# Not yet implemented

* Complex version requirements for forge modules (can only give a specific version)
* Support for r10k configuration files. Complex environment management is being actively worked on.
