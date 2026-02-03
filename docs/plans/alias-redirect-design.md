# Article Redirect System Design

**Status:** Ready to implement (frontmatter parsing complete ✓)

## Overview

Article redirects via frontmatter. One hop only (following MediaWiki's model).

Config-level rewrites/redirects are out of scope—nginx/reverse proxy handles this better.

## Frontmatter Syntax

```
---
redirect: Prince_(Disambiguation)
---
```

Uses plain article URL, not wikilink syntax. NestedText handles this naturally (no quoting needed).

## Behavior

| Route | Redirects? |
|-------|------------|
| `/wiki/Prince` | Yes → 301 to target |
| `/wiki/Prince?redirect=no` | No, shows redirect article |
| `/wiki/Prince?edit` | No, edits the redirect article |
| `/wiki/Prince?history` | No, shows redirect article history |
| `/wiki/Prince?revision={id}` | No |
| `/wiki/Prince?diff` | No |

Only the default "view" action triggers the redirect.

## Storage

Use the frontmatter JSONB cache in the Article table with an index:

```sql
CREATE INDEX idx_article_redirect ON Article(
    json_extract(frontmatter, '$.redirect')
) WHERE json_extract(frontmatter, '$.redirect') IS NOT NULL;
```

No separate Redirect table needed.

### Query patterns

```sql
-- Check if article is a redirect
SELECT json_extract(frontmatter, '$.redirect') as target
FROM Article WHERE url = ?;

-- Find all redirects pointing to an article (for "What links here")
SELECT url FROM Article
WHERE json_extract(frontmatter, '$.redirect') = ?;
```

## Chain Resolution

**One hop only**, following [MediaWiki's approach](https://www.mediawiki.org/wiki/Help:Redirects).

If A → B and B → C:
- Visiting A redirects to B
- B shows content with warning: "This is a redirect to [[C]]. Consider updating [[A]] to point directly to [[C]]."

Double redirects are an editorial problem, not a software problem. This matches MediaWiki's model where bots fix double redirects after page moves.

### Why not follow chains?

1. **Simpler implementation** - no chain resolution logic, no loop detection
2. **Prior art** - MediaWiki deliberately doesn't follow chains
3. **Transparency** - users see where they're actually going
4. **Performance** - single lookup, no recursion

## Redirect Note

When arriving via redirect, show:

> (Redirected from [Prince](/wiki/Prince?redirect=no))

### Implementation: Referer header

```go
func (a *app) handleView(...) {
    referer := req.Header.Get("Referer")
    if refererURL, err := url.Parse(referer); err == nil {
        sourceURL := extractArticleURL(refererURL.Path)
        // Check if referer article redirects to THIS article
        if a.articles.RedirectsTo(sourceURL, articleURL) {
            render["RedirectedFrom"] = sourceURL
        }
    }
}
```

**Why Referer header:**
- No URL pollution (`?from=` param)
- Self-validating against frontmatter
- Graceful degradation if stripped by privacy settings

## Loop Handling

If A → A (self-redirect) or A → B → A (detected when B is also a redirect):
- Serve 200 with article content
- Show error banner: "This article creates a redirect loop."

Loop detection happens at render time by checking if target is also a redirect.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Redirect to nonexistent article | 301 to target, target shows "not found" page |
| Delete redirect article | Normal deletion, no special handling |
| Edit redirect to remove frontmatter | No longer redirects |
| Self-redirect (A → A) | Loop error banner, serve content |
| Double redirect (A → B → C) | A → B works, B shows warning about double redirect |

## Implementation Plan

1. Add `redirect` field to Frontmatter struct
2. Create index on frontmatter redirect field
3. Update article handler to check for redirect on view
4. Add redirect note template partial
5. Add double-redirect warning template partial
6. Tests for all route behaviors

## Future Considerations

- **Special:DoubleRedirects** page listing all double redirects
- **Special:BrokenRedirects** page listing redirects to nonexistent articles
- Maintenance script to fix double redirects after bulk moves
