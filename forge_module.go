package main

import "fmt"
import "net/http"
import "io/ioutil"
import "encoding/json"

type ForgeModule struct {
	name        string
  version_requirement  string
  targetFolder string
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

func (m *ForgeModule) Download() string{
  forgeUrl := "https://forgeapi.puppetlabs.com:443/v3/"

  url := forgeUrl + "releases?"+
           "module=" + m.Name() +
           "&sort_by=release_date" +
           "&limit=1"

  resp, _ := http.Get(url)
  defer resp.Body.Close()
  body, _ := ioutil.ReadAll(resp.Body)

  var mr ModuleReleases
  err := json.Unmarshal(body, &mr)
  if (err != nil) {
    fmt.Println(err)
  }
  if (len(mr.Results) > 0) {
    fmt.Println("Need to download "+mr.Results[0].File_uri)
  }

  return ""
}

