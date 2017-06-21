package main

import "testing"
import "strings"
import "bufio"

func TestParseModule(t *testing.T)  {
  parser := PuppetFileParser{}

  s := "mod \"puppetlabs/puppetlabs-apache\", :git => \"https://github.com/puppetlabs/puppetlabs-apache.git\""
  f := bufio.NewScanner(strings.NewReader(s))
  m, _ := parser.parsePuppetFile(f)

  if len(m) != 1 ||
     m[0].Name() != "puppetlabs/puppetlabs-apache" ||
     m[0].(*GitModule).repoUrl != "https://github.com/puppetlabs/puppetlabs-apache.git" {
    t.Error()
  }
}
