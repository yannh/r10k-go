package puppetfileparser

import (
	"bufio"
	"fmt"
	"strings"
)

type ErrMalformedPuppetfile struct{ S string }

func (e ErrMalformedPuppetfile) Error() string { return e.S }
func NewErrMalformedPuppetfile(s string, params ...interface{}) error {
	return ErrMalformedPuppetfile{S: fmt.Sprintf(s, params)}
}

func parseParameter(line string) string {
	if strings.Contains(line, "=>") {
		return strings.Trim(strings.Split(line, "=>")[1], " \"'")
	}

	return strings.Trim(strings.SplitN(line, ":", 3)[2], " \"'")
}

func parseModule(line string) (map[string]string, error) {
	module := make(map[string]string)

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "mod") {
		return nil, NewErrMalformedPuppetfile("Error: Module definition not starting with mod")
	}

	for index, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "mod"):
			quoted_elements := strings.FieldsFunc(part, func(r rune) bool {
				return r == '\'' || r == '"'
			})
			// Several quoted entities, not separated by a coma, eg: mod "ntp" "1.0.3"`
			if len(quoted_elements) > 2 || len(quoted_elements)%2 != 0 {
				return nil, NewErrMalformedPuppetfile("error parsing line %s - missing coma?", part)
			}

			module["name"] = quoted_elements[1]

			// A line will contain : if it's in the form :tag: value or :tag => value
			// if not then it must be a version string, and no further parameter is allowed
		case index == 1 && !strings.Contains(part, "=>") && part != ":latest" && !strings.Contains(part, ":"):
			module["type"] = "forge"
			module["version"] = strings.Trim(part, " \"'")

		case index == 1 && part == ":latest":
			module["type"] = "forge"

		case strings.HasPrefix(part, ":github_tarball"):
			module["type"] = "github_tarball"
			module["repoName"] = parseParameter(part)

		case strings.HasPrefix(part, ":git"):
			module["type"] = "git"
			module["repoUrl"] = parseParameter(part)

		case strings.HasPrefix(part, ":install_path"):
			module["installPath"] = parseParameter(part)

		case strings.HasPrefix(part, ":tag"):
			module["tag"] = parseParameter(part)

		case strings.HasPrefix(part, ":ref"):
			module["ref"] = parseParameter(part)

		case strings.HasPrefix(part, ":branch"):
			module["branch"] = parseParameter(part)

		default:
			return nil, NewErrMalformedPuppetfile("unsupported parameter %s", part)
		}
	}

	return module, nil
}

func Parse(s *bufio.Scanner) (modules []map[string]string, opts map[string]string, err error) {
	opts = make(map[string]string)
	modules = make([]map[string]string, 0, 5)

	lineNumber := 0

	for block := ""; s.Scan(); {
		lineNumber++

		line := s.Text()
		line = strings.Split(s.Text(), "#")[0] // Remove comments
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		block += line

		optionValue := func(block string) string {
			return strings.FieldsFunc(block, func(r rune) bool {
				return r == '\'' || r == '"'
			})[1]
		}

		if !strings.HasSuffix(line, ",") { // Full Block
			switch {
			case strings.HasPrefix(block, "forge"):
				opts["forge"] = optionValue(block)

			case strings.HasPrefix(block, "moduledir"):
				opts["moduledir"] = optionValue(block)

			case strings.HasPrefix(block, "mod"):
				module, err := parseModule(block)
				if err != nil {
					return nil, nil, err
				}
				_, hasRef := module["ref"]
				_, hasBranch := module["branch"]
				_, hasTag := module["tag"]
				if (hasRef && hasBranch) || (hasRef && hasTag) || (hasBranch && hasTag) {
					return nil, nil, NewErrMalformedPuppetfile("can only set one of ref, branch, tag for module %s", module["name"])
				}

				modules = append(modules, module)

			default:
				return nil, nil, NewErrMalformedPuppetfile("failed parsing Puppetfile, error around line: %d", lineNumber)
			}

			block = ""
		}
	}

	return modules, opts, nil
}
