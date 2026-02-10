# Periwiki

Periwiki is a self-hosted wiki for people who miss the old web — fast pages, server-rendered HTML, and no fuss.

It runs as a single Go binary backed by SQLite, with zero configuration out of the box. Clone, build, run.

Built for small teams, passion projects, and personal knowledge bases.

## Features

- **CommonMark extended with WikiLinks** — Standard Markdown plus `[[Wikilink]]` syntax to connect articles
- **Full revision history** — Every edit is saved, with diffs between any two versions
- **User accounts with optional anonymous editing** — Authentication when you need it, open contribution when you don't

## Documentation

**User-facing documentation** is built into the wiki itself as help articles (the `Periwiki:` namespace). After running Periwiki, visit [Periwiki:Help_overview](/wiki/Periwiki:Help_overview) for the full list.

**Developing Periwiki**
- [URL Reference](urls.md) — Article URLs, history, diffs, special pages
- [Architecture](architecture.md) — System overview and request flow
