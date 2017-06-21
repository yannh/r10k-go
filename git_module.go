package main

import "os/exec"

type GitModule struct {
	name         string
	repoUrl      string
	installPath  string
	ref          string
	targetFolder string
}

type GitDownloadError struct {
	err error
}

func (gde *GitDownloadError) Error() string {
	return gde.err.Error()
}

func (gde *GitDownloadError) Retryable() bool {
	return true
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

func (m *GitModule) Download() (string, error) {
	var cmd *exec.Cmd

	if m.ref == "" {
		cmd = exec.Command("git", "clone", m.repoUrl, m.targetFolder)
	} else {
		cmd = exec.Command("git", "clone", "--branch", m.ref, m.repoUrl, m.targetFolder)
	}

	if err := cmd.Run(); err != nil {
		return "", &GitDownloadError{err: err}
	}

	return m.targetFolder, nil
}
