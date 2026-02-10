# Writing articles

Periwiki articles are written in [CommonMark](https://commonmark.org/) Markdown, extended with WikiLink syntax for connecting articles.

## Frontmatter

Articles can include optional frontmatter at the very beginning, enclosed in `---` fences. Frontmatter uses [NestedText](https://nestedtext.org/) format (similar to YAML, but all values are strings—no surprises with wikilinks).

```markdown
---
display_title: The Common Periwinkle
---

The rest of the article content goes here...
```

### Supported fields

| Field | Purpose |
|-------|---------|
| `display_title` | Override the article's display title (URL remains unchanged) |
| `layout` | Controls page appearance. `mainpage` shows "Main page" tab instead of "Article" |
| `toc` | Set to `false` to hide the automatic table of contents |

Additional fields are preserved for future use.

### Why NestedText?

YAML has a footgun: `[[wikilinks]]` are silently parsed as nested arrays. NestedText treats all values as strings, so wikilinks work naturally:

```
---
see_also: [[Related Article]]
---
```

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

## Footnotes

Footnotes allow you to add references or supplementary information without interrupting the flow of text.

```markdown
The common periwinkle can live for up to 10 years[^lifespan].

[^lifespan]: Smith, J. (2020). "Marine Gastropod Longevity." Journal of Molluscs.
```

This renders as a superscript link in the text, with the footnote content appearing in a "References" section at the bottom of the article. Clicking the footnote number scrolls to the reference; clicking the backlink (^) returns to the text.

Multiple references to the same footnote are supported:

```markdown
Periwinkles are herbivores[^diet]. They graze on algae[^diet].

[^diet]: They primarily consume microalgae and diatoms.
```

When a footnote is referenced multiple times, each backlink is labeled (^a, ^b, etc.) to return readers to the specific location in the text.

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
