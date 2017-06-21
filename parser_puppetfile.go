package main

import "strings"
import "errors"
import "bufio"
import "io"
import "fmt"
import "sync"

type PuppetFileParser struct {
}

func (p *PuppetFileParser) parseModule(line string) (PuppetModule, error) {
	var name, repoUrl, moduleType, installPath, ref, targetFolder string

	if !strings.HasPrefix(line, "mod") {
		return &GitModule{}, errors.New("Error: Module definition not starting with mod")
	}

	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "mod"):
			name = strings.FieldsFunc(part, func(r rune) bool {
				return r == '\'' || r == '"'
			})[1]

		case strings.HasPrefix(part, ":git"):
			moduleType = "git"
			repoUrl = strings.Trim(strings.Split(part, "=>")[1], " \"'")

		case strings.HasPrefix(part, ":install_path"):
			installPath = strings.Trim(strings.Split(part, "=>")[1], " \"'")

		case strings.HasPrefix(part, ":tag") || strings.HasPrefix(part, ":ref") || strings.HasPrefix(part, ":branch"):
			ref = strings.Trim(strings.Split(part, "=>")[1], " \"'")
		}
	}

	if moduleType == "git" {
		return &GitModule{name, repoUrl, installPath, ref, targetFolder}, nil
	} else {
    return nil, errors.New("Unknown module type")
  }
}

func (p *PuppetFileParser) parsePuppetFile(s *bufio.Scanner) ([]PuppetModule, map[string]string) {
	opts := make(map[string]string)
	modules := make([]PuppetModule, 0, 5)

	for block := ""; s.Scan(); {
		line := s.Text()
    fmt.Println(line)
		line = strings.Split(s.Text(), "#")[0] // Remove comments
		line = strings.TrimSpace(line)

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
		modulePath = "./modules/"
	}
	splitPath := strings.Split(m.Name(), "/")
	folderName := splitPath[len(splitPath)-1]
	m.SetTargetFolder(modulePath + folderName)
	return m
}
