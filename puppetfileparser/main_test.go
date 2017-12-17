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
	type expected struct {
		opts    map[string]string
		modules []map[string]string
	}

	testCases := []struct {
		puppetfile string
		result     expected
	}{
		{
			puppetfile: `
mod 'puppetlabs-razor'
mod 'puppetlabs-ntp', '0.0.3'
mod 'puppetlabs-stdlib', :latest
      `,
			result: expected{
				opts: map[string]string{},
				modules: []map[string]string{
					{"name": "puppetlabs-razor", "version": ""},
					{"name": "puppetlabs-ntp", "version": "0.0.3"},
					{"name": "puppetlabs-stdlib", "version": ""},
				},
			},
		}, {
			puppetfile: `
forge "https://forgeapi.puppetlabs.com"
moduledir "test_folder"

mod "ntp", "1.0.3"
mod 'puppetlabs-stdlib',
  :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git"
      `,
			result: expected{
				opts: map[string]string{
					"forge":     "https://forgeapi.puppetlabs.com",
					"moduledir": "test_folder",
				},
				modules: []map[string]string{
					{"name": "ntp", "version": "1.0.3"},
					{"name": "puppetlabs-stdlib", "repoUrl": "git://github.com/puppetlabs/puppetlabs-stdlib.git"},
				},
			},
		},
	}

	for _, c := range testCases {
		modules, opts, err := Parse(bufio.NewScanner(strings.NewReader(c.puppetfile)))

		if err != nil {
			t.Errorf("Failed parsing module: %v.\n", err)
		}

		for i, module := range modules {
			for attribute, value := range c.result.modules[i] {
				if module[attribute] != value {
					t.Errorf("Failed parsing module, expected %s for attribute %s, got %s.\n", value, attribute, module[attribute])
				}
			}
		}

		for opt, optv := range opts {
			if c.result.opts[opt] != optv {
				t.Errorf("Failed parsing puppetfile options, expected %s for option %s, got %s.\n", c.result.opts[opt], opt, optv)
			}
		}
	}
}

func TestParseMalformedPuppetfiles(t *testing.T) {
	testCases := []string{
		// tag & branch defined
		`mod 'puppetlabs-stdlib',
 :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
 :tag => "1.0",
 :branch => "featurebranch"`,

		// ref & branch defined
		`mod 'puppetlabs-stdlib',
 :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
 :ref => "12345678",
 :branch => "featurebranch"`,

		// ref & tag defined
		`mod 'puppetlabs-stdlib',
 :git => "git://github.com/puppetlabs/puppetlabs-stdlib.git",
 :ref => "12345678",
 :tag => "1.0"`,

		// Missing comma
		`forge "https://forgeapi.puppetlabs.com"
mod 'puppetlabs-stdlib'
	:git => "git://github.com/puppetlabs/puppetlabs-stdlib.git"
`,

		// Missing comma
		`mod "ntp" "1.0.3"`,
	}

	for _, c := range testCases {
		_, _, err := Parse(bufio.NewScanner(strings.NewReader(c)))
		if _, ok := err.(ErrMalformedPuppetfile); !ok {
			t.Errorf("expecting malformedPuppetFile error, got: %v.\n", err)
		}
	}
}
