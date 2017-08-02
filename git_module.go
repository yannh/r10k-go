package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

type GitModule struct {
	name        string
	repoURL     string
	envRoot     string
	installPath string
	cacheFolder string
	processed   func()
	want        struct {
		ref    string
		tag    string
		branch string
	}
}

func (m *GitModule) Name() string { return m.name }
func (m *GitModule) Processed()   { m.processed() }

func (m *GitModule) IsUpToDate() bool {
	if _, err := os.Stat(m.TargetFolder()); err != nil {
		return false
	}

	// folder exists, but no version specified, anything goes
	if m.want.ref == "" && m.want.branch == "" && m.want.tag == "" {
		return true
	}

	if m.want.ref != "" {
		commit, err := m.currentCommit()
		if err != nil {
			return false
		}
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

func (m *GitModule) gitCommand(to string) []string {
	cmd := "git worktree add --detach -f " + path.Join("..", "..", to)
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

func (m *GitModule) SetEnvRoot(s string) {
	m.envRoot = s
}

func (m *GitModule) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.repoURL))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *GitModule) TargetFolder() string {
	if m.envRoot == "" {
		log.Fatal("Environment root not defined")
	}

	splitPath := strings.FieldsFunc(m.name, func(r rune) bool {
		return r == '/' || r == '-'
	})
	folderName := splitPath[len(splitPath)-1]
	if folderName == "" {
		log.Fatal("Oups")
	}

	if m.installPath != "" {
		return path.Join(m.envRoot, m.installPath, folderName)
	}

	return path.Join(m.envRoot, "modules", folderName)
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
		return "", fmt.Errorf("failed getting current commit for %s", m.Name())
	}
	defer headFile.Close()

	scanner = bufio.NewScanner(headFile)
	scanner.Scan()
	version := scanner.Text()

	return version, nil
}

func (m *GitModule) updateCache() error {
	var cmd *exec.Cmd

	if _, err := os.Stat(m.cacheFolder); err == nil {
		if _, err := os.Stat(path.Join(m.cacheFolder, ".git")); err != nil {
			// Cache folder exists, but is not a GIT Repo - we remove it and redownload
			os.RemoveAll(m.cacheFolder)
		} else {
			// Cache exists and is a git repository, we try to update it
			cmd = exec.Command("git", "fetch")
			cmd.Dir = m.cacheFolder
			if err := cmd.Run(); err != nil {
				return &DownloadError{error: err, retryable: true}
			}
			return nil
		}
	}

	cmd = exec.Command("git", "clone", m.repoURL, m.cacheFolder)
	if err := cmd.Run(); err != nil {
		return &DownloadError{error: err, retryable: true}
	}

	return nil
}

func (m *GitModule) Download() DownloadError {
	var cmd *exec.Cmd
	var err error

	if err = m.updateCache(); err != nil {
		return DownloadError{error: err, retryable: true}
	}

	gc := m.gitCommand(m.TargetFolder())
	cmd = exec.Command(gc[0], gc[1:]...)
	cmd.Dir = m.cacheFolder

	if err = cmd.Run(); err != nil {
		return DownloadError{error: err, retryable: true}
	}

	return DownloadError{error: nil, retryable: false}
}
