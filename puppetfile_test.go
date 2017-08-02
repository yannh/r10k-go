package main

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestParseModuleGit(t *testing.T) {
	cases := []string{
		"mod 'puppetlabs/puppetlabs-apache', :git => 'https://github.com/puppetlabs/puppetlabs-apache.git'",
		"mod  \"puppetlabs/puppetlabs-apache\",    :git  =>      \"https://github.com/puppetlabs/puppetlabs-apache.git\"  ",
		"mod 'puppetlabs/puppetlabs-apache',:git:'https://github.com/puppetlabs/puppetlabs-apache.git'",
	}

	expected := &GitModule{
		name:    "puppetlabs/puppetlabs-apache",
		repoURL: "https://github.com/puppetlabs/puppetlabs-apache.git",
	}

	for _, c := range cases {
		pf := PuppetFile{}
		actual, err := pf.parseModule(c)
		if err != nil {
			t.Error(err)
		}

		if actual.Name() != expected.Name() ||
			actual.(*GitModule).repoURL != expected.repoURL {
			t.Error("failed parsing module")
		}
	}

}

func TestParse(t *testing.T) {
	type e struct {
		name    string
		version string
	}

	testCases := []struct {
		puppetfile string
		expected   []map[string]string
	}{
		{
			puppetfile: `
mod 'puppetlabs-razor'
mod 'puppetlabs-ntp', '0.0.3'
mod 'puppetlabs-stdlib', :latest
      `,
			expected: []map[string]string{
				{"name": "puppetlabs-razor", "version": ""},
				{"name": "puppetlabs-ntp", "version": "0.0.3"},
				{"name": "puppetlabs-stdlib", "version": ""},
			},
		}, {
			puppetfile: `
forge "https://forgeapi.puppetlabs.com"

mod "ntp", "1.0.3"
mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git"
      `,
			expected: []map[string]string{
				{"name": "ntp", "version": "1.0.3"},
				{"name": "puppetlabs-stdlib", "repoURL": "git://github.com/puppetlabs/puppetlabs-stdlib.git"},
			},
		},
	}

	for _, c := range testCases {
		pf := PuppetFile{}
		modules, _, err := pf.parse(bufio.NewScanner(strings.NewReader(c.puppetfile)))

		if err != nil {
			t.Errorf("Failed parsing module: %v.\n", err)
		}

		for i, module := range modules {
			if reflect.TypeOf(module).Elem().Name() == "ForgeModule" {
				m := module.(*ForgeModule)
				if m.Name() != c.expected[i]["name"] {
					t.Errorf("Failed parsing module, expected %s, got %s.\n", c.expected[i]["name"], module.Name())
				}
				if m.version != c.expected[i]["version"] {
					t.Errorf("Failed parsing module, expected %s, got %s.\n", c.expected[i]["version"], m.version)
				}
			} else if reflect.TypeOf(module).Elem().Name() == "GitModule" {
				m := module.(*GitModule)
				if m.Name() != c.expected[i]["name"] {
					t.Errorf("Failed parsing module, expected %s, got %s.\n", c.expected[i]["name"], module.Name())
				}
				if m.repoURL != c.expected[i]["repoURL"] {
					t.Errorf("Failed parsing module, expected %s, got %s.\n", c.expected[i]["repoURL"], m.repoURL)
				}
			} else {
				t.Errorf("Unknown module type: %s\n", reflect.TypeOf(module).Elem().Name())
			}

		}
	}
}
