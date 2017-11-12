package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/yannh/r10k-go/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

type ForgeModule struct {
	name          string
	version       string
	path          string
	modulesFolder string
	cacheFolder   string
	processed     func()
}

func (m *ForgeModule) Processed() {
	m.processed()
}

func (m *ForgeModule) SetModulesFolder(to string) {
	m.modulesFolder = to
}

func (m *ForgeModule) Folder() string {
	splitPath := strings.FieldsFunc(m.Name(), func(r rune) bool {
		return r == '/' || r == '-'
	})
	folderName := splitPath[len(splitPath)-1]

	return path.Join(m.modulesFolder, folderName)
}

func (m *ForgeModule) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.name))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *ForgeModule) Name() string {
	return m.name
}

type ModuleReleases struct {
	Results []struct {
		File_uri string
		Version  string
	}
}

func (m *ForgeModule) downloadToCache(r io.Reader) error {
	if err := os.MkdirAll(m.cacheFolder, 0755); err != nil {
		return fmt.Errorf("failed creating folder %s: %v", m.cacheFolder, err)
	}

	cacheFile := path.Join(m.cacheFolder, m.version+".tar.gz")
	out, err := os.Create(cacheFile)
	if err != nil {
		return fmt.Errorf("failed creating cache file %s: %v", cacheFile, err)
	}

	defer out.Close()

	_, err = io.Copy(out, r)

	return err
}

func (m *ForgeModule) IsUpToDate() bool {
	_, err := os.Stat(m.Folder())
	if err != nil {
		return false
	} else if m.version == "" {
		// Module is present and no version specified...
		return true
	}

	versionFile := path.Join(m.Folder(), ".version")
	version, err := ioutil.ReadFile(versionFile)
	if err != nil {
		// TODO error handling
		fmt.Println("Error opening version file :" + err.Error())
		return false
	}
	v := string(version)

	return v == m.version
}

func (m *ForgeModule) ModulesFolder() string {
	return m.modulesFolder
}

func (m *ForgeModule) getArchiveURL() (string, error) {
	forgeURL := "https://forgeapi.puppetlabs.com:443/"
	APIVersion := "v3"

	url := forgeURL + APIVersion + "/releases?" +
		"module=" + m.Name() +
		"&sort_by=release_date" +
		"&limit=100"

	resp, err := http.Get(url)
	if err != nil {
		return "", &DownloadError{err, true}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &DownloadError{fmt.Errorf("failed retrieving URL - %s", resp.Status), true}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", &DownloadError{err, true}
	}

	var mr ModuleReleases
	err = json.Unmarshal(body, &mr)

	if err != nil {
		return "", &DownloadError{err, true}
	} else if len(mr.Results) == 0 {
		return "", &DownloadError{fmt.Errorf("Could not find module %s", m.Name()), false}
	}

	// If version is not specified, we pick the latest version
	index := 0
	if m.version != "" {
		versionFound := false
		for i, result := range mr.Results {
			if m.version == result.Version {
				versionFound = true
				index = i
				break
			}
		}
		if !versionFound {
			return "", &DownloadError{fmt.Errorf("Could not find version %s for module %s", m.version, m.Name()), false}
		}
	} else {
		m.version = mr.Results[0].Version
	}

	return mr.Results[index].File_uri, nil
}

func (m *ForgeModule) Download(to string, cache *Cache) *DownloadError {
	var err error
	var url string

	m.cacheFolder = path.Join(cache.Folder, m.Hash())

	forgeURL := "https://forgeapi.puppetlabs.com:443/"
	if url, err = m.getArchiveURL(); err != nil {
		return &DownloadError{err, true}
	}

	if _, err = os.Stat(path.Join(m.cacheFolder, m.version+".tar.gz")); err != nil {
		forgeArchive, err := http.Get(forgeURL + url)
		if err != nil {
			return &DownloadError{fmt.Errorf("could not retrieve %s", forgeURL+url), true}
		}
		defer forgeArchive.Body.Close()

		if err := m.downloadToCache(forgeArchive.Body); err != nil {
			return &DownloadError{fmt.Errorf("could not retrieve %s", forgeURL+url), true}
		}
	}
	r, err := os.Open(path.Join(m.cacheFolder, m.version+".tar.gz"))
	if err != nil {
		return &DownloadError{fmt.Errorf("could not write to %s", path.Join(m.cacheFolder, m.version+".tar.gz")), false}
	}
	defer r.Close()

	if err = gzip.Extract(r, to); err != nil {
		return &DownloadError{err, true}
	}

	versionFile := path.Join(to, ".version")
	f, err := os.OpenFile(versionFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return &DownloadError{fmt.Errorf("could not create file %s", versionFile), false}
	}
	defer f.Close()
	f.WriteString(m.version)

	return nil
}
