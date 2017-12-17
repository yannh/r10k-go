package puppetfileparser

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseModuleGit(t *testing.T) {
	cases := []string{
		"mod 'puppetlabs/puppetlabs-apache', :git => 'https://github.com/puppetlabs/puppetlabs-apache.git'",
		"mod  \"puppetlabs/puppetlabs-apache\",    :git  =>      \"https://github.com/puppetlabs/puppetlabs-apache.git\"  ",
		"mod 'puppetlabs/puppetlabs-apache',:git:'https://github.com/puppetlabs/puppetlabs-apache.git'",
	}

	expected := map[string]string{
		"name":    "puppetlabs/puppetlabs-apache",
		"repoUrl": "https://github.com/puppetlabs/puppetlabs-apache.git",
	}

	for _, c := range cases {
		actual, err := parseModule(c)
		if err != nil {
			t.Error(err)
		}

		if actual["name"] != expected["name"] ||
			actual["repoUrl"] != expected["repoUrl"] {
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
				{"name": "puppetlabs-stdlib", "repoUrl": "git://github.com/puppetlabs/puppetlabs-stdlib.git"},
			},
		},
	}

	for _, c := range testCases {
		modules, _, err := Parse(bufio.NewScanner(strings.NewReader(c.puppetfile)))

		if err != nil {
			t.Errorf("Failed parsing module: %v.\n", err)
		}

		for i, module := range modules {
			for attribute, value := range c.expected[i] {
				if module[attribute] != value {
					t.Errorf("Failed parsing module, expected %s for attribute %s, got %s.\n", value, attribute, module[attribute])
				}
			}
		}
	}
}

func TestParseMalformedPuppetfiles(t *testing.T) {
	testCases := []string{
		`
forge "https://forgeapi.puppetlabs.com"

mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
  :tag => "1.0",
  :branch => "featurebranch"
`,
		`
forge "https://forgeapi.puppetlabs.com"

mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
  :ref => "12345678",
  :branch => "featurebranch"
`,
		`
forge "https://forgeapi.puppetlabs.com"

mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
  :ref => "12345678",
  :tag => "1.0"
`,
		`
forge "https://forgeapi.puppetlabs.com"

mod 'puppetlabs-stdlib'
	:git => "git://github.com/puppetlabs/puppetlabs-stdlib.git"
`,
		`mod "ntp" "1.0.3"`,
	}

	for _, c := range testCases {
		_, _, err := Parse(bufio.NewScanner(strings.NewReader(c)))
		if _, ok := err.(ErrMalformedPuppetfile); !ok {
			t.Errorf("expecting malformedPuppetFile error, got: %v.\n", err)
		}
	}
}
