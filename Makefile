.PHONY: build run seed seed-reset test vet tidy

build:
	go build ./...

run:
	go run ./cmd/saubala-back

seed:
	go run ./cmd/seed

seed-reset:
	go run ./cmd/seed -reset

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy
