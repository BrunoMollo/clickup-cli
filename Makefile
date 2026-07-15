.PHONY: build test test-race vet check

build:
	go build -o bin/clickdown ./cmd/clickdown

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

check: test-race vet build
