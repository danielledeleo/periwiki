---
display_title: Syntax help
---

## WikiLinks

Link between articles using double brackets:

| Syntax | Result |
|--------|--------|
| `[[Target]]` | Link to Target |
| `[[Target\|display text]]` | Link showing "display text" |

Spaces around brackets and pipes are trimmed.

**Dead links** (to non-existent pages) appear in red.

e.g. [[Periwiki:Deadlink]][^deadlink]

## Frontmatter

Optional metadata at the start of an article:

```
---
display_title: Custom Title
---
```

| Field | Purpose |
|-------|---------|
| `display_title` | Override the displayed title |

## Footnotes

Add references[^multi] with `[^label]` in text and `[^label]: content` at the end:

```
Fact[^1].

[^1]: Source citation.
```

Referencing[^multi] the same footnote multiple times will create multiple backlinks lettered a-z (see [below](#references)).

```
Fact[^1]
...
later reference to the same fact[^1]

[^1]: Source citation.
```


## Markdown quick reference

| Syntax | Result |
|--------|--------|
| `**bold**` | **bold** |
| `*italic*` | *italic* |
| `[text](url)` | Link[^link] |
| `# Heading` | Heading (1-6) |
| `` `code` `` | Inline code |
| `> quote` | Block quote |


[^deadlink]: Normally, a deadlink would lead to a page that you could edit.
[^multi]: This footnote was referenced twice
[^link]: For internal links, always prefer WikiLinks over Markdown links.