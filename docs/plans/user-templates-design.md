# Template & Widget System Design

Status: **Draft — all decisions resolved, ready for implementation planning**

## Overview

A template system enabling wiki-defined content types and widgets. Two layers:

- **Core widgets** — Go `html/template` files on disk (`templates/_widgets/`). Trusted code that produces HTML. Each widget does one thing well.
- **User templates** — Wiki articles under the `Template:*` namespace. Provide convenience wrappers, parameter defaults, and documentation around core widgets. Can also be plain markdown for simple transclusion.

**Goals:**
- Infobox widgets (structured data cards)
- Reusable content blocks (boilerplate, navigation, notices)
- Dynamic content types (recipes, profiles) via core widget + user template combos

**Non-goals:**
- Raw HTML authoring through the wiki editor
- Per-template JavaScript
- Page-level layout templates (TODO: revisit later)

## Architecture

### Trust Model

Trust is **structural**, not metadata:

| Layer | Location | Trust | Produces HTML? |
|-------|----------|-------|----------------|
| Core widgets | `templates/_widgets/*.html` | Trusted (code) | Yes — Go `html/template` |
| User templates | `Template:*` wiki articles | Untrusted (content) | No — markdown + param mapping |
| Articles | Regular wiki articles | Untrusted (content) | No — markdown with invocations |

Core widgets are the **only** source of HTML in the template system. User templates are pure data mapping — they cannot execute code or emit raw HTML. This eliminates the need for function allowlists, output sanitization of template results, or template injection prevention at the user template layer.

No `article.type` column or `template_meta` table. The `Template:*` namespace prefix identifies templates. All template configuration lives in frontmatter under the `templatedef:` scope.

### Core Widget Philosophy

Core widgets follow a **"do one thing well"** pattern. Each widget is generic and focused — it handles layout and structure, not domain semantics. User templates provide the domain-specific interface on top.

Example: `core:infobox` renders a title + list of key-value rows. It doesn't know what "born" or "occupation" means. `Template:Person` maps domain fields to generic rows.

All Go `FuncMap` functions from `templater/templater.go` are available to core widgets (text formatting, URL builders, namespace helpers). Core widgets own all error handling — Periwiki provides helper functions, widget authors handle variance/malformed data.

### Rendering Pipeline

Template expansion is a **Goldmark extension** (block parser + renderer), integrated into the AST. Trusted HTML (core widgets, TOC) bypasses Bluemonday via the **post-sanitization placeholder pattern** (see below).

```
Article markdown
  |
  v
Goldmark parse (builds AST, all extensions including template extension)
  |
  |-- Markdown syntax     --> standard AST nodes
  |-- [[Wiki links]]      --> WikiLink nodes (existing extension)
  |-- [^Footnotes]        --> Footnote nodes (existing extension)
  |-- {{Template|params}} --> TemplateNode (NEW extension)
  |
  v
Goldmark render (walks AST, all renderers)
  |
  |-- TemplateNode encountered:
  |       |
  |       |-- Core widget (core:name)?
  |       |     Parse fenced params, stash in expansion context
  |       |     Emit <div data-pw-widget="N"></div> placeholder
  |       |
  |       |-- User template (Template:*)?
  |       |     Fetch article body as raw string
  |       |     Substitute {{ .param }} references (string replacement)
  |       |     Re-parse result through Goldmark (recursive)
  |       |
  |       |-- Plain transclusion?
  |             Fetch Template:* body as raw string
  |             Re-parse through Goldmark (recursive)
  |
  |-- TOC: emit <div data-pw-toc></div> before first h2
  |
  v
HTML (with data-pw-* placeholders for trusted content)
  |
  v
Bluemonday sanitization
  |  Strips unsafe HTML from user-authored content.
  |  Preserves data-pw-* placeholders (allowlisted).
  |  Does NOT see core widget output or TOC — they haven't been expanded yet.
  |
  v
Post-sanitization expansion (single goquery pass)
  |
  |-- data-pw-toc    → scan headings, build TOC tree, replace placeholder
  |-- data-pw-widget → look up stashed params by index, execute Go template, replace
  |-- Sentinel sweep → verify no data-pw-* attributes remain in output
  |
  v
Final HTML (stored in revision)
```

**Why a Goldmark extension, not a pre-processing pass:** Block template bodies contain markdown, wikilinks, and footnotes. As child nodes of the TemplateNode in Goldmark's AST, they are parsed natively by all extensions in a single pass. A pre-processor would need to call Goldmark on block bodies separately, creating nested Goldmark invocations with disconnected footnote numbering and heading IDs.

**Core widgets emit placeholders, not HTML.** During Goldmark rendering, `core:*` invocations stash their parsed params in a numbered slice and emit an inert `<div data-pw-widget="N"></div>`. The actual Go template execution happens after Bluemonday, so widget HTML is never sanitized. User templates and plain transclusions are expanded during Goldmark rendering (they produce markdown that is re-parsed), so their content *is* sanitized — which is correct, since it's user-authored.

**State isolation across re-parses.** User template expansion creates new Goldmark parse contexts. Footnote counters and heading IDs will collide if each sub-parse starts from zero. An `ExpansionContext` struct threads shared state (visited-template set, depth counter, widget param stash, footnote counter, heading ID set) across all recursive re-parses within a single article render. Implementation will try global counters first, namespace prefixing as fallback.

