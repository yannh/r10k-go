package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
)

type PuppetFileParser struct {
}

func (p *PuppetFileParser) parseParameter(line string) string {
	if strings.Contains(line, "=>") {
		return strings.Trim(strings.Split(line, "=>")[1], " \"'")
	} else {
		return strings.Trim(strings.SplitN(line, ":", 3)[2], " \"'")
	}
}

func (p *PuppetFileParser) parseModule(line string) (PuppetModule, error) {
	var name, repoUrl, moduleType, installPath, targetFolder, version string
	var tag, ref, branch = "", "", ""

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "mod") {
		return &GitModule{}, errors.New("Error: Module definition not starting with mod")
	}

	for index, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "mod"):
			name = strings.FieldsFunc(part, func(r rune) bool {
				return r == '\'' || r == '"'
			})[1]

		// A line will contain : if it's in the form :tag: value or :tag => value
		// if not then it must be a version string, and no further parameter is allowed
		case index == 1 && !strings.Contains(part, "=>") && part != ":latest" && !strings.Contains(part, ":"):
			moduleType = "forge"
			version = strings.Trim(part, " \"'")
			break

		case strings.HasPrefix(part, ":git"):
			moduleType = "git"
			repoUrl = p.parseParameter(part)

		case strings.HasPrefix(part, ":install_path"):
			installPath = p.parseParameter(part)

		case strings.HasPrefix(part, ":tag"):
			tag = p.parseParameter(part)

		case strings.HasPrefix(part, ":ref"):
			ref = p.parseParameter(part)

		case strings.HasPrefix(part, ":branch"):
			branch = p.parseParameter(part)

		default:
			fmt.Println("Unsupported parameter: " + part)
		}
	}

	if moduleType == "git" {
		return &GitModule{
			name:        name,
			repoUrl:     repoUrl,
			installPath: installPath,
			want: struct {
				ref    string
				tag    string
				branch string
			}{
				ref,
				tag,
				branch,
			},
			targetFolder: targetFolder,
			cacheFolder:  ""}, nil
	} else {
		return &ForgeModule{name: name, version: version}, nil
	}
}

func (p *PuppetFileParser) parsePuppetFile(s *bufio.Scanner) ([]PuppetModule, map[string]string) {
	opts := make(map[string]string)
	modules := make([]PuppetModule, 0, 5)

	for block := ""; s.Scan(); {
		line := s.Text()
		line = strings.Split(s.Text(), "#")[0] // Remove comments

		if len(line) == 0 {
			continue
		}

		block += line

		if !strings.HasSuffix(line, ",") { // Full Block
			switch {
			case strings.HasPrefix(block, "forge"):
				opts["forge"] = strings.FieldsFunc(block, func(r rune) bool {
					return r == '\'' || r == '"'
				})[1]

			case strings.HasPrefix(block, "moduledir"):
				opts["moduledir"] = strings.FieldsFunc(block, func(r rune) bool {
					return r == '\'' || r == '"'
				})[1]

			case strings.HasPrefix(block, "mod"):
				module, _ := p.parseModule(block)
				modules = append(modules, module)
			}
			block = ""
		}
	}

	return modules, opts
}

func (p *PuppetFileParser) parse(puppetFile io.Reader, modulesChan chan PuppetModule, wg *sync.WaitGroup) error {

	s := bufio.NewScanner(puppetFile)
	s.Split(bufio.ScanLines)

	modules, opts := p.parsePuppetFile(s)

	for _, module := range modules {
		module = p.compute(module, opts)
		wg.Add(1)
		modulesChan <- module
	}

	return nil
}

func (p *PuppetFileParser) compute(m PuppetModule, opts map[string]string) PuppetModule {
	modulePath, ok := opts["modulePath"]
	if !ok {
		modulePath = "modules"
	}

	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder(path.Join(modulePath, folderName))
	return m
}
