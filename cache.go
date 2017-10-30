package main

import (
	"fmt"
	"os"
	"sync"
)

type Cache struct {
	sync.Mutex
	Folder string
	Locks  map[string]*sync.Mutex
}

func NewCache(cacheFolder string) (*Cache, error) {
	if _, err := os.Stat(cacheFolder); err != nil {
		if err = os.MkdirAll(cacheFolder, 0775); err != nil {
			return &Cache{}, fmt.Errorf("Failed creating cache folder %s: %s", cacheFolder, err.Error())
		}
	}
	return &Cache{Folder: cacheFolder, Locks: make(map[string]*sync.Mutex)}, nil
}

func (cache *Cache) Has(module PuppetModule) bool {
	if _, err := os.Stat(cache.Folder + module.Hash()); err == nil {
		return true
	}
	return false
}

func (cache *Cache) LockModule(o string) {
	cache.Lock()
	defer cache.Unlock()

	if _, ok := cache.Locks[o]; !ok {
		cache.Locks[o] = new(sync.Mutex)
	}
	(*cache).Locks[o].Lock()
}

func (cache *Cache) UnlockModule(o string) {
	(*cache).Locks[o].Unlock()
}