**Parser/renderer separation.** The template Goldmark extension's block parser must be a pure syntax recognizer with no side effects. The parser creates `TemplateNode` AST nodes; the renderer fetches template bodies and performs expansion. This is critical because the `LinkExtractor` needs to register the parser (to find template references in markdown) without triggering any rendering or database access.

**Fallback architecture.** If the Goldmark extension approach proves too constrained (re-parse state isolation, code fence interactions), the fallback is a standalone lexer/parser using island grammar techniques — a two-mode lexer that handles markdown structure (code fences) and template syntax in a unified pass. This would replace Goldmark's template extension but not Goldmark itself (Goldmark still renders the expanded markdown). Validated against Goldmark's CommonMark test suite.

### Relationship to Existing Infrastructure

These systems are already in place (Phase 0 prerequisites from the original design):

- **Render queue** — Priority-based with interactive/background tiers, article deduplication
- **Stale content detection** — Boot-time template hash check, lazy re-rendering. `HashRenderTemplates()` must be extended to also hash `templates/_widgets/` (currently only hashes `templates/_render/`).
- **Namespace routing** — `Foo:Bar` URLs route through `NamespaceHandler`; `Template:*` namespace needs registration
- **Frontmatter parsing** — NestedText parser, `frontmatter` BLOB column on article table
- **Special pages registry** — Dynamic handler registration

## Invocation Syntax

### Named Parameters (fenced NestedText)

The primary calling convention. All invocations use `---` fences for NestedText params:

```markdown
{{Person
---
name: Valentino Garavani
born: May 11, 1932
job:
  title: Fashion designer
  tenure: 1959-present
---
}}
```

No-param invocations omit the fences:

```markdown
{{Stub}}
```

**Positional shorthand is deferred** — not in first pass.

### Core Widget Invocation

The `core:` prefix invokes a disk template directly:

```markdown
{{core:infobox
---
title: Valentino Garavani
accent: #4a7c59
rows:
  - label: Born
    value: May 11, 1932
---
}}
```

This executes `templates/_widgets/infobox.html` with the params as its Go template data context.

### Block Syntax (Templates with Body Content)

Templates can wrap content using matching close tags. The body is rendered as HTML and passed to the widget as `.body`:

```markdown
{{core:callout
---
icon: warning
accent: red
---
}}
**Do not edit** this page without first consulting the admin!

See [[Help:Editing]] for more details.
{{/core:callout}}
```

**Decision:** Matching close tag `{{/Name}}` (not `{{end}}`). Supports unambiguous nesting.

The block body is parsed by Goldmark as child nodes of the TemplateNode (using `parser.HasChildren`). All Goldmark extensions (wikilinks, footnotes, etc.) work natively in the body. At render time, child nodes are rendered to HTML and passed to the core widget as `.body`.

### Param References

User templates use `{{ .dotNotation }}` inside curly braces to reference passed parameters:

- `{{ .name }}` — a specific param
- `{{ .job.title }}` — nested access
- `{{ .body }}` — reserved key for block content (injected by the system)

The `{{ }}` syntax is visually distinct from literal values and familiar (Go templates, Mustache).

**Where param substitution happens:**

| Context | Goldmark parses it? | How `{{ .var }}` is handled |
|---------|--------------------|-----------------------------|
| `---` fenced params | No — raw string | String substitution before parsing |
| User template body | No — fetched as raw string | String substitution before re-parsing |
| Block body (slot content) | Yes — child nodes | **Not supported** — see Scoping below |

### Scoping: No Param Refs in Block Bodies

Block body content is parsed by Goldmark as child nodes. Rather than introducing an inline ParamRefNode extension with complex scoping rules, **block bodies cannot contain `{{ .var }}` references.**

This is not a limitation in practice. If a block invocation appears inside a user template body (which is common), the surrounding template's `{{ .var }}` references are substituted *before* the block is parsed:

```
Template:InfoPage body (raw string in database):

  {{Warning
  ---
  icon: danger
  ---
  }}
  Contact {{ .admin.name }} about {{ .title }}!
  {{/Warning}}

After string substitution (InfoPage params applied):

  {{Warning
  ---
  icon: danger
  ---
  }}
  Contact Jane about Secret Page!
  {{/Warning}}

Goldmark then parses this. The block body is plain text — no {{ }} refs remain.
```

User templates handle all dynamic composition. Article authors writing block bodies directly can only use literal content — which is correct, since articles have no params to reference.

### Pass-Through and Selective Mapping

When a user template invokes another template or core widget, it maps its params explicitly:

**Simple pass-through:**
```
{{core:infobox
---
title: {{ .name }}
accent: #4a7c59
---
}}
```

**Renamed:**
```
{{CreditLine
---
creator: {{ .name }}
period: {{ .job.tenure }}
---
}}
```

**Modified/augmented:**
```
{{CreditLine
---
creator: {{ .name }} and Steve
period: {{ .job.tenure }}
note: Retired in 2008
---
}}
```

**Selective:** Only params explicitly listed in the fenced NestedText are passed. Unlisted params are not forwarded.

String substitution is blind text replacement on the raw template body. After substitution, the result is valid NestedText with all-literal values. This composes naturally across any depth of template nesting.

## Template Interface Schema

A `Template:*` article's frontmatter defines its calling interface under the `templatedef:` scope. This keeps widget metadata separate from normal article frontmatter fields.

### Schema Format (Lean Conventions)

```
---
templatedef:
  widget: infobox
  description: Biographical infobox for people
  params:
    name!: Full name of the person
    born: Birth date
    job:
      title: Job title
      tenure: Active period
  defaults:
    job:
      tenure: present
  example:
    name: Jane Example
    born: January 1, 1970
    job:
      title: Placeholder
---
```

