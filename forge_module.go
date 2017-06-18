package main

import "fmt"
import "os"
import "strings"
import "archive/tar"
import "compress/gzip"
import "net/http"
import "io/ioutil"
import "io"
import "encoding/json"

type ForgeModule struct {
	name                string
	version_requirement string
	targetFolder        string
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
	}
}

func (m *ForgeModule) Gunzip(r io.Reader, targetFolder string) error {
	gzf, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(gzf)
	i := 0

	if _, err = os.Stat(m.TargetFolder()); err != nil {
		os.Mkdir(m.TargetFolder(), 0755)
	}

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		name := header.Name

		// The files in the archive are all in a parent folder,
		// we want to extract all files directly to TargetFolder
		namePath := strings.Split(name, "/")
		switch len(namePath) {
		case 0:
			break
		case 1:
			name = "/"
		default:
			name = strings.Join(namePath[1:], "/")
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.Mkdir(m.TargetFolder()+"/"+name, 0755)
			continue
		case tar.TypeReg:
			data := make([]byte, header.Size)
			_, err := tarReader.Read(data)
			if err != nil {
				panic("Error reading file!!! PANIC!!!!!!")
			}

			ioutil.WriteFile(m.TargetFolder()+"/"+name, data, 0755)

		default:
			fmt.Printf("%s : %c %s %s\n",
				"Yikes! Unable to figure out type",
				header.Typeflag,
				"in file",
				name,
			)
		}

		i++
	}

	return nil
}

func (m *ForgeModule) Download() (string, error) {
	forgeUrl := "https://forgeapi.puppetlabs.com:443/"
	ApiVersion := "v3"

	url := forgeUrl + ApiVersion + "/releases?" +
		"module=" + m.Name() +
		"&sort_by=release_date" +
		"&limit=1"

	resp, _ := http.Get(url)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var mr ModuleReleases
	err := json.Unmarshal(body, &mr)
	if err != nil {
		fmt.Println(err)
	}
	if len(mr.Results) > 0 {
		forgeArchive, _ := http.Get(forgeUrl + mr.Results[0].File_uri)
		defer forgeArchive.Body.Close()
		if err = m.Gunzip(forgeArchive.Body, m.TargetFolder()); err != nil {
			fmt.Println("Error processing url: " + err.Error())
			return "", err
		}
	}

	return "", nil
}
