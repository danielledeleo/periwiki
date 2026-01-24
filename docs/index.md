# Periwiki

Periwiki is a self-hosted wiki for people who miss the old web — fast pages, server-rendered HTML, and no fuss.

It runs as a single Go binary backed by SQLite, with zero configuration out of the box. Clone, build, run.

Built for small teams, passion projects, and personal knowledge bases.

## Features

- **CommonMark extended with WikiLinks** — Standard Markdown plus `[[Wikilink]]` syntax to connect articles
- **Full revision history** — Every edit is saved, with diffs between any two versions
- **User accounts with optional anonymous editing** — Authentication when you need it, open contribution when you don't

## Documentation

**Running Periwiki**
- [Installation](installation-and-configuration.md) — Building from source, configuration, first run
- [Security](security.md) — Password handling, sessions, HTML sanitization
- [Troubleshooting](troubleshooting.md) — Common issues and log interpretation

**Using Periwiki**
- [Writing Articles](writing-articles.md) — Markdown syntax and WikiLinks
- [URL Reference](urls.md) — Article URLs, history, diffs, special pages

**Developing Periwiki**
- [Architecture](architecture.md) — System overview and request flow
