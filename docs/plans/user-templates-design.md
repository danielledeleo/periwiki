# User-Defined Templates Design

## Overview

A template system enabling wiki-defined content types, widgets, and layouts. Templates are stored as special articles (`Template:Name`) and invoked by other articles via frontmatter or inline syntax.

**Goals:**
- Infobox widgets (structured data cards like Wikipedia)
- Reusable content blocks (boilerplate, navigation boxes)
- Dynamic content types (recipes, profiles, custom layouts)
- Replace hardcoded templates (homepage becomes an article invoking `Template:Homepage`)

**Non-goals (for now):**
- User-authored templates (admin-only initially)
- Runtime template expansion (preprocessing only)
- Per-template JavaScript

## Core Concepts

### Template Tiers

| Tier | Complexity | Invocation | Use Case |
|------|-----------|------------|----------|
| Basic | Simple params | Inline shortcode `{:Name: "a" "b"}` | Formatters, lookups, small insertions |
| Widget | Structured data | Fenced block `{:Name:}...{/:Name:}` | Infoboxes, callouts, data tables |
| Page | Full layout | Frontmatter `template: Name` | Content types, custom layouts |

### Execution Model

**Preprocessing:** Template expansion happens at article save time. Output is stored in the revision's rendered content. Templates cannot access runtime context (current user, current date).

**Hybrid mode (special pages):** Special pages like `Special:Sitemap` use templates for layout but populate data at request time. The template defines *how* to display; the handler provides *what* to display.

```
Special:Sitemap request
     |
     v
Handler gathers runtime data (sitemap entries)
     |
     v
Load Template:SpecialSitemap (or fallback to hardcoded)
     |
     v
Execute template with handler data as context
     |
     v
Response (not stored)
```

## Article Frontmatter

Article frontmatter serves two distinct purposes that interact with the template system:

### Standard Metadata vs Template Parameters

**Standard metadata** — Fields that apply to all articles regardless of template:
```yaml
---yaml
title: Valentino Garavani
description: Italian fashion designer
tags: [fashion, designer, italian]
created: 2026-01-15
---
```

**Template parameters** — Fields specific to the invoked page template:
```yaml
---yaml
template: Biography
# Standard metadata
tags: [fashion, designer]
# Template-specific parameters
birth_date: 1932-05-11
nationality: Italian
occupation: Fashion designer
---
```

The rendering pipeline:
1. Parses all frontmatter fields
2. Validates template-specific fields against the template schema
3. Passes both to the template (standard fields available as `.Meta.*`, template params as `.Params.*` or top-level)

### Relationship to Tagging

A tagging system builds on template infrastructure but requires additional components:

**What templates provide:**
- Frontmatter parsing (tags declared as `tags: [a, b, c]`)
- Rendering (page templates can output tag links)
- Tag page layout (`Template:TagPage` for `Tag:*` special pages)

**What tagging requires beyond templates:**
- Tag index table (mapping tags → articles for efficient queries)
- Tag page handler (special page that queries articles by tag)
- Editor integration (autocomplete, validation)

**Implicit tags from templates:** Templates could declare tags that are automatically applied:
```yaml
# Template:Recipe frontmatter
implicit_tags: [recipe]
```
Any article using `Template:Recipe` inherits the `recipe` tag.

### Plain Articles (No Template)

Articles without a `template:` field still have frontmatter for metadata. The system provides a default rendering that:
- Renders the markdown body
- Displays standard metadata (tags, dates) in a consistent location
- Could be customizable via a `Template:DefaultArticle` in the future

## Template Authoring

Templates use Go's `html/template` syntax with a restricted function set.

### Template Page Structure

```yaml
---yaml
type: widget
visibility: public
config_languages: [yaml, kdl]
params:
  name:
    type: string
    required: true
    description: "Person's full name"
  born:
    type: date
    format: "2006-01-02"
  image:
    type: string
    pattern: "^File:.*"
  awards:
    type: list
    items: { type: string }
css: |
  [data-template="infobox"] {
    border: 1px solid var(--border-color);
    padding: 1rem;
    float: right;
    width: 300px;
  }
---
<aside data-template="infobox">
  <h3>{{.name}}</h3>
  {{if .born}}<p>Born: {{.born | formatDate "January 2, 2006"}}</p>{{end}}
  {{if .image}}<img src="{{.image}}" alt="{{.name}}">{{end}}
  {{if .awards}}
  <ul>
    {{range .awards}}<li>{{.}}</li>{{end}}
  </ul>
  {{end}}
</aside>
```

### Frontmatter Language Tags

Templates and articles can specify their frontmatter language:

```
---yaml
title: My Article
template: Homepage
---

---kdl
title "My Article"
template "Homepage"
---
```

## Invocation Syntax

*Note: Syntax is placeholder, to be refined during implementation.*

### Basic (inline)

```markdown
The designer {:FullName: "Valentino" "Garavani"} was born in Italy.
```

### Widget (multi-line)

```markdown
{:InfoBox:}
name: Valentino Garavani
born: 1932-05-11
awards:
  - Legion of Honour
  - CFDA Award
{/:InfoBox:}
```

