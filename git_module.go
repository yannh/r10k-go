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

type gitModule struct {
	name        string
	repoURL     string
	installPath string
	cacheFolder string
	folder      string
	want        *git.Ref
}

func (m *gitModule) getName() string { return m.name }
func (m *gitModule) getInstallPath() string {
	return m.installPath
}

func (m *gitModule) isUpToDate(folder string) bool {
	if _, err := os.Stat(folder); err != nil {
		return false
	}

	// folder exists, but no version specified, anything goes
	if m.want == nil {
		return true
	}

	cmd := exec.Command("git", "show", "-s", "--pretty=%d", "HEAD")
	cmd.Dir = folder
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	switch m.want.RefType {
	case git.TypeRef:
		commit, err := m.currentCommit(folder)
		if err != nil {
			return false
		}

		return strings.Contains(string(output), "origin/"+m.want.Ref) ||
			strings.Contains(string(output), "tag: "+m.want.Ref) ||
			m.want.Ref == commit

	case git.TypeBranch:
		return strings.Contains(string(output), "origin/"+m.want.Ref)

	case git.TypeTag:
		return strings.Contains(string(output), "tag: "+m.want.Ref)
	}

	return false
}

func (m *gitModule) hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.repoURL))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *gitModule) currentCommit(folder string) (string, error) {
	var gitFile, headFile *os.File
	var err error
	worktreeFolder := ""

	if gitFile, err = os.Open(path.Join(folder, ".git")); err != nil {
		return "", fmt.Errorf("Error getting current commit for %s", m.getName())
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
		return "", fmt.Errorf("failed getting current commit for %s", m.getName())
	}
	defer headFile.Close()

	scanner = bufio.NewScanner(headFile)
	scanner.Scan()
	version := scanner.Text()

	return version, nil
}

func (m *gitModule) updateCache() error {
	if _, err := os.Stat(m.cacheFolder); err == nil {
		if _, err := os.Stat(path.Join(m.cacheFolder, ".git")); err != nil {
			// cache folder exists, but is not a GIT Repo - we remove it and redownload
			os.RemoveAll(m.cacheFolder)
		} else {
			// cache exists and is a git repository, we try to update it
			if err := git.Fetch(m.cacheFolder); err != nil {
				return &downloadError{error: err, retryable: true}
			}
			return nil
		}
	}

	if err := git.Clone(m.repoURL, m.cacheFolder); err != nil {
		return &downloadError{error: err, retryable: true}
	}

	return nil
}

func (m *gitModule) download(to string, cache *cache) *downloadError {
	var err error

	m.cacheFolder = path.Join(cache.folder, m.hash())

	if err = m.updateCache(); err != nil {
		return &downloadError{error: fmt.Errorf("failed updating cache: %v", err), retryable: true}
	}

	if err = os.MkdirAll(path.Join(to, ".."), 0755); err != nil {
		return &downloadError{error: fmt.Errorf("failed creating folder: %v", to), retryable: false}
	}

	if err = git.WorktreeAdd(m.cacheFolder, m.want, to); err != nil {
		return &downloadError{error: fmt.Errorf("failed creating subtree: %v", err), retryable: true}
	}

	return nil
}
