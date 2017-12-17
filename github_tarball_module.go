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
)

type githubTarballModule struct {
	name        string
	repoName    string
	version     string
	installPath string
}

type ghModuleRelease []struct {
	Name       string
	TarballURL string `json:"tarball_url"`
}

func (m *githubTarballModule) getName() string {
	return m.name
}

func (m *githubTarballModule) getInstallPath() string {
	return m.installPath
}

func (m *githubTarballModule) hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.name))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *githubTarballModule) isUpToDate(folder string) bool {
	_, err := os.Stat(folder)
	if err != nil {
		return false
	} else if m.version == "" {
		// Module is present and no version specified...
		return true
	}

	versionFile := path.Join(folder, ".version")
	version, err := ioutil.ReadFile(versionFile)
	if err != nil {
		// TODO error handling
		fmt.Println("Error opening version file :" + err.Error())
		return false
	}
	v := string(version)

	return v == m.version
}

func (m *githubTarballModule) downloadToCache(r io.Reader, cacheFolder string) error {
	if err := os.MkdirAll(path.Join(cacheFolder), 0755); err != nil {
		return err
	}

	out, err := os.Create(path.Join(cacheFolder, m.version+".tar.gz"))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)

	return err
}

func (m *githubTarballModule) downloadURL() (string, error) {
	ghAPIRoot := "https://api.github.com"

	url := ghAPIRoot + "/repos/" + m.repoName + "/tags"

	resp, err := http.Get(url)
	if err != nil {
		return "", &downloadError{err, true}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &downloadError{fmt.Errorf("failed retrieving URL - %s", resp.Status), true}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", &downloadError{err, true}
	}

	var gr ghModuleRelease
	if err = json.Unmarshal(body, &gr); err != nil {
		return "", err
	}

	index := 0
	if m.version != "" {
		versionFound := false
		for i, result := range gr {
			if m.version == result.Name {
				versionFound = true
				index = i
				break
			}
		}
		if !versionFound {
			return "", &downloadError{fmt.Errorf("Could not find version %s for module %s", m.version, m.getName()), false}
		}
	} else {
		m.version = gr[0].Name
	}

	return gr[index].TarballURL, nil
}

func (m *githubTarballModule) download(to string, cache *cache) *downloadError {
	var err error
	var url string

	cacheFolder := path.Join(cache.folder, m.hash())

	if url, err = m.downloadURL(); err != nil {
		return &downloadError{err, true}
	}

	if _, err = os.Stat(path.Join(cacheFolder, m.version+".tar.gz")); err != nil {
		forgeArchive, err := http.Get(url)
		if err != nil {
			return &downloadError{fmt.Errorf("Failed retrieving %s", url), true}
		}
		defer forgeArchive.Body.Close()

		m.downloadToCache(forgeArchive.Body, cacheFolder)
	}

	r, err := os.Open(path.Join(cacheFolder, m.version+".tar.gz"))
	if err != nil {
		return &downloadError{err, false}
	}

	defer r.Close()

	if err = gzip.Extract(r, to); err != nil {
		return &downloadError{err, false}
	}

	versionFile := path.Join(to, ".version")
	r, err = os.OpenFile(versionFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return &downloadError{fmt.Errorf("Failed creating file %s", versionFile), false}
	}
	defer r.Close()
	r.WriteString(m.version)

	return nil
}
