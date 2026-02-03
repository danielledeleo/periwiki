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

### Namespace Model: Allowlist, Not Blocklist

All `Foo:Bar` URLs are reserved by default. The colon is a namespace delimiter and is system-controlled. Only explicitly allowed namespaces (e.g., `Category:`, `Talk:`, `User:`) will be openable for user article creation. This is not yet implemented - currently all non-`Special:` namespaced URLs return 404.

The route `/wiki/{namespace}:{page}` catches all colon URLs. `Special:` is handled by the special page system. Future namespaces will be added to an allowlist as needed.

### Re-rendering Considerations

When an article's categories change, category pages need re-rendering. With hierarchical categories, parent category pages may also need updates.

Options to explore:
1. **Eager re-render** - Update all affected category pages immediately
2. **Lazy invalidation** - Mark category pages stale, re-render on next view
3. **Periodic rebuild** - Category member lists rebuilt on schedule

The existing render queue infrastructure may be adaptable.

## Widget System Design

### Syntax

Widgets are invoked with double-brace delimiters: `{{ ... }}`. The inner content is NestedText. The top-level key is the widget name, its value is the parameters.

Three invocation patterns:

**Inline** - widget name + string value:
```
{{ Flag: Canada }}
```

**Block with NestedText params** - widget name + structured data:
```
{{
Infobox:
    type: fashion designer
    name: Valentino
    birth_date: 1932-05-11
    birth_place: [[Voghera]], Lombardy
}}
```

**Block with body content** - widget name + params + raw markdown body:
```
{{ Accordion: plain }}
This gets inserted into the template.
- markdown
- support
{{ /Accordion }}
```

### Expansion Model: Macro Expansion

Widgets produce markdown, not HTML. All widgets are expanded via outermost-first macro expansion (normal-order reduction) before a single markdown-to-HTML compilation step.

- **Outermost-first**: Resolve the outermost `{{ }}` invocations each pass, splice results back in, repeat until no widgets remain or limits are hit.
- This is the same strategy used by MediaWiki templates, TeX, m4, and Lisp macros.
- Guarantees termination in more cases than innermost-first (avoids expanding widgets that may be discarded by outer templates).

**Limits (both enforced):**
- **Depth limit**: ~5-10 (how deep the expansion stack goes per pass iteration)
- **Total expansion count**: ~200 (total widget invocations across the entire page)
- Hit either limit → stop expanding, emit error marker in output.

### Widget Templates: Two Sources

1. **Theme-provided** (disk files) - ship with the theme, admin-controlled
2. **User-defined** (wiki pages) - created by wiki authors, stored as `Template:WidgetName` pages

Different calling conventions to avoid namespace overlap (exact syntax TBD).

### CSS

For now, widget CSS lives in the global theme stylesheet. Future option: CSS `@scope` for component-scoped styles (Baseline as of late 2025, all major browsers).

## Implementation Approach (TBD)

### Data Model

- Extend `Frontmatter` struct with `Categories []string`
- Join table: `article_category(article_id, category_url)`
- Optional: `category_parent(child_url, parent_url)` for hierarchy

## Prior Art Reviewed

- **MediaWiki** - `[[Category:Foo]]` inline syntax, auto-generated category pages, hierarchical via subcategory declarations. Template CSS is a mess (Common.css, TemplateStyles, inline).
- **Drupal** - Vocabulary-based taxonomies, terms with hierarchy support, flexible but complex
- **Web Components** - Shadow DOM scoped styles; CSS `@scope` achieves same effect without JS

## Open Questions

1. How are category hierarchies declared? In the category page's frontmatter?
2. Exact calling convention for theme vs user-defined widgets
3. Should empty categories (no members) show in any index?
4. How do we handle category page creation - auto-create stubs, or require explicit creation?
5. NestedText strictness inside markdown - strict or relaxed on whitespace? Needs prototyping.

## Next Steps

1. Prototype the widget parser as a Goldmark extension (inline + block)
2. Implement macro expansion with depth/count limits
3. Design the frontmatter extension for categories
4. Prototype flat categories first, add hierarchy as a second phase
