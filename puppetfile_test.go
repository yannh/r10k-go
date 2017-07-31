package main

import (
	"testing"
	"bufio"
	"strings"
)

func TestParseModuleGit(t *testing.T) {
	cases := []string{
		"mod 'puppetlabs/puppetlabs-apache', :git => 'https://github.com/puppetlabs/puppetlabs-apache.git'",
		"mod  \"puppetlabs/puppetlabs-apache\",    :git  =>      \"https://github.com/puppetlabs/puppetlabs-apache.git\"  ",
		"mod 'puppetlabs/puppetlabs-apache',:git:'https://github.com/puppetlabs/puppetlabs-apache.git'",
	}

	expected := &GitModule{
		name:    "puppetlabs/puppetlabs-apache",
		repoUrl: "https://github.com/puppetlabs/puppetlabs-apache.git",
	}

	for _, c := range cases {
		pf := PuppetFile{}
    actual, err := pf.parseModule(c)
		if err != nil {
			t.Error(err)
		}

		if actual.Name() != expected.Name() ||
			actual.(*GitModule).repoUrl != expected.repoUrl {
			t.Error("Failed parsing module")
		}
	}

}

func TestParse(t *testing.T) {

  s := bufio.NewScanner(strings.NewReader("mod 'puppetlabs-razor'\nmod 'puppetlabs-ntp', '0.0.3'"))
	pf := PuppetFile{}
	modules, _ := pf.parse(s)

	fm := modules[0].(*ForgeModule)
	if fm.name != "puppetlabs-razor" {
		t.Error("Failed parsing file")
	}
	fm = modules[1].(*ForgeModule)
	if fm.name != "puppetlabs-ntp" || fm.version != "0.0.3" {
		t.Error("Failed parsing file")
	}


}
