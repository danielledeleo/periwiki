all: periwiki

find_go = find . -name '*.go' -not -path './model/*'
find_templates = find templates -name '*.html'
find_embedded = find internal/embedded/help -name '*.md'

gosources := $(shell $(find_go)) go.sum go.mod
templates := $(shell $(find_templates))
embedded := $(shell $(find_embedded))

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
	PATH="$(shell pwd)/.bin:$(PATH)" go generate ./...

periwiki: model $(gosources) $(templates) $(embedded)
	go build -o periwiki ./cmd/periwiki

run: periwiki
	./periwiki

watch:
	@command -v entr >/dev/null 2>&1 || { echo "entr not found — see https://eradman.com/entrproject/"; exit 1; }
	{ $(find_go); $(find_templates); $(find_embedded); echo ./Makefile; } | entr -r make run

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
	rm -f periwiki
	rm -rf internal/storage/skeleton.db
	rm -rf model
	rm -f internal/embedded/metadata_gen.go
	rm -f coverage.out coverage.html

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "  all                Build the periwiki binary (default)"
	@echo "  run                Build and run the server"
	@echo "  watch              Rebuild and restart on file changes (requires entr)"
	@echo "  model              Regenerate SQLBoiler models from schema"
	@echo "  test               Run tests"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-coverage      Run tests with coverage summary"
	@echo "  test-coverage-html Run tests and generate HTML coverage report"
	@echo "  test-race          Run tests with race detector"
	@echo "  clean              Remove build artifacts and generated files"
	@echo "  help               Show this help"

.PHONY: all run watch test test-verbose test-coverage test-coverage-html test-race clean help
