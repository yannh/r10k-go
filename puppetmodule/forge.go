package puppetmodule

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

	"github.com/yannh/r10k-go/gzip"
)

type ForgeModule struct {
	name    string
	version string
}

func NewForgeModule(name, version string) *ForgeModule {
	return &ForgeModule{
		name:    name,
		version: version,
	}
}

func (m *ForgeModule) GetInstallPath() string {
	return ""
}

func (m *ForgeModule) hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.name))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *ForgeModule) Name() string {
	return m.name
}

type moduleReleases struct {
	Results []struct {
		FileURI string `json:"file_uri"`
		Version string
	}
}

func (m *ForgeModule) downloadToCache(r io.Reader, cacheFolder string) error {
	if err := os.MkdirAll(cacheFolder, 0755); err != nil {
		return fmt.Errorf("failed creating folder %s: %v", cacheFolder, err)
	}

	cacheFile := path.Join(cacheFolder, m.version+".tar.gz")
	out, err := os.Create(cacheFile)
	if err != nil {
		return fmt.Errorf("failed creating cache file %s: %v", cacheFile, err)
	}

	defer out.Close()

	_, err = io.Copy(out, r)

	return err
}

func (m *ForgeModule) IsUpToDate(folder string) bool {
	_, err := os.Stat(folder)
	if err != nil {
		return false
	} else if m.version == "" {
		// Module is present and no Version specified...
		return true
	}

	versionFile := path.Join(folder, ".Version")
	version, err := ioutil.ReadFile(versionFile)
	if err != nil {
		// TODO error handling
		fmt.Println("Error opening Version file :" + err.Error())
		return false
	}
	v := string(version)

	return v == m.version
}

func (m *ForgeModule) getArchiveURL() (string, error) {
	forgeURL := "https://forgeapi.puppetlabs.com:443/"
	APIVersion := "v3"

	url := forgeURL + APIVersion + "/releases?" +
		"module=" + m.GetName() +
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

	var mr moduleReleases
	err = json.Unmarshal(body, &mr)

	if err != nil {
		return "", &DownloadError{err, true}
	} else if len(mr.Results) == 0 {
		return "", &DownloadError{fmt.Errorf("Could not find module %s", m.GetName()), false}
	}

	// If Version is not specified, we pick the latest Version
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
			return "", &DownloadError{fmt.Errorf("Could not find Version %s for module %s", m.version, m.GetName()), false}
		}
	} else {
		m.version = mr.Results[0].Version
	}

	return mr.Results[index].FileURI, nil
}

func (m *ForgeModule) Download(to string, cache string) *DownloadError {
	var err error
	var url string

	cacheFolder := path.Join(cache, m.hash())

	forgeURL := "https://forgeapi.puppetlabs.com:443/"
	if url, err = m.getArchiveURL(); err != nil {
		return &DownloadError{err, true}
	}

	if _, err = os.Stat(path.Join(cacheFolder, m.version+".tar.gz")); err != nil {
		forgeArchive, err := http.Get(forgeURL + url)
		if err != nil {
			return &DownloadError{fmt.Errorf("could not retrieve %s", forgeURL+url), true}
		}
		defer forgeArchive.Body.Close()

		if err := m.downloadToCache(forgeArchive.Body, cacheFolder); err != nil {
			return &DownloadError{fmt.Errorf("could not retrieve %s", forgeURL+url), true}
		}
	}
	r, err := os.Open(path.Join(cacheFolder, m.version+".tar.gz"))
	if err != nil {
		return &DownloadError{fmt.Errorf("could not write to %s", path.Join(cacheFolder, m.version+".tar.gz")), false}
	}
	defer r.Close()

	if err = gzip.Extract(r, to); err != nil {
		return &DownloadError{err, true}
	}

	versionFile := path.Join(to, ".Version")
	f, err := os.OpenFile(versionFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return &DownloadError{fmt.Errorf("could not create file %s", versionFile), false}
	}
	defer f.Close()
	f.WriteString(m.version)

	return nil
}