With explicit config language:

```markdown
{:InfoBox:kdl}
name "Valentino Garavani"
born 1932-05-11
awards "Legion of Honour" "CFDA Award"
{/:InfoBox:}
```

### Page (frontmatter)

```yaml
---yaml
template: Homepage
featured_article: Fashion_design
show_recent: true
---
Optional body content accessible as {{.Body}}
```

### Template Pinning

Invokers can pin to a specific template revision:

```markdown
{:InfoBox@r42:}
name: Valentino
{/:InfoBox:}
```

Unpinned invocations use the latest template version and trigger re-renders when the template updates.

## Template CSS

Templates can include scoped CSS that gets collected and deduplicated at build time.

### Scoping via Data Attributes

Templates output scoped markup:

```html
<aside data-template="infobox">...</aside>
```

CSS uses attribute selectors:

```css
[data-template="infobox"] {
  display: grid;
  gap: 1rem;
}
[data-template="infobox"] .field {
  border-bottom: 1px solid var(--border-color);
}
```

### Parameterized Styles

CSS custom properties bridge template parameters to styles:

```yaml
params:
  color:
    type: string
    default: "var(--theme-accent)"
css: |
  [data-template="callout"] {
    border-left: 4px solid var(--callout-color);
    background: color-mix(in srgb, var(--callout-color) 10%, transparent);
  }
---
<aside data-template="callout" style="--callout-color: {{.color}};">
  {{.Body}}
</aside>
```

### Build Pipeline

1. Collect CSS from all templates used by the article
2. Deduplicate (same template used multiple times)
3. Inject as single `<style>` block in page `<head>`

Templates can reference theme variables (`var(--border-color)`) for consistency.

## Security Model

### Trust Tiers

| Author | Can Create | Function Access | Output Treatment |
|--------|------------|-----------------|------------------|
| Admin | Any template | Extended allowlist | Trusted (no sanitization) |
| User (future) | Basic/Widget | Restricted allowlist | Sanitized via Bluemonday |

### Template Visibility

- **Public:** Invokable by any article or template
- **Private:** Only invokable by system or same-namespace templates

### Function Allowlists

**User templates (restricted):**
- String: `upper`, `lower`, `trim`, `replace`, `split`, `join`
- Date: `formatDate`
- Collections: `len`, `index`, `first`, `last`
- Logic: `eq`, `ne`, `lt`, `gt`, `and`, `or`, `not`

**Admin templates (extended):**
- All user functions, plus:
- Data queries: Article lookups, category listings
- Template helpers: `safeHTML`, `safeJS`, `safeCSS`
- System: Config values, feature flags

### Resource Limits

- Output size cap (e.g., 1MB rendered)
- Execution timeout (e.g., 500ms)
- Recursion depth limit for nested templates (e.g., 10 levels)

### HTML Sanitization (User Templates)

User template output passes through Bluemonday with a restrictive policy.

**Blocked elements:** `script`, `style`, `link`, `meta`, `base`, `iframe`, `frame`, `object`, `embed`, `applet`, `form`, `svg`

**Blocked attributes:** `on*` event handlers, `style` (or sanitize CSS properties)

**Blocked URL schemes:** `javascript:`, `data:` (except `data:image/*`)

**Attack vectors to test:**
- Direct script injection
- Event handler attributes (`onerror`, `onmouseover`, etc.)
- JavaScript/data URLs in href/src
- SVG with embedded scripts
- Meta refresh redirects
- Base tag hijacking
- Form injection for phishing
- CSS-based data exfiltration

### Template Injection Prevention

Parameters are always data, never parsed as template source:

```go
// SAFE: parameters are data
tmpl.Execute(w, map[string]string{"Name": userInput})

// DANGEROUS: never do this
templateSource := "Hello, " + userInput + "!"
```

Additional safeguards:
- Template names in `{{template .Name}}` must be from allowlist, not parameters
- No re-parsing of template output
- Nested template calls use the invoking article's trust level

## Rendering Pipeline

```
Article Content
     |
     v
1. Parse Frontmatter --- Extract page template, metadata
     |
     v
2. Template Expansion --- Resolve {:Template:} invocations (recursive, depth-limited)
     |
     v
3. Markdown Render --- Goldmark + extensions (wikilinks, footnotes)
     |
     v
4. Sanitization --- Bluemonday (policy based on template trust)
     |
     v
5. CSS Collection --- Gather and dedupe template CSS
     |
     v
Stored in Revision (rendered content + build metadata)
```

### Error Handling

| Error | Behavior |
|-------|----------|
| Template not found | Render placeholder: `[Template:Name not found]` |
| Template is private | Render placeholder: `[Template:Name is private]` |
| Schema validation failed | Render with details: `[Template:Name: missing required param 'title']` |
| Circular reference | Render placeholder: `[Circular template reference detected]` |
| Depth limit exceeded | Render placeholder: `[Template nesting too deep]` |
| Timeout/size limit | Render placeholder: `[Template:Name exceeded resource limits]` |

**Publish blocking:** Errors render visibly but prevent save/publish. Author must fix template invocations before committing.

## Storage Schema

