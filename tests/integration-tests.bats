#!/usr/bin/env bats

setup() {
  # We clean modules and cache between each test
  rm -rf modules modules-test .cache123 test-fixtures/modules environments-production
}

@test "invocation with a nonexistent puppetfile prints an error" {
  run r10k-go puppetfile install --puppetfile nonexistent
  [ "$status" -eq 1 ]
  [[ "$output" = *"no such file or directory"* ]]
}

@test "invocation with test puppetfile succeeds" {
  run r10k-go puppetfile install --puppetfile test-fixtures/Puppetfile-simple
  [ "$status" -eq 0 ]
  [[ "$output" = *"Downloaded voxpopuli/nginx"* ]]
}

@test "should fail on invalid Puppetfile" {
  run r10k-go puppetfile install --puppetfile test-fixtures/Puppetfile-invalid
  [[ "$output" = *"failed parsing Puppetfile"* ]]
  [ "$status" -ne 0 ]
}

@test "should support install_path parameter" {
  run r10k-go puppetfile install --puppetfile test-fixtures/Puppetfile-installpath
  [ -d test-fixtures/test_install_path ]
  [ "$status" -eq 0 ]
}

@test "should install a test environment successfully" {
  run r10k-go deploy environment single_git
  [ -f environments-production/single_git/modules/firewall/LICENSE ]
  [ -d .cache123 ]
  [ "$status" -eq 0 ]
}