- `name!` — trailing `!` means required
- `awards[]` — trailing `[]` means list type
- Leaf values are descriptions
- Nested maps declare their shape inline
- `defaults:` is a separate section matching the params shape
- `widget:` links to `templates/_widgets/{name}.html`
- `description:` documents the template's purpose
- `example:` provides sample params for the template view page

### Template Types

- **Widget template** — has `templatedef.widget`. Body maps params to a core widget invocation.
- **Plain transclusion** — no `widget` field. Body is markdown with optional `{{ .param }}` substitution, inserted as-is.

## Template Pages and Documentation

### Viewing `/wiki/Template:Person`

When a user navigates to a template page, they see:
- A rendered preview using `templatedef.example` data
- The transcluded content from `Template:Person/Documentation`

### Documentation Subpages

Template documentation lives at `Template:Name/Documentation` — a regular wiki article linked automatically from the template view page. Similar to how talk pages work.

### Talk Pages

Templates have talk pages at `Talk:Template:Person` (not `Template_talk:Person` as MediaWiki does).

## Template CSS

**Decision:** CSS custom property overrides only (Option B).

Base widget styles live in the site theme CSS (`static/main.css`). Core widget templates only emit inline `style="--var: value"` for parameterized aspects. No CSS collection/injection pipeline needed.

```html
<!-- Widget outputs: -->
<aside class="widget-infobox" style="--accent: #4a7c59">...</aside>
```

```css
/* In theme CSS (static/main.css): */
.widget-infobox {
  border: 1px solid var(--border-color);
  border-top: 3px solid var(--accent, var(--theme-accent));
  float: right;
  width: 300px;
}
```

Note: `main.css` already has a `.infobox` class (lines 288-332). The `.widget-infobox` class should either reuse those styles or both classes should be emitted (`class="infobox widget-infobox"`) for backward compatibility with any manually-created infoboxes.

The first-pass implementation includes CSS for aside/infobox styling to support the `core:infobox` widget neatly (similar to MediaWiki infoboxes).

## Full Example

### Core widget: `templates/_widgets/infobox.html`

```html
<aside class="widget-infobox" style="--accent: {{.accent | default "#555"}}">
  <h3>{{.title}}</h3>
  {{range .rows | skipEmpty}}
  <dl><dt>{{.label}}</dt><dd>{{.value}}</dd></dl>
  {{end}}
</aside>
```

Three params: `title` (string), `rows` (list of label/value pairs), `accent` (optional CSS color). Every row value is a plain string. `default` and `skipEmpty` are helper functions provided via the FuncMap.

### User template: `Template:Person` (wiki article)

```markdown
---
templatedef:
  widget: infobox
  description: Biographical infobox for people
  params:
    name!: Full name of the person
    born: Birth date
    died: Death date
    occupation: Primary occupation
  defaults:
    occupation: Unknown
  example:
    name: Jane Example
    born: January 1, 1970
    occupation: Placeholder
---
{{core:infobox
---
title: {{ .name }}
accent: #4a7c59
rows:
  - label: Born
    value: {{ .born }}
  - label: Died
    value: {{ .died }}
  - label: Occupation
    value: {{ .occupation }}
---
}}

{{CreditLine
---
creator: {{ .name }}
---
}}
```

### User template: `Template:CreditLine` (plain transclusion)

```markdown
---
templatedef:
  description: Small italic attribution line
  params:
    creator!: Name of the page creator
    period: Time period
    note: Additional note
---
*Page created by {{ .creator }} ({{ .period }}). {{ .note }}*
```

### User template: `Template:Stub` (simple transclusion, no params)

```markdown
---
templatedef:
  description: Notice for incomplete articles
---
*This article is a stub. You can help by expanding it.*
```

### Article using the templates

```markdown
---
title: Valentino Garavani
---
{{Person
---
name: Valentino Garavani
born: May 11, 1932
occupation: Fashion designer
---
}}

Valentino Garavani is an Italian fashion designer known for
his [[Valentino (brand)|brand]].

{{Stub}}
```

### Resolution trace

```
1. Goldmark parses article, creates AST with two TemplateNodes
2. Renderer hits TemplateNode "Person"
3. Fetch Template:Person body as raw string
4. Parse caller params: {name: "Valentino Garavani", born: "May 11, 1932", occupation: "Fashion designer"}
5. Apply templatedef.defaults (occupation already provided, skip)
6. String-substitute {{ .name }}, {{ .born }}, {{ .occupation }} in template body
7. Result contains {{core:infobox ---...---}} and {{CreditLine ---...---}}
8. Re-parse result through Goldmark
9. Goldmark creates two new TemplateNodes: core:infobox and CreditLine
10. Renderer hits core:infobox — execute templates/_widgets/infobox.html with resolved params → HTML
11. Renderer hits CreditLine — fetch template, substitute {{ .creator }}, re-parse
12. CreditLine body is plain markdown, no more {{ }} invocations — render to HTML
13. Back in outer document: renderer hits TemplateNode "Stub"
14. Fetch Template:Stub body — plain markdown, no params — re-parse and render
15. All TemplateNodes resolved. Final HTML assembled.
```

### Final rendered HTML

