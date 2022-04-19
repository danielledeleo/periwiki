all: static/main.css periwiki

static/main.css: src/main.scss
	sass src/main.scss static/main.css

.bin:
	mkdir -p .bin

.bin/sqlboiler: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/volatiletech/sqlboiler/v4@latest
.bin/sqlboiler-sqlite3: .bin
	GOBIN=$(shell pwd)/.bin go install github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest

db/skeleton.db: db/schema.sql
	rm db/skeleton.db
	sqlite3 -init db/schema.sql db/skeleton.db ""

model: db/skeleton.db sqlboiler.toml .bin/sqlboiler .bin/sqlboiler-sqlite3
	go generate

periwiki: model
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
