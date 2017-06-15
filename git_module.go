package main

import "os/exec"
import "fmt"

type GitModule struct {
	name        string
	repoUrl     string
	installPath string
	ref         string
  targetFolder string
}

func (m *GitModule) Name() string {
  return m.name
}

func (m *GitModule) SetTargetFolder(folder string) {
  m.targetFolder = folder
}

func (m *GitModule) TargetFolder() string {
  return m.targetFolder
}

func (m *GitModule) Download() string{
  var cmd *exec.Cmd

  if (m.ref == "") {
    cmd = exec.Command("git", "clone", m.repoUrl, m.targetFolder)
  } else {
    cmd = exec.Command("git", "clone", "--branch", m.ref, m.repoUrl, m.targetFolder)
  }
  cmd.Run()
  cmd.Wait()
  fmt.Println("Downloaded "+m.repoUrl + " to "+m.targetFolder)

  return m.targetFolder
}

