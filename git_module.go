package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type GitModule struct {
	name         string
	repoUrl      string
	installPath  string // TODO: Implement
	targetFolder string
	cacheFolder  string
	want         struct {
		ref    string
		tag    string
		branch string
	}
}

func (m *GitModule) Name() string {
	return m.name
}

func (m *GitModule) IsUpToDate() bool {
	if _, err := os.Stat(m.TargetFolder()); err != nil {
		return false
	}

	// If nothing specified, anything goes
	if m.want.ref == "" && m.want.branch == "" && m.want.tag == "" {
		return true
	}

	if m.want.ref != "" {
		commit, _ := m.currentCommit()
		return m.want.ref == commit
	}

	cmd := exec.Command("git", "show", "-s", "--pretty=%d", "HEAD")
	cmd.Dir = m.TargetFolder()
	output, _ := cmd.Output()

	if m.want.branch != "" {
		return strings.Contains(string(output), "origin/"+m.want.branch)
	}

	if m.want.tag != "" {
		return strings.Contains(string(output), "tag: "+m.want.tag)
	}

	return false
}

func (m *GitModule) gitCommand() []string {
	cwd, _ := os.Getwd()

	cmd := "git worktree add --detach -f " + path.Join(cwd, m.targetFolder)
	if m.want.ref != "" {
		cmd += " " + m.want.ref
	}

	if m.want.branch != "" {
		cmd += " origin/" + m.want.branch
	}

	return strings.Split(cmd, " ")
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

func (m *GitModule) currentCommit() (string, error) {
	var gitFile, headFile *os.File
	var err error
	worktreeFolder := ""

	if gitFile, err = os.Open(path.Join(m.TargetFolder(), ".git")); err != nil {
		return "", fmt.Errorf("Error getting current commit for %s", m.Name())
	}

	defer gitFile.Close()

	scanner := bufio.NewScanner(gitFile)

	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "gitdir:") {
			worktreeFolder = strings.Trim(strings.Split(t, ":")[1], " ")
		}
	}

	if headFile, err = os.Open(path.Join(worktreeFolder, "HEAD")); err != nil {
		return "", fmt.Errorf("Error getting current commit for %s", m.Name())
	}
	defer headFile.Close()

	scanner = bufio.NewScanner(headFile)
	scanner.Scan()
	version := scanner.Text()

	return version, nil
}

func (m *GitModule) downloadToCache() error {
	var cmd *exec.Cmd

	if _, err := os.Stat(m.cacheFolder); err != nil {
		cmd = exec.Command("git", "clone", m.repoUrl, m.cacheFolder)
		if err := cmd.Run(); err != nil {
			return &DownloadError{error: err, retryable: true}
		}
	}

	return nil
}

func (m *GitModule) Download() DownloadError {
	var cmd *exec.Cmd
	var err error

	if _, err := os.Stat(m.cacheFolder); err != nil {
		if err = m.downloadToCache(); err != nil {
			return DownloadError{error: err, retryable: true}
		}
	}

	gc := m.gitCommand()
	cmd = exec.Command(gc[0], gc[1:]...)
	cmd.Dir = m.cacheFolder

	if err = cmd.Run(); err != nil {
		return DownloadError{error: err, retryable: true}
	}

	return DownloadError{error: nil, retryable: false}
}
