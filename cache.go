package main

import (
	"fmt"
	"os"
	"sync"
)

type cache struct {
	sync.Mutex
	folder string
	Locks  map[interface{}]*sync.Mutex
}

func newCache(cacheFolder string) (*cache, error) {
	if _, err := os.Stat(cacheFolder); err != nil {
		if err = os.MkdirAll(cacheFolder, 0775); err != nil {
			return &cache{}, fmt.Errorf("Failed creating cache folder %s: %s", cacheFolder, err.Error())
		}
	}
	return &cache{folder: cacheFolder, Locks: make(map[interface{}]*sync.Mutex)}, nil
}

func (cache *cache) lockModule(o interface{}) { // FIXME that was a bad improvement, we want to lock on target folder name
	cache.Lock()
	if _, ok := cache.Locks[o]; !ok {
		cache.Locks[o] = new(sync.Mutex)
	}
	l := (*cache).Locks[o]
	// We need to unlock cache before waiting for the
	// module lock, otherwise UnlockModule can not unlock it
	cache.Unlock()

	l.Lock()
}

func (cache *cache) unlockModule(o interface{}) {
	cache.Lock()
	(*cache).Locks[o].Unlock()
	cache.Unlock()
}
