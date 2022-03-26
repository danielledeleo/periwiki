<img width="192" height="65.6667" src="assets/iwikii2x.png">


MediaWiki-inspired, [Goldmark](https://github.com/yuin/goldmark) powered wiki written in Go with an SQLite backend.

## Why?
Because I don't like managing PHP installations.

There are a ton of "Hello World" wikis written in Go out there<sup>1</sup>. The longterm goal of iwikii is be a serious, lightweight, (maybe fast) alternative to MediaWiki for smaller, simple wikis.

## What's with the name?
It came from Namelix's neat machine learning name generator. I pronounce it /ɪwɪki/ and spell it **iwikii**. 

## It sort of looks like MediaWiki
That is not an accident. It is what a wiki should look like.

## What's the license?
The [Mozilla Public License](LICENSE). Share what you do with it!

## Build/Run
Requirements: `make`, and `go`. `sass` is optional unless you make any edits to the .scss as a compiled .css file is included. 

`git clone github.com/jagger27/iwikii`

`make` (or just `go build`)

`./iwikii`

## Anything else?
See [TODO](TODO.md) for a little insight on what's on the road map.

<sub>1: Mostly because of this wonderful intro to Go web apps, [Writing Web Applications](https://golang.org/doc/articles/wiki/).</sub>
