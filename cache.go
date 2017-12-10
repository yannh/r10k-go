package main

import (
	"fmt"
	"os"
	"sync"
)

type Cache struct {
	sync.Mutex
	folder string
	Locks  map[interface{}]*sync.Mutex
}

func NewCache(cacheFolder string) (*Cache, error) {
	if _, err := os.Stat(cacheFolder); err != nil {
		if err = os.MkdirAll(cacheFolder, 0775); err != nil {
			return &Cache{}, fmt.Errorf("Failed creating cache folder %s: %s", cacheFolder, err.Error())
		}
	}
	return &Cache{folder: cacheFolder, Locks: make(map[interface{}]*sync.Mutex)}, nil
}

func (cache *Cache) LockModule(o interface{}) {
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

func (cache *Cache) UnlockModule(o interface{}) {
	cache.Lock()
	(*cache).Locks[o].Unlock()
	cache.Unlock()
}
