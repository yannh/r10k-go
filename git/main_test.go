package git

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.RemoveAll("tmp")
	if _, err := os.Stat("test-fixtures/git-repo/git"); err == nil {
		os.Rename("test-fixtures/git-repo/git", "test-fixtures/git-repo/.git")
	}
	res := m.Run()
	if _, err := os.Stat("test-fixtures/git-repo/.git"); err == nil {
		os.Rename("test-fixtures/git-repo/.git", "test-fixtures/git-repo/git")
	}
	os.Exit(res)
}

func TestCloneSuccess(t *testing.T) {
	if err := Clone("test-fixtures/git-repo/", "tmp/git-repo"); err != nil {
		fmt.Print(err)
		t.Error(err.Error())
	}

	filename := "tmp/git-repo/readme.md"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("cloning failed; failed to create " + filename)
	}

	os.RemoveAll("tmp")
}

func TestCloneNonRepo(t *testing.T) {
	var err error
	if err = Clone("test-fixtures/not-a-git-repo/", "tmp/git-repo"); err == nil {
		t.Error("cloning a non existing repository should fail")
	}
}

func TestRepoHasRemoteBranchFailure(t *testing.T) {
	if RepoHasRemoteBranch("test-fixtures/git-repo/", "not-a-branch") == true {
		t.Error("repository test-fixtures/git-repo/ should not have a branch not-a-branch")
	}
}

func TestWorktreeAdd(t *testing.T) {
	if err := WorktreeAdd("test-fixtures/git-repo/", nil, "tmp/git-repo"); err != nil {
		t.Error(err)
	}

	filename := "tmp/git-repo/readme.md"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("creating worktree failed; failed to create " + filename)
	}
}

func TestWorktreeAddIncorrectPath(t *testing.T) {
	if err := WorktreeAdd("test-fixtures/not-a-git-repo/", nil, "tmp/git-repo"); err == nil {
		t.Error(err)
	}
}
