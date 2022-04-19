all: static/main.css periwiki

static/main.css: src/main.scss
	sass src/main.scss static/main.css

generate: db/skeleton.db sqlboiler.toml
	go generate

db/skeleton.db: db/schema.sql
	sqlite3 -init db/schema.sql db/skeleton.db ""

periwiki: generate
	go build

run:
	./periwiki

test: generate
	go test -v ./...

clean:
	rm -rf static/main.css
	rm -rf db/skeleton.db
	rm -rf model

.PHONY: all generate run test clean
