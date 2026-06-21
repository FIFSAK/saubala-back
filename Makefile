.PHONY: build run test vet tidy

build:
	go build ./...

run:
	go run ./cmd/saubala-back

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy
