.PHONY: build install test

build:
	go build -o bin/zi ./cmd/zi

install:
	go install ./cmd/zi

test:
	go test ./...
