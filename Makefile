#!/usr/bin/make -f

.PHONY: all go-deps unit-tests test install integration-tests

all: clean go-deps test build integration-tests

clean:
	rm -rf .cache modules r10k-go environment

go-deps:
	go get -t ./...

test: unit-tests

unit-tests:
	go test -v ./...
	go vet -v ./...

install:
	go install ./...

integration-tests:
ifdef RUN_INTEGRATION_TESTS
	bats tests/integration-tests.bats
endif
