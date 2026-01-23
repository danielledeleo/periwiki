# Writing articles

Periwiki articles are written in [CommonMark](https://commonmark.org/) Markdown, extended with WikiLink syntax for connecting articles.

## WikiLinks

WikiLinks use double-bracket syntax to link between articles:

```markdown
Usage:
[[Target]]
[[Target|text override]]
```

```markdown
The common periwinkle, [[Littorina littorea]], is found along rocky coastlines.
```

This creates a link to the article titled "Littorina littorea", using that title as the link text.

To display text that differs from the destination article's title, use a pipe separator:

```markdown
The [[Littorina littorea|common periwinkle]] is found along rocky coastlines.
```

This links to "Littorina littorea" but displays "common periwinkle" to the reader.

```markdown
It is permissible for [[ Wikilinks | wiki links ]] to contain extra spaces.
```

Trailing and leading spaces around `[[`, `|`, and `]]` are trimmed. Extra spaces within the text are left intact.

### Dead links

WikiLinks to articles that do not yet exist are displayed in red, distinguishing them from links to existing content. These are sometimes called "red links." Clicking a dead link takes you to the editor for that article, inviting you to create it.

## Table of Contents

Articles containing two or more `##` headings automatically display a table of contents before the first heading.

## Markdown Reference

Periwiki supports standard CommonMark syntax. For the complete specification, see [commonmark.org](https://spec.commonmark.org/).

| Syntax | Result |
|--------|--------|
| `**bold**` | **bold** |
| `*italic*` | *italic* |
| `[text](url)` | Hyperlink * |
| `# Heading` | Heading (levels 1–6) |
| `` `code` `` | Inline code |
| `> quote` | Block quotation |

\* Standard Markdown links require the full URL. For internal links between articles, WikiLinks are preferred:

```markdown
<!-- Markdown link (verbose, fragile) -->
For more, see [Periwinkles](/wiki/Periwinkles).

<!-- WikiLink (preferred) -->
For more, see [[Periwinkles]].
```