```html
<aside class="widget-infobox" style="--accent: #4a7c59">
  <h3>Valentino Garavani</h3>
  <dl><dt>Born</dt><dd>May 11, 1932</dd></dl>
  <dl><dt>Occupation</dt><dd>Fashion designer</dd></dl>
</aside>

<p><em>Page created by Valentino Garavani (). </em></p>

<p>Valentino Garavani is an Italian fashion designer known for
his <a href="/wiki/Valentino_(brand)">brand</a>.</p>

<p><em>This article is a stub. You can help by expanding it.</em></p>
```

**Note:** The CreditLine output shows `()` and a trailing space because `period` and `note` were not passed — blind string substitution replaces absent params with empty strings, leaving orphaned punctuation. This is a known limitation of the no-conditionals design. See "Design Decisions to Finalize" for mitigation options.

## Security

### Structural Safety

The two-layer architecture provides security by construction:

1. **Core widgets** are Go templates on disk — reviewed code, not user input
2. **User templates** are content, not code — they map params and produce markdown
3. **Parameters are always data** — passed as `map[string]any` to Go templates, never parsed as template source
4. **No raw HTML from editor** — user-authored content goes through Goldmark + Bluemonday

### Sanitization: Post-Sanitization Placeholder Pattern

Core widget HTML **never passes through Bluemonday**. Instead, the Goldmark extension emits inert placeholder elements (`<div data-pw-widget="N"></div>`) that Bluemonday preserves via a minimal allowlist addition. After sanitization, a single expansion pass replaces placeholders with the real widget HTML.

This same pattern applies to **TOC injection** (`<div data-pw-toc></div>`), establishing `data-pw-*` as a general mechanism for trusted post-sanitization content.

**Why this is safe:**

1. **Goldmark is the gate.** Without `html.WithUnsafe()` (which is NOT enabled), Goldmark strips all raw HTML from markdown input. A user writing `<div data-pw-widget="0"></div>` in their article gets it stripped before Bluemonday ever runs. The only way a `data-pw-*` element enters the HTML stream is if a Goldmark extension emits one.
2. **Placeholder params are opaque.** The placeholder carries only an integer index into a side-channel slice. No user-controlled data in the attribute value.
3. **Final sentinel sweep.** After expansion, the output is checked for any remaining `data-pw-*` attributes. Unexpanded placeholders are stripped and logged. The sentinel is an internal implementation detail that never reaches the browser.

**Invariant:** The placeholder system depends on Goldmark not passing raw HTML through. Enabling `html.WithUnsafe()` would break this assumption and must never be done without redesigning the sanitization strategy.

**Bluemonday policy change:** Add `data-pw-widget` and `data-pw-toc` attributes to the allowlist on `div` elements. This is the only sanitizer modification needed.

```go
bm.AllowAttrs("data-pw-widget").
    Matching(regexp.MustCompile(`^[0-9]+$`)).
    OnElements("div")
bm.AllowAttrs("data-pw-toc").
    Matching(regexp.MustCompile(`^$`)).
    OnElements("div")
```

`data-pw-widget` requires a non-empty integer index (references a stashed param set). `data-pw-toc` carries no value (empty string only).

### Template Injection Prevention

- Unescaped `{{` in param values is a parse error. Callers must use `\{{` for literal braces. This prevents template injection through variables — param values are always data, never code.
- Template names in invocations must resolve to known articles or core widgets
- Circular references detected via a visited set in the `ExpansionContext`. The error placeholder includes the full cycle chain for debugging.
- Core widgets MUST use Go `html/template` (not `text/template`) — auto-escaping is a security invariant since user-supplied param values flow into widget HTML output.

### Resource Limits

- **Recursion depth:** 100 (configurable, matching the transitive invalidation BFS depth cap)
- **Output size cap:** TBD
- **No execution timeout needed** — user templates are substitution only, core widgets are simple Go templates

## Error Handling

| Error | Behavior |
|-------|----------|
| Template not found | Render placeholder: `[Template:Name not found]` |
| Core widget not found | Render placeholder: `[core:name not found]` |
| Missing required param | Render warning: `[Template:Name: missing required param 'title']` |
| Invalid param | Render warning with details |
| Circular reference | Render placeholder with chain: `[Circular reference: Template:A → Template:B → Template:A]` |
| Depth limit exceeded | Render placeholder: `[Template nesting too deep (limit 100, at Template:Name)]` |

Error placeholders must be visually distinct. All placeholders emit `<span class="pw-template-error">...</span>` with dedicated CSS styling (red text, dashed border). Errors baked into stored HTML persist until the article is re-rendered — without dependency tracking (Phase 6), there is no automatic mechanism to fix articles when a missing template is later created.

Core widgets handle all error cases internally — the system provides helper functions, widget authors decide how to handle variance.

## Dependency Tracking

Template dependencies reuse the existing `ArticleLink` table and backlink infrastructure.

### Storage

Template references are stored in `ArticleLink` alongside wikilinks. The `target_slug` uses the full namespaced slug: `Template:Person`. A `{{Person}}` invocation produces target slug `Template:Person`; a `[[Template:Person]]` wikilink produces the same slug. Both are visible in one query.

`core:*` widget references are NOT tracked in `ArticleLink` — core widget changes are detected by the staleness hash (`HashRenderTemplates` hashing `templates/_widgets/`).

### Extraction

Extend `LinkExtractor` to include the template block parser in its Goldmark instance. A single AST walk extracts both `WikiLink` nodes and `TemplateNode` nodes. The parser is side-effect-free (see "Parser/renderer separation" above), so using it in the lightweight extractor is safe.

Only direct invocations are extracted from markdown source. Transitive dependencies (templates used by templates) are tracked by each template article's own link row, not by the consuming article.

