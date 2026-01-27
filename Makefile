all: periwiki

gosources := $(wildcard *.go) $(wildcard **/*.go) go.sum go.mod

.bin:
	mkdir -p .bin

.bin/sqlboiler: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/aarondl/sqlboiler/v4@latest
.bin/sqlboiler-sqlite3: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest

internal/storage/skeleton.db: internal/storage/schema.sql
	rm -f internal/storage/skeleton.db
	sqlite3 -init internal/storage/schema.sql internal/storage/skeleton.db ""

model: internal/storage/skeleton.db sqlboiler.toml .bin/sqlboiler .bin/sqlboiler-sqlite3
	PATH="$(shell pwd)/.bin:$(PATH)" go generate

periwiki: model $(gosources)
	go build -o periwiki ./cmd/periwiki

run: periwiki
	./periwiki

test: model
	go test ./...

test-verbose: model
	go test -v ./...

test-coverage: model
	go test -cover ./...

test-coverage-html: model
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: model
	go test -race ./...

clean:
	rm -rf .bin
	rm -rf internal/storage/skeleton.db
	rm -rf model
	rm -f coverage.out coverage.html

.PHONY: all run test test-verbose test-coverage test-coverage-html test-race clean
