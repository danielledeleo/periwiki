---
display_title: Syntax reference
---

Quick reference for Periwiki's Markdown extensions. For a comprehensive guide, see [[Periwiki:Writing_articles|Writing Articles]].

## WikiLinks

| Syntax | Result |
|--------|--------|
| `[[Target]]` | Link to Target |
| `[[Target\|display text]]` | Link showing "display text" |

Spaces around brackets and pipes are trimmed. Links to non-existent pages appear in red.

## Frontmatter

Optional metadata at the start of an article, enclosed in `---` fences. Uses [NestedText](https://nestedtext.org/) format.

```
---
display_title: Custom Title
---
```

| Field | Purpose |
|-------|---------|
| `display_title` | Override the displayed title |
| `layout` | Page appearance (`mainpage` shows "Main page" tab) |
| `toc` | Set to `false` to hide the table of contents |

## Footnotes

Add references with `[^label]` in text and `[^label]: content` at the end:

```
Fact[^1].

[^1]: Source citation.
```

Referencing the same footnote multiple times creates multiple backlinks lettered a-z.

## Markdown

| Syntax | Result |
|--------|--------|
| `**bold**` | **bold** |
| `*italic*` | *italic* |
| `[text](url)` | Hyperlink |
| `# Heading` | Heading (1-6) |
| `` `code` `` | Inline code |
| `> quote` | Block quote |

For internal links, always prefer WikiLinks over Markdown links.