### Invalidation

Template edits must trigger invalidation on every save (not just creation like wikilinks). The `updateLinks` code path changes:

- `PreviousID == 0` → fire `invalidateBacklinkers` (existing: turns redlinks blue)
- `isTemplateURL(article.URL)` → fire transitive invalidation (new: re-renders all dependent articles)

**Transitive invalidation** uses depth-limited BFS: query `SelectBacklinks("Template:CreditLine")`, for each result that is itself a template recurse, queue re-renders for all non-template leaf articles. Visited set prevents cycles. Depth cap matches the rendering recursion limit.

### Known Limitation: Wikilinks Inside Template Bodies

Wikilinks inside template bodies are tracked as outgoing links of the *template article*, not of articles that transclude it. The backlinks page for a target shows `Template:Person` but not every article using `{{Person}}`. This mirrors MediaWiki's transclusion behavior. Rendering is unaffected — the existence checker runs at render time and correctly colors links.

## Implementation Phases

### Phase 0: Prerequisites (DONE)

- [x] Content re-render queue with priority tiers
- [x] Stale content detection and lazy re-rendering
- [x] Namespace routing (`NamespaceHandler`)
- [x] NestedText frontmatter parsing
- [x] Special pages registry

### Phase 1: Core Widget Rendering

- Register `Template:*` namespace in `NamespaceHandler`
- `Template:*` articles: create, edit, view (same as regular articles)
- Goldmark block parser: recognize `{{core:name ...}}` syntax
  - `---` fence parsing with state machine in `Continue()` to avoid thematic break collision
  - Parser is side-effect-free (pure syntax recognition, no template fetching)
  - Recommended parser priority 80, `CanInterruptParagraph() == true`
- Parse `---` fenced NestedText params
- Execute `templates/_widgets/*.html` with params and FuncMap
- Add `default` and `skipEmpty` helper functions to FuncMap
- `TemplateResolver` interface for template body lookup (enables isolated testing without DB)
- CSS for aside/infobox styling in `static/main.css`
- `.pw-template-error` CSS class for error placeholders
- Error placeholders for missing widgets / bad params (wrapped in `<span class="pw-template-error">`)
- First widget: `core:infobox` (title + rows + accent)
- Add `templates/_widgets/` to `HashRenderTemplates()` glob
- Add `_widgets` exclusion to `meta_test.go` template subdirectory glob test
- Fix existing sanitizer divergence: `testutil.go` must call `server.NewSanitizer()` instead of inline `bluemonday.UGCPolicy()`

### Phase 2: User Template Transclusion

- Goldmark block parser: recognize `{{Name ...}}` syntax (wiki template lookup)
- Fetch `Template:*` article content as raw string via `TemplateResolver`
- `{{ .param }}` string substitution (blind text replacement, single-pass)
- `ExpansionContext` struct threading state across recursive re-parses (visited set, depth counter, widget stash, footnote counter, heading ID set)
- Circular reference detection via visited set in `ExpansionContext`
- Re-parse substituted content through Goldmark (recursive, using shared `ExpansionContext`)
- Block syntax with `{{/Name}}` close tags and `.body` support
- Plain markdown transclusion (no `widget:` field — e.g., `{{Stub}}`)
- Recursive expansion with depth limit (100)
- Error placeholders for missing templates

### Phase 3: Template Interface Schema

- `templatedef:` frontmatter scope (widget, description, params, defaults, example)
- Lean convention-based schema (`!` for required, `[]` for list)
- Required/optional param validation
- Default value injection from `templatedef.defaults`
- Schema-aware error messages
- Template view page: dedicated `template_view.html` template showing:
  - Header banner identifying this as a template, with `templatedef.description`
  - Rendered preview using `templatedef.example` data (or message if absent)
  - Transcluded `/Documentation` subpage (with "create documentation" link if missing)
- Editor guidance: callout box on Template: edit pages explaining `templatedef:` syntax, linking to `Periwiki:Templates` help article

### Phase 4: Template CSS

- Custom property overrides in theme CSS (decided)
- Base widget styles in `static/main.css`
- Widgets emit `style="--var: value"` for parameterized aspects

### Phase 5: Template Documentation & Talk Pages

- `Template:Name/Documentation` subpages (like talk pages)
- `Talk:Template:Person` talk page routing
- Auto-link documentation from template view page
- `Special:AllTemplates` page listing all templates with descriptions
- `Periwiki:Templates` embedded help article documenting syntax and available core widgets

### Phase 6: Dependency Tracking

- Extend `LinkExtractor` to extract template references alongside wikilinks
- Store template refs in `ArticleLink` with `Template:*` target slugs
- Trigger invalidation on every template edit (not just creation)
- Transitive BFS invalidation with visited set and depth cap
- Consider `link_type` column in `ArticleLink` (`'wikilink'` vs `'template'`)
- Reset `SettingLinkBackfillDone` in migration to force re-backfill
- Re-render triggers on template change
- Deletion safeguards
- Usage stats in template editor

### Future / Out of Scope

- Positional param shorthand
- Page-level layout templates (frontmatter `template:` directive)
- Template revision pinning
- Special page hybrid mode (template + handler data)
- Template visibility (public/private)
- Schema API route (`Template:Name/Schema`)
- Conditionals in user templates (Periwiki-specific syntax, not Go template passthrough)
- Inline template invocations (within paragraphs)
- Composite ETag incorporating HTML hash (overlaps with the existing TODO for whole-site cache busting on deploy)

## Testing Strategy

### Unit Tests

