#!/usr/bin/make -f

.PHONY: all go-deps unit-tests test build integration-tests

all: go-deps test build integration-tests

go-deps:
	go get -t ./...

test: unit-tests

unit-tests:
	go test -v ./...
	go vet -v ./...

build:
	go build ./...

integration-tests:
ifdef RUN_INTEGRATION_TESTS
	./r10k-go install --puppetfile test-fixtures/Puppetfile
endif