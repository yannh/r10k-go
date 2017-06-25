package main

import (
	"crypto/sha1"
	"encoding/base64"
	"os"
	"os/exec"
	"path"
)

type GitModule struct {
	name         string
	repoUrl      string
	installPath  string // TODO: Implement
	ref          string
	targetFolder string
	cacheFolder  string
}

type GitDownloadError struct {
	err       error
	retryable bool
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

func (m *GitModule) SetCacheFolder(folder string) {
	m.cacheFolder = folder
}

func (m *GitModule) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.repoUrl))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *GitModule) SetTargetFolder(folder string) {
	m.targetFolder = folder
}

func (m *GitModule) TargetFolder() string {
	return m.targetFolder
}

func (m *GitModule) downloadToCache() error {
	var cmd *exec.Cmd

	if _, err := os.Stat(m.cacheFolder); err != nil {
		cmd = exec.Command("git", "clone", m.repoUrl, m.cacheFolder)
		if err := cmd.Run(); err != nil {
			return &GitDownloadError{err: err, retryable: true}
		}
	}

	return nil
}

func (m *GitModule) Download() error {
	var cmd *exec.Cmd
	var err error

	if _, err := os.Stat(m.cacheFolder); err != nil {
		if err = m.downloadToCache(); err != nil {
			return err
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &GitDownloadError{err: err, retryable: false}
	}

	if m.ref == "" {
		cmd = exec.Command("git", "worktree", "add", "--detach", "-f", path.Join(cwd, m.targetFolder))
	} else {
		cmd = exec.Command("git", "worktree", "add", "--detach", "-f", path.Join(cwd, m.targetFolder), m.ref)
	}
	cmd.Dir = m.cacheFolder

	if err = cmd.Run(); err != nil {
		return &GitDownloadError{err: err, retryable: true}
	}

	return nil
}
