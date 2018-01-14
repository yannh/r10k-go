package puppetmodule

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/yannh/r10k-go/git"
)

type GitModule struct {
	name        string   // puppetlabs-apache
	repoURL     string   // https://github.com/puppetlabs/puppetlabs-apache.git
	installPath string   // if specified per module, otherwise empty string
	want        *git.Ref // The tag, branch or ref
}

func NewGitModule(name, repoURL, installPath string, want *git.Ref) *GitModule {
	return &GitModule{
		name:        name,
		repoURL:     repoURL,
		installPath: installPath,
		want:        want,
	}
}

func (m *GitModule) Name() string { return m.name }
func (m *GitModule) GetInstallPath() string {
	return m.installPath
}

func (m *GitModule) IsUpToDate(folder string) bool {
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

func (m *GitModule) hash() string {
	hasher := sha1.New()
	hasher.Write([]byte(m.repoURL))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (m *GitModule) currentCommit(folder string) (string, error) {
	var gitFile, headFile *os.File
	var err error
	worktreeFolder := ""

	if gitFile, err = os.Open(path.Join(folder, ".git")); err != nil {
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

func (m *GitModule) updateCache(cacheFolder string) error {
	if _, err := os.Stat(cacheFolder); err == nil {
		if _, err := os.Stat(path.Join(cacheFolder, ".git")); err != nil {
			// cache folder exists, but is not a GIT Repo - we remove it and redownload
			os.RemoveAll(cacheFolder)
		} else {
			// cache exists and is a git repository, we try to update it
			if err := git.Fetch(cacheFolder); err != nil {
				return &DownloadError{error: err, Retryable: true}
			}
			return nil
		}
	}

	if err := git.Clone(m.repoURL, cacheFolder); err != nil {
		return &DownloadError{error: err, Retryable: true}
	}

	return nil
}

func (m *GitModule) Download(to string, cache string) *DownloadError {
	var err error

	if err = m.updateCache(path.Join(cache, m.hash())); err != nil {
		return &DownloadError{error: fmt.Errorf("failed updating cache: %v", err), Retryable: true}
	}

	if err = os.MkdirAll(path.Join(to, ".."), 0755); err != nil {
		return &DownloadError{error: fmt.Errorf("failed creating folder: %v", to), Retryable: false}
	}

	if err = git.WorktreeAdd(path.Join(cache, m.hash()), m.want, to); err != nil {
		return &DownloadError{error: fmt.Errorf("failed creating subtree: %v", err), Retryable: true}
	}

	return nil
}
