#!/usr/bin/make -f

.PHONY: all go-deps unit-tests test install integration-tests release build

all: clean lint go-deps install test release

clean:
	rm -rf .cache modules r10k-go environments test_install_path

lint:
	golint ./...

go-deps:
	go get -t ./...

unit-tests:
	go test -v ./...
	go vet -v ./...

integration-tests:
ifdef RUN_INTEGRATION_TESTS
	bats tests/integration-tests.bats
endif

test: unit-tests integration-tests

build:
	go build ./...

install:
	go install -race ./...

release:
	go install ./...
