package puppetmodule

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

type GithubTarballModule struct {
	Name        string
	RepoName    string
	Version     string
	InstallPath string
}

type ghModuleRelease []struct {
	Name       string
	TarballURL string `json:"tarball_url"`
}

func (m *GithubTarballModule) GetName() string {
	return m.Name
}

func (m *GithubTarballModule) GetInstallPath() string {
	return m.InstallPath
}

func (m *GithubTarballModule) hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.Name))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *GithubTarballModule) IsUpToDate(folder string) bool {
	_, err := os.Stat(folder)
	if err != nil {
		return false
	} else if m.Version == "" {
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

	return v == m.Version
}

func (m *GithubTarballModule) downloadToCache(r io.Reader, cacheFolder string) error {
	if err := os.MkdirAll(path.Join(cacheFolder), 0755); err != nil {
		return err
	}

	out, err := os.Create(path.Join(cacheFolder, m.Version+".tar.gz"))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, r)

	return err
}

func (m *GithubTarballModule) downloadURL() (string, error) {
	ghAPIRoot := "https://api.github.com"

	url := ghAPIRoot + "/repos/" + m.RepoName + "/tags"

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

	var gr ghModuleRelease
	if err = json.Unmarshal(body, &gr); err != nil {
		return "", err
	}

	index := 0
	if m.Version != "" {
		versionFound := false
		for i, result := range gr {
			if m.Version == result.Name {
				versionFound = true
				index = i
				break
			}
		}
		if !versionFound {
			return "", &DownloadError{fmt.Errorf("Could not find version %s for module %s", m.Version, m.GetName()), false}
		}
	} else {
		m.Version = gr[0].Name
	}

	return gr[index].TarballURL, nil
}

func (m *GithubTarballModule) Download(to string, cache string) *DownloadError {
	var err error
	var url string

	cacheFolder := path.Join(cache, m.hash())

	if url, err = m.downloadURL(); err != nil {
		return &DownloadError{err, true}
	}

	if _, err = os.Stat(path.Join(cacheFolder, m.Version+".tar.gz")); err != nil {
		forgeArchive, err := http.Get(url)
		if err != nil {
			return &DownloadError{fmt.Errorf("Failed retrieving %s", url), true}
		}
		defer forgeArchive.Body.Close()

		m.downloadToCache(forgeArchive.Body, cacheFolder)
	}

	r, err := os.Open(path.Join(cacheFolder, m.Version+".tar.gz"))
	if err != nil {
		return &DownloadError{err, false}
	}

	defer r.Close()

	if err = gzip.Extract(r, to); err != nil {
		return &DownloadError{err, false}
	}

	versionFile := path.Join(to, ".version")
	r, err = os.OpenFile(versionFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return &DownloadError{fmt.Errorf("Failed creating file %s", versionFile), false}
	}
	defer r.Close()
	r.WriteString(m.Version)

	return nil
}
