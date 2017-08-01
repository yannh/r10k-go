package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
)

type PuppetFile struct {
	*os.File
	wg       *sync.WaitGroup
	filename string
}

func NewPuppetFile(puppetfile string) *PuppetFile {
	f, err := os.Open(puppetfile)
	if err != nil {
		log.Fatalf("could not open %s: %v", puppetfile, err)
	}

	return &PuppetFile{File: f, wg: &sync.WaitGroup{}, filename: puppetfile}
}

func (m *PuppetFile) Filename() string         { return m.filename }
func (p *PuppetFile) Close()                   { p.File.Close() }
func (p *PuppetFile) moduleProcessedCallback() { p.wg.Done() }

func (p *PuppetFile) parseParameter(line string) string {
	if strings.Contains(line, "=>") {
		return strings.Trim(strings.Split(line, "=>")[1], " \"'")
	} else {
		return strings.Trim(strings.SplitN(line, ":", 3)[2], " \"'")
	}
}

func (p *PuppetFile) parseModule(line string) (PuppetModule, error) {
	var name, repoUrl, repoName, moduleType, installPath, targetFolder, version string
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

		case index == 1 && part == ":latest":
			moduleType = "forge"
			version = "" // Latest will be downloaded when no version is given

		case strings.HasPrefix(part, ":github_tarball"):
			moduleType = "github_tarball"
			repoName = p.parseParameter(part)

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
			fmt.Printf("Unsupported parameter %s in %s\n", part, p.filename)
		}
	}

	switch {
	case moduleType == "git":
		return &GitModule{
			name:        name,
			repoUrl:     repoUrl,
			installPath: installPath,
			processed:   p.moduleProcessedCallback,
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

	case moduleType == "github_tarball":
		return &GithubTarballModule{
			name:         name,
			repoName:     repoName,
			version:      version,
			targetFolder: targetFolder,
			processed:    p.moduleProcessedCallback,
			cacheFolder:  "",
		}, nil

	default:
		return &ForgeModule{name: name, version: version, processed: p.moduleProcessedCallback}, nil
	}
}

func (p *PuppetFile) parse(s *bufio.Scanner) ([]PuppetModule, map[string]string, error) {
	opts := make(map[string]string)
	modules := make([]PuppetModule, 0, 5)

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
				module, err := p.parseModule(block)
				if err != nil {
					return nil, nil, err
				}
				modules = append(modules, module)

			default:
				return nil, nil, fmt.Errorf("failed parsing Puppetfile, error around line: %d\n", lineNumber)
			}

			block = ""
		}

	}

	return modules, opts, nil
}

type ErrMalformedPuppetfile struct{ s string }

func (e ErrMalformedPuppetfile) Error() string { return e.s }

func (p *PuppetFile) Process(modules chan<- PuppetModule, done func()) error {
	parsedModules, opts, err := p.parse(bufio.NewScanner(p.File))
	if err != nil {
		done()
		return ErrMalformedPuppetfile{err.Error()}
	}
	modulePath, ok := opts["moduledir"]
	if !ok {
		modulePath = "modules"
	}

	for _, module := range parsedModules {
		splitPath := strings.FieldsFunc(module.Name(), func(r rune) bool {
			return r == '/' || r == '-'
		})
		folderName := splitPath[len(splitPath)-1]
		module.SetTargetFolder(path.Join(modulePath, folderName))

		p.wg.Add(1)
		modules <- module
	}

	go func() {
		p.wg.Wait()
		done()
	}()

	return nil
}
