

.PHONY: all go-deps unit-tests test

all: go-deps test

go-deps:
	go get -t ./...

test: unit-tests

unit-tests:
	go test -v ./...
	go vet -v ./...

