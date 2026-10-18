.PHONY: build test run

build:
	go build -o bin/termchess ./cmd/termchess

test:
	go test ./...

run:
	go run ./cmd/termchess
