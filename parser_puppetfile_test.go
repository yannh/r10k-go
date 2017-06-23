package main

import "testing"

func TestParseModuleGit(t *testing.T) {
	parser := PuppetFileParser{}

	cases := []string{
		"mod 'puppetlabs/puppetlabs-apache', :git => 'https://github.com/puppetlabs/puppetlabs-apache.git'",
		"mod  \"puppetlabs/puppetlabs-apache\",    :git  =>      \"https://github.com/puppetlabs/puppetlabs-apache.git\"  ",
	}

	expected := &GitModule{
		name:    "puppetlabs/puppetlabs-apache",
		repoUrl: "https://github.com/puppetlabs/puppetlabs-apache.git",
	}

	for _, c := range cases {
		actual, err := parser.parseModule(c)

		if err != nil {
			t.Fatal(err)
		}

		if actual.Name() != expected.Name() ||
			actual.(*GitModule).repoUrl != expected.repoUrl {
			t.Fatal("Failed parsing module")
		}
	}

}

func TestParseModuleForge(t *testing.T) {
	parser := PuppetFileParser{}

	cases := []string{
		"mod 'puppetlabs/puppetlabs-apache'",
		"mod    \"puppetlabs/puppetlabs-apache\"  ",
	}

	expected := &ForgeModule{
		name: "puppetlabs/puppetlabs-apache",
	}

	for _, c := range cases {
		actual, err := parser.parseModule(c)

		if err != nil {
			t.Fatal(err)
		}

		if actual.Name() != expected.Name() {
			t.Fatal("Failed parsing module")
		}
	}

}
