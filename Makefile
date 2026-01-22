all: periwiki

gosources := $(wildcard *.go) $(wildcard **/*.go) go.sum go.mod

.bin:
	mkdir -p .bin

.bin/sqlboiler: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/volatiletech/sqlboiler/v4@latest
.bin/sqlboiler-sqlite3: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest

db/skeleton.db: db/schema.sql
	rm -f db/skeleton.db
	sqlite3 -init db/schema.sql db/skeleton.db ""

model: db/skeleton.db sqlboiler.toml .bin/sqlboiler .bin/sqlboiler-sqlite3
	PATH="$(shell pwd)/.bin:$(PATH)" go generate

periwiki: model $(gosources)
	go build

run: periwiki
	./periwiki

test: model
	go test -v ./...

clean:
	rm -rf .bin
	rm -rf db/skeleton.db
	rm -rf model

.PHONY: all run test clean
