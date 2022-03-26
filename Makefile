all: static/main.css go

static/main.css: src/main.scss
	sass src/main.scss static/main.css

go:
	go build