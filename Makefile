.PHONY: build run test fmt tidy

build:
	go build -o bin/grandet ./cmd/grandet

run:
	go run ./cmd/grandet --help

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy
