.PHONY: test race vet fmt build

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

build:
	go build ./cmd/qlog