- Invocation syntax parsing (fenced NestedText)
- `{{ .param }}` string substitution (flat, nested, modified, selective)
- Recursive expansion with depth limiting
- Schema validation (required params, defaults)
- Error placeholder generation
- Placeholder emission (correct `data-pw-widget` attributes with sequential indices)
- TOC placeholder emission (correct `data-pw-toc` placement before first h2)
- Post-sanitization expansion (placeholder → real HTML replacement)
- Sentinel cleanup (unexpanded placeholders stripped and logged)

### Integration Tests

- Full rendering pipeline with templates + existing extensions (wikilinks, footnotes)
- Core widget execution with various param shapes
- User template wrapping core widget (fetch → substitute → re-parse → execute)
- Block syntax with body content (wikilinks in body, footnotes in body)
- Plain markdown transclusion
- Template invoking another template with param mapping
- Multiple template invocations in one article
- Transitive template invalidation (A uses Template:B which uses Template:C — editing C re-renders A)
- Preview with template expansion (verify templates expand in preview handler)
- Footnote numbering across template boundaries (no collisions)
- Heading IDs across template boundaries (no collisions)

### Round-Trip Tests

- Markdown input → full pipeline → final HTML contains no `data-pw-` attributes
- Articles with zero widgets still render identically (no regression from placeholder machinery)
- Articles with multiple widgets: all placeholders expanded, correct ordering preserved
- Mixed content: widget placeholders interleaved with user-authored HTML that Bluemonday modifies

### Security Tests

- Parameters containing `{{` sequences (must not trigger expansion)
- Deeply nested templates (depth limit enforcement)
- Circular template references
- Large output generation
- Template names from user input (must resolve to known templates only)
- **Placeholder injection:** raw `<div data-pw-widget="0"></div>` in markdown source — verify Goldmark strips it, never reaches output
- **Placeholder spoofing:** `<div data-pw-widget="99999"></div>` with out-of-bounds index — verify sentinel cleanup handles it
- **Attribute injection via markdown:** various attempts to sneak `data-pw-*` attributes through markdown syntax (links, images, etc.)

### Fuzz Tests

Four fuzzing targets, using Go's native `testing.F` framework:

**1. Markdown input fuzzing** (`FuzzRenderPipeline`)
- Seed corpus: normal articles, articles with `{{`, raw HTML, `data-pw-` strings, nested fences
- Fuzz random byte strings as markdown input through the full pipeline
- **Invariants checked on every iteration:**
  - Output contains no `data-pw-` attributes
  - Output contains no `<script>`, `onclick=`, `onerror=`, `javascript:` (Bluemonday guarantees)
  - Output is valid/parseable HTML (no unclosed tags from partial sentinel expansion)
  - Function does not panic

**2. Template param fuzzing** (`FuzzWidgetParams`)
- Seed corpus: normal param maps, params with HTML, params with `{{`, params with `data-pw-`
- Fuzz random strings as param values passed to core widget Go templates
- **Invariants checked on every iteration:**
  - Go `html/template` auto-escaping holds (no unescaped `<` or `>` in output from param values)
  - Widget output is well-formed HTML
  - Function does not panic

**3. String substitution fuzzing** (`FuzzParamSubstitution`)
- Seed corpus: normal param values, values with `{{`, values with `{{ .other }}`, values with `---`, values with raw HTML
- Fuzz random strings as param values through the substitution + re-parse pipeline
- **Invariants checked on every iteration:**
  - Substitution is single-pass (injected `{{ .x }}` does not recursively resolve)
  - Output contains no `data-pw-` attributes
  - Function does not panic

**4. Boundary fuzzing** (`FuzzTemplateInvocationParsing`)
- Seed corpus: valid invocations, unclosed `{{`, unclosed `---` fences, `{{/` without open, deeply nested `{{`
- Fuzz random strings that look like partial template invocations
- **Invariants checked on every iteration:**
  - Parser either produces a valid TemplateNode or cleanly ignores the input
  - No partial placeholders emitted (never a `data-pw-widget` without a matching stashed entry)
  - Function does not panic

### Regression Tests

- Existing article rendering unchanged
- WikiLink and footnote extensions still work
- Frontmatter parsing unaffected
- Special pages render correctly
- TOC rendering unchanged (same output, different insertion mechanism)
  - Note: 7 existing TOC tests will need pipeline adjustment for the placeholder-based insertion
- `skipTOC` frontmatter flow works with new placeholder mechanism

## NestedText and Type Coercion

