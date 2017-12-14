package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

type Ref struct {
	Ref    string
	Tag    string
	Branch string
}

func RevParse(path string) error {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run()
}

func Clone(repo string, ref Ref, to string) error {
	cmdParameters := "clone"
	if ref.Branch != "" {
		cmdParameters += " -b " + ref.Branch
	}
	cmdParameters += " " + repo + " " + to

	cmd := exec.Command("git", strings.Split(cmdParameters, " ")...)
	if output, err := cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git %s: %s", cmdParameters, string(output))
		return err
	}

	return nil
}

func Fetch(path string) error {
	var err error

	cmd := exec.Command("git", "fetch")
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git fetch: %s", string(output))
	}

	return err
}

func Checkout(path string, branch string) error {
	var err error

	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git fetch: %s", string(output))
	}

	return err
}

func RepoHasRemoteBranch(origin string, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--exit-code", "-h", origin, branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func WorktreeAdd(directory string, ref Ref, to string) error {
	var cmd *exec.Cmd
	var cwd string
	var err error

	if _, err := os.Stat(path.Join(directory, ".git")); err != nil {
		return fmt.Errorf("can not create worktree from %s: folder is not a git repository", directory)
	}

	if !path.IsAbs(to) {
		cwd, _ = os.Getwd()
		to = path.Join(cwd, to)
	}

	cmdLineParameters := "worktree add --detach -f " + to
	if ref.Ref != "" {
		cmdLineParameters += " " + ref.Ref
	}

	if ref.Branch != "" {
		cmdLineParameters += " " + ref.Branch
	}

	cmd = exec.Command("git", strings.Split(cmdLineParameters, " ")...)
	cmd.Dir = directory
	if output, err := cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git %s: %s", cmdLineParameters, string(output))
	}

	return err
}
