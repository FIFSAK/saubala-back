.PHONY: build run client test vet

build:
	go build ./...

run:
	go run ./cmd/saubala-back

client:
	go run ./cmd/client

test:
	go test ./...

vet:
	go vet ./...
