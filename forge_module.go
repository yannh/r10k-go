package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
)

type ForgeModule struct {
	name    string
	version string
	// version_requirement string  ignored for now
	targetFolder string
	cacheFolder  string
	processed    func()
}

func (m *ForgeModule) Processed() {
	m.processed()
}

func (m *ForgeModule) SetCacheFolder(folder string) {
	m.cacheFolder = folder
}

func (m *ForgeModule) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.name))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *ForgeModule) Name() string {
	return m.name
}

func (m *ForgeModule) SetTargetFolder(folder string) {
	m.targetFolder = folder
}

func (m *ForgeModule) TargetFolder() string {
	return m.targetFolder
}

type ModuleReleases struct {
	Results []struct {
		File_uri string
		Version  string
	}
}

func (m *ForgeModule) downloadToCache(r io.Reader) error {
	os.MkdirAll(path.Join(m.cacheFolder), 0755)

	out, err := os.Create(path.Join(m.cacheFolder, m.version+".tar.gz"))
	defer out.Close()

	_, err = io.Copy(out, r)

	return err
}

func (m *ForgeModule) IsUpToDate() bool {
	_, err := os.Stat(m.TargetFolder())
	if err != nil {
		return false
	} else if m.version == "" {
		// Module is present and no version specified...
		return true
	}

	versionFile := path.Join(m.TargetFolder(), ".version")
	version, err := ioutil.ReadFile(versionFile)
	if err != nil {
		// TODO error handling
		fmt.Println("Error opening version file :" + err.Error())
		return false
	}
	v := string(version)

	return v == m.version
}

func (m *ForgeModule) getDownloadUrl() (string, error) {
	forgeUrl := "https://forgeapi.puppetlabs.com:443/"
	ApiVersion := "v3"

	url := forgeUrl + ApiVersion + "/releases?" +
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

func (m *ForgeModule) Download() DownloadError {
	var err error
	var url string

	forgeUrl := "https://forgeapi.puppetlabs.com:443/"
	if url, err = m.getDownloadUrl(); err != nil {
		return DownloadError{err, true}
	}

	if _, err = os.Stat(path.Join(m.cacheFolder, m.version+".tar.gz")); err != nil {
		forgeArchive, err := http.Get(forgeUrl + url)
		if err != nil {
			return DownloadError{fmt.Errorf("Failed retrieving %s", forgeUrl+url), true}
		}
		defer forgeArchive.Body.Close()

		m.downloadToCache(forgeArchive.Body)
	}
	r, _ := os.Open(path.Join(m.cacheFolder, m.version+".tar.gz"))
	defer r.Close()

	if err = extract(r, m.targetFolder); err != nil {
		return DownloadError{err, true}
	}

	versionFile := path.Join(m.targetFolder, ".version")
	r, err = os.OpenFile(versionFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return DownloadError{fmt.Errorf("Failed creating file %s", versionFile), false}
	}
	defer r.Close()
	r.WriteString(m.version)

	return DownloadError{nil, false}
}
