#!/usr/bin/env bats

setup() {
  # We clean modules and cache between each test
  rm -rf modules .tmp
}

@test "invocation with a nonexistent puppetfile prints an error" {
  run ./r10k-go install --puppetfile nonexistent
  [ "$status" -eq 1 ]
  [[ "$output" = *"no such file or directory"* ]]
}

@test "invocation with test puppetfile succeeds" {
  run ./r10k-go install --puppetfile test-fixtures/Puppetfile-simple
  [ "$status" -eq 0 ]
  [[ "$output" = *"Downloaded voxpopuli/nginx"* ]]
}