### Article Type Field

```sql
ALTER TABLE article ADD COLUMN type TEXT NOT NULL DEFAULT 'article';
-- Values: 'article', 'template'
```

### Template Metadata

```sql
CREATE TABLE template_meta (
    article_id INTEGER PRIMARY KEY REFERENCES article(id),
    template_type TEXT NOT NULL,        -- 'basic', 'widget', 'page'
    visibility TEXT NOT NULL DEFAULT 'public',
    config_languages TEXT NOT NULL,     -- JSON array: ["yaml", "kdl"]
    schema TEXT NOT NULL,               -- JSON schema definition
    trust_level TEXT NOT NULL           -- 'admin', 'user'
);

CREATE INDEX idx_template_meta_visibility ON template_meta(visibility);
```

### Build Metadata

Store template versions used at build time in the revision table:

```sql
ALTER TABLE revision ADD COLUMN build_metadata TEXT; -- JSON
```

```json
{
  "built_at": "2026-01-24T18:27:49Z",
  "templates": {
    "Template:Infobox": { "revision": 15, "pinned": false },
    "Template:Citation": { "revision": 8, "pinned": true }
  }
}
```

### Schema API Route

`Template:Name/Schema` returns the parsed schema as JSON for tooling and editor support.

### Usage Statistics

Template edit view shows usage stats per revision:
- How many articles reference each template version
- Which are pinned vs. floating (latest)
- Link to `Template:Name/Dependents` for full list

## Editing Safeguards

- **Breaking change detection:** Warn when saving a template if required params were added (will break existing invocations)
- **Dependent article list:** `Template:Name/Dependents` shows articles invoking this template
- **Re-render trigger:** Queue re-render of dependent articles when template changes (unpinned invocations only)
- **Deletion safeguards:** Show dependent count before deletion; options: cancel, force delete, or deprecate

## Implementation Phases

### Phase 0: Prerequisites
- Content re-render queue system
- Article type field migration

### Phase 1: Foundation
- `Template:` namespace recognition
- Template storage (`template_meta` table)
- Schema parsing (frontmatter extraction)
- Basic template execution (no nesting)
- Admin-only authorship
- Single config language (YAML)

### Phase 2: Invocation Syntax
- Inline basic templates `{:Name: args}`
- Multi-line widget syntax `{:Name:}...{/:Name:}`
- Goldmark extension for parsing invocations
- Schema validation on invocation
- Error placeholders

### Phase 3: Page Templates
- Frontmatter `template:` directive
- Page layout templates
- Special page hybrid mode (template + handler data)
- Homepage as article with `Template:Homepage`

### Phase 4: Dependency Tracking
- Build metadata storage (JSON column)
- Template revision pinning (`@r42`)
- Re-render triggers on template update
- Deletion safeguards with dependent count
- Usage stats in template editor

### Phase 5: Template CSS
- CSS section in template frontmatter
- Scoped output via `data-template` attributes
- CSS collection and deduplication at build
- CSS custom properties for parameterized styles

### Phase 6: Advanced Features
- Nested template invocation (with depth limits)
- Additional config languages (KDL)
- Template visibility (public/private)
- `Template:Name/Schema` API route
- `Template:Name/Dependents` view

### Phase 7: User Templates (Future)
- Tiered function allowlists
- Output sanitization for user-authored templates

## Testing Strategy

### Unit Tests

- Schema parsing and validation
- Parameter type coercion (string, date, list)
- Function allowlist enforcement
- Resource limit enforcement (timeout, size, depth)

### Integration Tests

- Full rendering pipeline with templates
- Nested template resolution
- Build metadata storage and retrieval
- Re-render queue triggers

### Security Tests

**Template injection:**
- Parameters containing `{{` sequences
- Template names from user input
- Nested template privilege escalation

**HTML injection (user templates):**
- Direct `<script>` injection
- Event handler attributes (`onerror`, `onmouseover`, `onload`)
- JavaScript URLs (`href="javascript:..."`)
- Data URLs with HTML content
- SVG with embedded scripts
- Meta refresh redirects
- Base tag hijacking
- Form injection
- CSS-based exfiltration (`url()` in styles)
- Attribute breakout via parameters

**Resource exhaustion:**
- Deeply nested templates (depth limit)
- Large output generation (size limit)
- Slow template execution (timeout)
- Circular template references

### Regression Tests

- Existing article rendering unchanged
- WikiLink and footnote extensions still work
- Special pages render correctly

## Open Research

- [ ] **MediaWiki template system:** How does MediaWiki handle template dependencies, deletion, and breaking changes? <!-- Research before Phase 4 -->
- [ ] **Hugo text/template patterns:** Best practices for template function design and security <!-- Research before Phase 1 -->
- [ ] **KDL parsing in Go:** Available libraries, maturity, edge cases <!-- Research before Phase 6 -->
- [ ] **Invocation syntax refinement:** Finalize `{:Template:}` syntax, consider alternatives <!-- Ongoing through Phase 2 -->
- [ ] **Tagging system design:** Index storage, tag page handlers, editor integration, implicit tags from templates <!-- After Phase 3 -->
