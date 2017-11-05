#!/usr/bin/env bats

setup() {
  # We clean modules and cache between each test
  rm -rf modules modules-test .cache test-fixtures/modules
}

@test "invocation with a nonexistent puppetfile prints an error" {
  run r10k-go install --puppetfile nonexistent
  [ "$status" -eq 1 ]
  [[ "$output" = *"no such file or directory"* ]]
}

@test "invocation with test puppetfile succeeds" {
  run r10k-go install --puppetfile test-fixtures/Puppetfile-simple
  [ "$status" -eq 0 ]
  [[ "$output" = *"Downloaded voxpopuli/nginx"* ]]
}

# @test "should fail if module has incorrect URL" {
#   run r10k-go install --puppetfile test-fixtures/Puppetfile-wrong-url
#   [ "$status" -ne 0 ]
# }

@test "should fail on invalid Puppetfile" {
  run r10k-go install --puppetfile test-fixtures/Puppetfile-invalid
  [[ "$output" = *"failed parsing Puppetfile"* ]]
  [ "$status" -ne 0 ]
}

@test "should support install_path parameter" {
  run r10k-go install --puppetfile test-fixtures/Puppetfile-installpath
  [ -d test_install_path ]
  [ "$status" -eq 0 ]
}

