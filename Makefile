all: sass go

sass:
	sass src/main.scss static/main.css

go:
	go build