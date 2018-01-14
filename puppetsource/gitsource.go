package puppetsource

import (
	"fmt"
	"log"
	"path"

	"github.com/yannh/r10k-go/git"
)

type GitSource struct {
	name     string
	location string
	basedir  string
	prefix   string
	remote   string
}

func NewGitSource(name, location, basedir, prefix, remote string) *GitSource {
	return &GitSource{
		name:     name,
		location: location,
		basedir:  basedir,
		prefix:   prefix,
		remote:   remote,
	}
}

func (s *GitSource) Name() string     { return s.name }
func (s *GitSource) Remote() string   { return s.remote }
func (s *GitSource) Location() string { return s.location }
func (s *GitSource) Basedir() string  { return s.basedir }

func (s *GitSource) Fetch(cache string) error {
	if cache == "" {
		return fmt.Errorf("can not fetch source without cache")
	}
	s.location = path.Join(cache, s.Name())

	// Clone if gitSource doesnt exist, fetch otherwise
	if err := git.RevParse(s.location); err != nil {
		if err := git.Clone(s.Remote(), s.location); err != nil {
			log.Fatalf("%s", err)
		}
	} else {
		git.Fetch(s.location)
	}
	return nil
}
