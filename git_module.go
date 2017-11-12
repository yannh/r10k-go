package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/yannh/r10k-go/git"
	"os"
	"os/exec"
	"path"
	"strings"
)

type GitModule struct {
	name          string
	repoURL       string
	modulesFolder string
	installPath   string
	cacheFolder   string
	folder        string
	processed     func()
	want          git.Ref
}

func (m *GitModule) Name() string { return m.name }
func (m *GitModule) Processed()   { m.processed() }

func (m *GitModule) IsUpToDate() bool {
	if _, err := os.Stat(m.Folder()); err != nil {
		return false
	}

	// folder exists, but no version specified, anything goes
	if m.want.Ref == "" && m.want.Branch == "" && m.want.Tag == "" {
		return true
	}

	if m.want.Ref != "" {
		commit, err := m.currentCommit()
		if err != nil {
			return false
		}
		return m.want.Ref == commit
	}

	cmd := exec.Command("git", "show", "-s", "--pretty=%d", "HEAD")
	cmd.Dir = m.Folder()
	output, _ := cmd.Output()

	if m.want.Branch != "" {
		return strings.Contains(string(output), "origin/"+m.want.Branch)
	}

	if m.want.Tag != "" {
		return strings.Contains(string(output), "tag: "+m.want.Tag)
	}

	return false
}

func (m *GitModule) Hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.repoURL))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *GitModule) SetModulesFolder(to string) {
	m.modulesFolder = to
}

func (m *GitModule) ModulesFolder() string {
	if m.installPath != "" {
		return m.installPath
	}

	return m.modulesFolder
}

func (m *GitModule) Folder() string {
	splitPath := strings.FieldsFunc(m.Name(), func(r rune) bool {
		return r == '/' || r == '-'
	})
	folderName := splitPath[len(splitPath)-1]

	return path.Join(m.ModulesFolder(), folderName)
}

func (m *GitModule) currentCommit() (string, error) {
	var gitFile, headFile *os.File
	var err error
	worktreeFolder := ""

	if gitFile, err = os.Open(path.Join(m.Folder(), ".git")); err != nil {
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
	if _, err := os.Stat(m.cacheFolder); err == nil {
		if _, err := os.Stat(path.Join(m.cacheFolder, ".git")); err != nil {
			// Cache folder exists, but is not a GIT Repo - we remove it and redownload
			os.RemoveAll(m.cacheFolder)
		} else {
			// Cache exists and is a git repository, we try to update it
			if err := git.Fetch(m.cacheFolder); err != nil {
				return &DownloadError{error: err, retryable: true}
			}
			return nil
		}
	}

	if err := git.Clone(m.repoURL, git.Ref{}, m.cacheFolder); err != nil {
		return &DownloadError{error: err, retryable: true}
	}

	return nil
}

func (m *GitModule) Download(to string, cache *Cache) *DownloadError {
	var err error

	m.cacheFolder = path.Join(cache.Folder, m.Hash())

	if err = m.updateCache(); err != nil {
		return &DownloadError{error: fmt.Errorf("failed updating cache: %v", err), retryable: true}
	}

	if err = git.WorktreeAdd(m.cacheFolder, m.want, to); err != nil {
		return &DownloadError{error: fmt.Errorf("failed creating subtree: %v", err), retryable: true}
	}

	return nil
}
