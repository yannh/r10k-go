package main

import (
	"fmt"
	"os"
)

type Cache struct {
	folder string
}

func NewCache(cacheFolder string) (Cache, error) {
	if _, err := os.Stat(cacheFolder); err != nil {
		if err = os.MkdirAll(cacheFolder, 0775); err != nil {
			return Cache{}, fmt.Errorf("Failed creating cache folder %s: %s", cacheFolder, err.Error())
		}
	}
	return Cache{folder: cacheFolder}, nil
}

func (cache Cache) Has(module PuppetModule) bool {
	if _, err := os.Stat(cache.folder + module.Hash()); err == nil {
		return true
	}
	return false
}
