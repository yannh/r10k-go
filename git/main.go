package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

const TypeRef = uint8(0)
const TypeTag = uint8(1)
const TypeBranch = uint8(2)

type Ref struct {
	RefType uint8
	Ref     string
}

func sanitizeShellChars(s string) string {
	reg := regexp.MustCompile("[']")
	return reg.ReplaceAllString(s, "")
}

func NewRef(refType uint8, ref string) *Ref {
	if refType > TypeBranch {
		return nil
	}

	ref = sanitizeShellChars(ref)

	return &Ref{
		RefType: refType,
		Ref:     ref,
	}
}

func RevParse(path string) error {
	cmd := exec.Command("git", "rev-parse")
	cmd.Dir = path
	return cmd.Run()
}

func Clone(repo string, to string) error {
	cmdParameters := "clone"

	cmdParameters += " " + repo + " " + to

	cmd := exec.Command("git", strings.Split(cmdParameters, " ")...)
	fmt.Println("Running git " + cmdParameters)
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

func Checkout(path string, ref *Ref) error {
	var err error

	if ref == nil {
		return nil
	}

	fmt.Println("Running git checkout " + ref.Ref)
	cmd := exec.Command("git", "checkout", ref.Ref)
	cmd.Dir = path

	if output, err := cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git fetch: %s", string(output))
	}

	return err
}

func RepoHasRemoteBranch(origin string, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--exit-code", "-h", origin, branch)
	fmt.Println("Running git ls-remote --exit-code -h" + origin + " " + branch)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func WorktreeAdd(directory string, ref *Ref, to string) error {
	var cmd *exec.Cmd
	var cwd string
	var err error
	var output []byte

	if _, err := os.Stat(path.Join(directory, ".git")); err != nil {
		return fmt.Errorf("can not create worktree from %s: folder is not a git repository", directory)
	}

	if !path.IsAbs(to) {
		cwd, _ = os.Getwd()
		to = path.Join(cwd, to)
	}

	cmdLineParameters := "worktree add --detach -f " + to

	if ref != nil {
		switch ref.RefType {
		case TypeBranch:
			cmdLineParameters += " " + ref.Ref

		case TypeTag, TypeRef:
			cmdLineParameters += " " + ref.Ref
		}
	}

	fmt.Println("Running git " + cmdLineParameters)
	cmd = exec.Command("git", strings.Split(cmdLineParameters, " ")...)
	cmd.Dir = directory
	if output, err = cmd.CombinedOutput(); err != nil {
		err = fmt.Errorf("failed running git %s: %s", cmdLineParameters, string(output))
	}

	return err
}
