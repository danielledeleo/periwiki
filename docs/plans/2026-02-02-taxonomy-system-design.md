# Taxonomy System Design

Status: **Draft** - Initial brainstorming, not yet implementation-ready.

## Problem

Enable user-driven taxonomies for grouping and discovering content. Users should be able to create their own classification systems without rigid system-imposed structures.

## Design Decisions

### Terminology: Categories, not Tags

"Tag" is semantically weak - what does `tag: red` mean? "Category" encourages editorial thinking about classification. `Category:90s_Foosball_Tournaments` signals intent and structure.

### Declaration: Frontmatter-based

Categories declared explicitly in article frontmatter, not inline markup:

```
---
categories:
  - 90s_Foosball_Tournaments
  - Competitive_Sports
---
```

This keeps the "magic" controlled and explicit.

### Category Pages: Hybrid Content

Category pages (e.g., `/Category:90s_Foosball_Tournaments`) are real wiki pages that can contain curated content AND auto-generated member lists.

- Authors can add introductory text, context, or curation notes
- A widget/template injects the dynamic member list
- If no page exists, a default view shows just the member list

### Scope: Hierarchical Categories (Option 2)

Categories can have parent categories, enabling:
- `Category:Sports` → `Category:Foosball` → `Category:90s_Foosball_Tournaments`
- Queries like "all articles in Sports or its subcategories"

**Not in scope:** Arbitrary named relationships between pages (graph database territory). If this becomes necessary, revisit the architecture.

### Re-rendering Considerations

When an article's categories change, category pages need re-rendering. With hierarchical categories, parent category pages may also need updates.

Options to explore:
1. **Eager re-render** - Update all affected category pages immediately
2. **Lazy invalidation** - Mark category pages stale, re-render on next view
3. **Periodic rebuild** - Category member lists rebuilt on schedule

The existing render queue infrastructure may be adaptable.

## Implementation Approach (TBD)

### Data Model

- Extend `Frontmatter` struct with `Categories []string`
- Join table: `article_category(article_id, category_url)`
- Optional: `category_parent(child_url, parent_url)` for hierarchy

### Widget System

Needs design. Questions:
- Syntax for embedding dynamic content in pages?
- How do widgets declare render dependencies?
- Can widgets be used outside category pages?

## Prior Art Reviewed

- **MediaWiki** - `[[Category:Foo]]` inline syntax, auto-generated category pages, hierarchical via subcategory declarations
- **Drupal** - Vocabulary-based taxonomies, terms with hierarchy support, flexible but complex

## Open Questions

1. How are category hierarchies declared? In the category page's frontmatter?
2. Widget syntax - `{{members}}` template style? Something else?
3. Should empty categories (no members) show in any index?
4. How do we handle category page creation - auto-create stubs, or require explicit creation?

## Next Steps

1. Design the frontmatter extension for categories
2. Design the widget/template system (may be its own feature)
3. Prototype flat categories first, add hierarchy as a second phase
