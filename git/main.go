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
	return cmd.Run()
}

func Fetch(path string) error {
	cmd := exec.Command("git", "fetch")
	cmd.Dir = path
	return cmd.Run()
}

func ListBranches(path string) ([]string, error) {
	cmd := exec.Command("git", "branch", "--all", "--format", "%(refname:short)")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("Error getting branches %v", output)
	}

	return strings.Split(strings.Trim(string(output), "\n"), "\n"), nil
}

func RepoHasBranch(origin string, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--exit-code", "-h", origin, branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func WorktreeAdd(directory string, ref Ref, to string) error {
	var cmd *exec.Cmd
	var cwd string

	if !path.IsAbs(to) {
		cwd, _ = os.Getwd()
		to = path.Join(cwd, to)
	}

	cmdLineParameters := "worktree add --detach -f " + to
	if ref.Ref != "" {
		cmdLineParameters += " " + ref.Ref
	}

	if ref.Branch != "" {
		cmdLineParameters += " origin/" + ref.Branch
	}

	cmd = exec.Command("git", strings.Split(cmdLineParameters, " ")...)
	cmd.Dir = directory
	return cmd.Run()
}
