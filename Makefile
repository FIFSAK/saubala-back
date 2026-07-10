.PHONY: build run seed seed-reset import import-dry test vet tidy

build:
	go build ./...

run:
	go run ./cmd/saubala-back

seed:
	go run ./cmd/seed

seed-reset:
	go run ./cmd/seed -reset

import:
	go run ./cmd/import-xlsx -file "поставки 2026.xlsx"

import-dry:
	go run ./cmd/import-xlsx -file "поставки 2026.xlsx" -dry-run

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy
