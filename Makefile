# Mill Makefile — build, test, and release targets

.PHONY: build test vet fmt release version

## Build the mill binary
build:
	go build -o mill ./cmd/mill

## Run all tests
test:
	go test ./...

## Run go vet
vet:
	go vet ./...

## Format code
fmt:
	go fmt ./...

## Show current version
version:
	go run ./cmd/mill version

## Cut a new release: scripts/release.sh <version>
# Usage: make release VERSION=v0.2.0
release:
	@scripts/release.sh $(VERSION)
