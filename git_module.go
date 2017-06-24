package main

import "os/exec"
import "os"
import "crypto/sha1"
import "encoding/base64"

type GitModule struct {
	name         string
	repoUrl      string
	installPath  string
	ref          string
	targetFolder string
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

func (m *GitModule) Download(cache Cache) (string, error) {
	var cmd *exec.Cmd
	var err error

	hash := m.Hash()
	if !cache.Has(m) {
		cmd = exec.Command("git", "clone", m.repoUrl, cache.folder+hash)
		if err := cmd.Run(); err != nil {
			return "", &GitDownloadError{err: err, retryable: true}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", &GitDownloadError{err: err, retryable: false}
	}

	// TODO use path.Join instead of / everwhere
	if m.ref == "" {
		cmd = exec.Command("git", "worktree", "add", "--detach", "-f", cwd+"/"+m.targetFolder)
	} else {
		cmd = exec.Command("git", "worktree", "add", "--detach", "-f", cwd+"/"+m.targetFolder, m.ref)
	}
	cmd.Dir = cache.folder + hash

	if err = cmd.Run(); err != nil {
		return "", &GitDownloadError{err: err, retryable: true}
	}

	return m.targetFolder, nil
}
