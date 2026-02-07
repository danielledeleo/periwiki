all: periwiki

find_go = find . -name '*.go' -not -name '*_gen.go'
find_templates = find templates -name '*.html'
find_embedded = find internal/embedded/help -name '*.md'
find_statics = find static -type f

gosources := $(shell $(find_go)) go.sum go.mod
templates := $(shell $(find_templates))
embedded := $(shell $(find_embedded))
statics := $(shell $(find_statics))

generate:
	go generate ./internal/embedded

periwiki: generate $(gosources) $(templates) $(embedded) $(statics)
	go build -o periwiki ./cmd/periwiki

run: periwiki
	./periwiki

watch:
	@command -v entr >/dev/null 2>&1 || { echo "entr not found — see https://eradman.com/entrproject/"; exit 1; }
	{ $(find_go); $(find_templates); $(find_embedded); $(find_statics); echo ./Makefile; } | entr -r make run

test: generate
	go test ./...

test-verbose: generate
	go test -v ./...

test-coverage: generate
	go test -cover ./...

test-coverage-html: generate
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race: generate
	go test -race ./...

clean:
	rm -f periwiki
	rm -f internal/embedded/metadata_gen.go
	rm -f coverage.out coverage.html

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "  all                Build the periwiki binary (default)"
	@echo "  run                Build and run the server"
	@echo "  watch              Rebuild and restart on file changes (requires entr)"
	@echo "  generate           Run code generation (embedded metadata)"
	@echo "  test               Run tests"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-coverage      Run tests with coverage summary"
	@echo "  test-coverage-html Run tests and generate HTML coverage report"
	@echo "  test-race          Run tests with race detector"
	@echo "  clean              Remove build artifacts and generated files"
	@echo "  help               Show this help"

.PHONY: all run watch generate test test-verbose test-coverage test-coverage-html test-race clean help
