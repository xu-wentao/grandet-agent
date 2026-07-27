.PHONY: build run test test-fast test-integration fmt fmt-check vet conformance check tidy

build:
	go build -o bin/grandet ./cmd/grandet

run:
	go run ./cmd/grandet --help

test:
	$(MAKE) test-fast
	$(MAKE) test-integration

test-fast:
	go test ./...

test-integration:
	go test -tags=integration ./...

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$(gofmt -l ./cmd ./internal)"

vet:
	go vet ./...

conformance:
	go test ./internal/architecture

check: fmt-check vet conformance test

tidy:
	go mod tidy
