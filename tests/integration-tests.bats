#!/usr/bin/env bats

setup() {
  # We clean modules and cache between each test
  rm -rf modules modules-test .cache123 test-fixtures/modules environments-production test-fixtures/test_install_path
  ./test-fixtures/create-git-fixtures.sh
}

@test "invocation with a nonexistent puppetfile prints an error" {
  run r10k-go puppetfile install --puppetfile nonexistent
  [ "$status" -eq 1 ]
  [[ "$output" = *"no such file or directory"* ]]
}

@test "invocation with test puppetfile succeeds" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout validpuppetfile
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [ "$status" -eq 0 ]
  [[ "$output" = *"Downloaded testmodule"* ]]
  [ -d test-fixtures/source/modules/testmodule/ ]
}

@test "should fail on invalid Puppetfile" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout invalidpuppetfile
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [[ "$output" = *"failed parsing Puppetfile"* ]]
  [ "$status" -ne 0 ]
}

@test "should support install_path parameter" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout installpath
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [ -d test-fixtures/source/test_install_path/testmodule ]
  [ "$status" -eq 0 ]
}

@test "should support moduledir" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout validpuppetfile
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile --moduledir my_modules
  [ "$status" -eq 0 ]
  [[ "$output" = *"Downloaded testmodule"* ]]
  [ -d test-fixtures/source/my_modules/testmodule/ ]
}

@test "should download appropriate version" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout installv1
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [[ "$output" = *"Downloaded testmodule"* ]]
  [ -d test-fixtures/source/modules/testmodule/ ]
  [ "$status" -eq 0 ]
  run git --git-dir test-fixtures/source/modules/testmodule/.git  describe --tags
  [[ "$output" = *"v1"* ]]

  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout validpuppetfile
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [[ "$output" = *"Downloaded testmodule"* ]]
  [ "$status" -eq 0 ]
  run git --git-dir test-fixtures/source/modules/testmodule/.git  describe --tags
  [[ "$output" = *"v3"* ]]

  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout installv1
  run r10k-go puppetfile install --puppetfile test-fixtures/source/Puppetfile
  [ -d test-fixtures/source/modules/testmodule/ ]
  [ "$status" -eq 0 ]
  run git --git-dir test-fixtures/source/modules/testmodule/.git  describe --tags
  [[ "$output" = *"v1"* ]]
}


@test "should be able to validate a valid Puppetfile" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout validpuppetfile
  run r10k-go puppetfile check --puppetfile test-fixtures/source/Puppetfile
  [[ "$output" = *"Syntax OK"* ]]
  [ "$status" -eq 0 ]
}

@test "should fail validating an invalid Puppetfile" {
  run git --git-dir test-fixtures/source/.git --work-tree test-fixtures/source/ checkout invalidpuppetfile
  run r10k-go puppetfile check --puppetfile test-fixtures/source/Puppetfile
  [[ "$output" = *"failed parsing Puppetfile"* ]]
  [ "$status" -ne 0 ]
}

@test "should fail validating a non-existing Puppetfile" {
  run r10k-go puppetfile check --puppetfile test-fixtures/not-a-real-puppetfile
  [[ "$output" = *"could not open file"* ]]
  [ "$status" -ne 0 ]
}

@test "should install a test environment successfully" {
  run r10k-go deploy environment single_git
  [ -f environments-production/single_git/modules/firewall/LICENSE ]
  [ -d .cache123 ]
  [ "$status" -eq 0 ]
}
