# Usage

## Article Markdown
periwiki uses Goldmark's standard CommonMark parser with one addition: WikiLinks.

```markdown
Within your [[text]], you can create wikilinks that point to other articles on your wiki.

You may also [[ Destination | change where ]] your link points.
```

Result:
```html
<p>Within your <a href="text" title="text" rel="nofollow">text</a>, you can wikilinks that point to other articles on your wiki.</p>
<p>You may also <a href="Destination" title="change where" rel="nofollow">change where</a> your link points.</p>
```