Per the [NestedText schema philosophy](https://nestedtext.org/en/latest/schemas.html), NestedText deliberately does not interpret data types. Everything parsed is strings, lists of strings, or maps of strings. Type coercion and validation are the responsibility of the consuming application, not the format.

This has direct implications for our schema design:

1. **User template schemas don't encode types.** The template frontmatter declares param names, descriptions, required/optional, and defaults — but not types. NestedText will always produce strings.

2. **Core widgets are the source of truth for types.** The Go template (or a companion schema) knows that `.born` should be parsed as a date and `.port` as an integer. Type coercion happens in Go when the core widget receives params.

3. **Validation is two-stage:**
   - **User template schema:** Does the invocation have the expected param names? Are required params present?
   - **Core widget execution:** Can the string values be coerced to the expected Go types? (Errors here produce rendering warnings.)

4. **No booleans in NestedText.** `required: true` in a schema is the string `"true"`. This favors convention-based schemas (like `name!` for required) over verbose metadata schemas that fight the format.

This aligns with NestedText's design: the format handles structure (nesting, lists, maps), the application handles meaning (types, validation, coercion). See also [NestedText techniques](https://nestedtext.org/en/latest/techniques.html) for validation patterns in Python (Voluptuous, Pydantic) — we'll need a Go equivalent.

## Open Questions

All resolved.

- [x] ~~**Close tag syntax:**~~ `{{/Name}}` matching close tags (supports unambiguous nesting)
- [x] ~~**Param reference syntax:**~~ `{{ .name }}` in curly braces with dot notation. String substitution before re-parsing. No refs in block bodies.
- [x] ~~**Schema format:**~~ Lean conventions (`!` for required, `[]` for list) under `templatedef:` scope
- [x] ~~**CSS approach:**~~ Custom property overrides in theme CSS. Widgets emit `style="--var: value"`.
- [x] ~~**Sanitization strategy:**~~ Post-sanitization placeholder pattern. Core widgets and TOC emit `data-pw-*` placeholders that survive Bluemonday, expanded after sanitization. Final sentinel sweep strips any unexpanded placeholders.
- [x] ~~**Conditionals in user templates:**~~ Deferred to future (Periwiki-specific syntax, not Go templates). User templates stay pure data mapping for now.
- [x] ~~**Tail call optimization:**~~ Deferred to future.
- [x] ~~**Type coercion:**~~ Core widgets handle it. NestedText produces strings, widgets work with strings or coerce as needed.
- [x] ~~**TOC placeholder mechanism:**~~ Best-effort Goldmark emission, fallback to goquery insertion. TOC constructed post-sanitization.
- [x] ~~**Footnote/heading ID isolation:**~~ ExpansionContext with global counters (primary), namespace prefixing (fallback). Standalone parser if Goldmark extension is too constrained.
- [x] ~~**`{{` escape syntax:**~~ `<nowiki>` Goldmark extension. Code fences already protect `{{` via parser priority.
- [x] ~~**`{{` in param values:**~~ Parse error. Use `\{{` for literal braces. Params are data, never code.
- [x] ~~**Empty param rendering:**~~ Core widgets handle conditionals. User templates are pure passthrough.
- [x] ~~**Migration:**~~ No special strategy. `{{` is Periwiki syntax.
- [x] ~~**Template name case sensitivity:**~~ Case-sensitive. Normalization at URL/routing layer.
- [x] ~~**Sentinel cleanup:**~~ goquery DOM scan.

## Resolved Design Decisions

Decisions surfaced by the expert review, resolved through discussion.

### TOC placeholder mechanism

**Decision:** Best-effort Goldmark emission, fallback to existing goquery insertion.

The Goldmark renderer attempts to emit `<div data-pw-toc></div>` before the first `<h2>` during the render walk. If this proves impractical, the existing goquery-based TOC code continues to inject the placeholder between Goldmark and Bluemonday. Either way, actual TOC construction happens in the post-sanitization goquery pass — the placeholder just marks the insertion point. TOC must be built post-sanitization because it needs to see headings contributed by expanded widgets.

### Footnote and heading ID isolation

**Decision:** Goldmark extension is the primary approach. Fallback to standalone parser.

The primary plan is to implement template expansion as a Goldmark extension, leveraging Goldmark's own handling of code fences (higher priority than the template block parser, so `{{` inside code blocks is never matched). Footnote and heading ID collisions across recursive re-parses will be addressed during implementation — try `ExpansionContext` with global counters first, namespace prefixing as fallback.

If the Goldmark extension approach hits too many compromises with re-parse state isolation, the fallback is to write a standalone lexer/parser that handles markdown + templates as a unified grammar, validated against Goldmark's CommonMark test suite.

### `{{` escape syntax

**Decision:** `<nowiki>` Goldmark extension.

`<nowiki>...</nowiki>` suppresses template expansion inside its content. The pre-pass (or Goldmark extension) strips the tags and passes the content through literally — Goldmark parses the inner content as normal markdown but template invocations are not expanded.

Code fences already protect `{{` because Goldmark's code block parser has higher priority than the template block parser. `<nowiki>` handles the prose case — documenting template syntax in help articles, etc.

### `{{` in param values

**Decision:** Unescaped `{{` in param values is a parse error. Use `\{{` for literal braces.

Param values are data, never code. Unescaped `{{` in a NestedText param value produces a clear error: `param 'example' contains unescaped '{{' — use '\{{' for literal braces`. The `\{{` escape is unescaped to literal `{{` during substitution, producing inert text that cannot be interpreted as a template invocation.

This prevents template injection through variables — a user cannot cause template expansion by crafting param values.

### Empty param rendering artifacts

**Decision:** Core widgets handle conditional rendering. User templates are pure param passthrough.

User templates do not have conditional syntax. If rendering depends on whether a param is present (e.g., omitting parentheses when `period` is absent), that logic belongs in a core widget using Go template conditionals (`{{ if .period }}`). User templates map params to core widget invocations; core widgets decide how to render absent values.

The CreditLine example in this doc demonstrates the limitation — it should be reimplemented as a core widget or restructured to avoid embedding optional params in punctuation.

Future: conditionals in user templates are on the roadmap but will use a Periwiki-specific syntax designed for wiki editors, not Go template passthrough.

### Migration for existing `{{` content

**Decision:** No special migration strategy.

`{{` is Periwiki template syntax. Existing content containing `{{` will be parsed as template invocations — this is correct behavior. Articles that happen to contain `{{` in prose will show error placeholders if the invocation doesn't match a known template, which is visible and fixable by editors.

### Template name case sensitivity

**Decision:** Case-sensitive at the template layer.

`Template:Person` and `Template:person` are distinct articles. `Template:Wow` and `Template:WOW` are distinct. Case normalization (e.g., capitalizing the first letter of a template name when navigating to a not-yet-created page) will be handled at the URL/routing layer, not the template resolution layer.

### Sentinel cleanup mechanism

**Decision:** goquery DOM scan.

The post-sanitization expansion pass already uses goquery. The sentinel sweep checks for elements with `data-pw-*` attributes in the parsed DOM — no false positives from text content that discusses template internals. Unexpanded placeholders are removed from the DOM and logged.

## Expert Review Summary

Ten expert agents reviewed this design against the existing codebase. This section records their findings for reference during implementation.

### Confirmed Design Strengths

- The two-layer trust model (core widgets produce HTML, user templates produce markdown) eliminates an entire class of injection attacks.
- The post-sanitization placeholder pattern correctly prevents trusted HTML from being sanitized. The `WithUnsafe()` invariant is well-reasoned.
- Extension renderers bypass `WithUnsafe` by design (they write directly to the output buffer), so Goldmark extensions can emit placeholder HTML even without `html.WithUnsafe()`.
- The `ArticleLink` table and backlink infrastructure can be reused for template dependency tracking without duplication.
- The overlay FS correctly embeds `_widgets/` via the `all:` prefix in `content.go`.
- `Talk:Template:Person` routing works correctly with the existing namespace handler pattern.
- `Template:Person/Documentation` subpages work as regular articles — no special routing needed.
- String substitution being deliberately simple (no conditionals, no loops) keeps the attack surface small.

### Pre-existing Bugs Found

**Sanitizer policy divergence (HIGH).** `testutil/testutil.go` creates `bluemonday.UGCPolicy()` inline instead of calling `server.NewSanitizer()`. This means tests miss the `input`/`label` element allowlisting and the `data-line-number` attribute rule. Fix: call `server.NewSanitizer()` in the test setup. (Found independently by Testing and Dual Registration experts.)

**Unicode heading IDs (LOW).** Bluemonday's `id` attribute regex may strip non-ASCII characters from heading IDs generated by Goldmark. Pre-existing issue, not caused by this design.

### Cross-Cutting Risks

**ETag staleness after template-triggered re-renders (HIGH).** The current ETag is based on the markdown hash (via `article.Hash`). When a template change triggers a re-render, the article's markdown hasn't changed — only its HTML output. The ETag stays the same, so browsers with cached responses receive 304 Not Modified for stale content. This overlaps with the existing TODO item about whole-site cache busting on deploy. A composite ETag incorporating the HTML hash would solve both problems.

**Rendering service circular dependency (HIGH).** The template Goldmark extension needs to fetch template bodies (via `TemplateResolver`), but the rendering service creates the Goldmark instance. If `TemplateResolver` is backed by `ArticleService`, and `ArticleService` depends on `RenderingService`, there is a circular dependency. Solution: `TemplateResolver` should be a thin interface backed by the article *repository* (not service), since template body fetching doesn't need rendering.

**Pipeline transitions from stateless to stateful (MEDIUM).** The current rendering pipeline is stateless — `Render(markdown) → html`. With template expansion, rendering needs an `ExpansionContext` carrying state across recursive re-parses. This changes the `RenderingService` interface and affects all callers (article service, preview handler, rerender paths).

### UX Risks

**No template discovery mechanism.** Without `Special:AllTemplates`, users cannot browse available templates, defeating reuse. Planned for Phase 5.

**No editor guidance for Template: pages.** A blank textarea with no indication that `templatedef:` frontmatter is expected. Planned for Phase 3.

**Preview behavior with templates.** The preview handler calls `Render()` which will expand templates, but: (a) previewing a template article shows raw `{{ .param }}` markers instead of rendered output with example data, (b) this isn't explicitly tested. Needs attention in Phase 2.

**Empty-param artifacts visible in output.** Blind substitution of absent params leaves orphaned punctuation. Documented limitation for v1, mitigated by pushing complex formatting to core widgets.

**MediaWiki migration friction.** Key differences (`{{{param}}}` vs `{{ .param }}`, pipe-delimited vs fenced NestedText, `<noinclude>` vs `templatedef:` frontmatter) will trip up experienced wiki editors. Help articles should address this explicitly.

### Implementation Constraints

| Constraint | Details |
|-----------|---------|
| `_widgets/` in template hash | `HashRenderTemplates()` must hash `templates/_widgets/` alongside `templates/_render/` |
| `meta_test.go` exclusion | `TestAllTemplateSubdirsHaveGlobs` must skip `_widgets` directory |
| Sanitizer unification | Tests must call `server.NewSanitizer()` instead of inline policy |
| `---` fence parsing | State machine in `Continue()` to distinguish param fence close from thematic break |
| Parser priority | Recommended priority 80, `CanInterruptParagraph() == true` |
| goquery for sentinel sweep | Use DOM scan, not `strings.Contains`, to avoid false positives |
| `html/template` enforcement | Core widgets must use `html/template`, never `text/template` — auto-escaping is a security invariant |
| Widget placeholder index | Assigned during render, from a global counter in `ExpansionContext`. Fresh indices for re-parsed content from the same counter. |
| TOC test updates | 7 existing TOC tests need pipeline adjustment for placeholder-based insertion |
| `skipTOC` frontmatter | Must flow through to the new placeholder mechanism |
