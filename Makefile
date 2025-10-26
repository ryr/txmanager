.PHONY: all lint test vet race

all: lint vet test

lint:
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test ./... -v

race:
	go test ./... -race -v
