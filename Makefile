.PHONY: build run test integration-test fmt tidy vet

build:
	go build -o bin/grandet ./cmd/grandet

run:
	go run ./cmd/grandet --help

test:
	go test ./...

integration-test:
	go test -tags=integration ./internal/infrastructure

vet:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

tidy:
	go mod tidy
