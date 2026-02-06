# Template & Widget System Design

Status: **Draft — decisions in progress**

## Overview

A template system enabling wiki-defined content types and widgets. Two layers:

- **Core widgets** — Go `html/template` files on disk (`templates/_render/widget/`). Trusted code that produces HTML. Highly configurable with many parameters.
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
| Core widgets | `templates/_render/widget/*.html` | Trusted (code) | Yes — Go `html/template` |
| User templates | `Template:*` wiki articles | Untrusted (content) | No — markdown + param mapping |
| Articles | Regular wiki articles | Untrusted (content) | No — markdown with invocations |

Core widgets are the **only** source of HTML in the template system. User templates are pure data mapping — they cannot execute code or emit raw HTML. This eliminates the need for function allowlists, output sanitization of template results, or template injection prevention at the user template layer.

No `article.type` column or `template_meta` table. The `Template:*` namespace prefix identifies templates. All template configuration lives in frontmatter.

### Rendering Pipeline

Template expansion is a **Goldmark extension**, same as wikilinks and footnotes. Single rendering pass:

```
Article markdown
  |
  v
Goldmark render (single pass, all extensions)
  |
  |-- Markdown syntax     --> HTML (existing)
  |-- [[Wiki links]]      --> <a> tags (existing extension)
  |-- [^Footnotes]        --> <sup>/<section> (existing extension)
  |-- {{Template|params}} --> expanded content (NEW extension)
  |       |
  |       |-- User template (Template:*)?
  |       |     Fetch article, substitute params, feed result
  |       |     back for further expansion (recursive)
  |       |
  |       |-- Core widget (core:name)?
  |       |     Execute disk template with params, emit HTML
  |       |
  |       |-- Plain transclusion?
  |             Fetch Template:* content, insert as-is
  |
  v
HTML
  |
  v
Bluemonday sanitization
  |
  v
Final HTML (stored in revision)
```

Recursive expansion handles user templates that invoke other templates or core widgets. Resolved content is fed back through Goldmark until no `{{...}}` invocations remain.

### Relationship to Existing Infrastructure

These systems are already in place (Phase 0 prerequisites from the original design):

- **Render queue** — Priority-based with interactive/background tiers, article deduplication
- **Stale content detection** — Boot-time template hash check, lazy re-rendering
- **Namespace routing** — `Foo:Bar` URLs route through `NamespaceHandler`; `Template:*` namespace needs registration
- **Frontmatter parsing** — NestedText parser, `frontmatter` BLOB column on article table
- **Special pages registry** — Dynamic handler registration

## Invocation Syntax

### Two Calling Conventions

Like a function call — positional for simple inline use, named (NestedText) for structured data.

**Positional (single line):**
```markdown
{{Stub}}
{{FullName "Valentino" "Garavani"}}
{{core:badge "warning" "Deprecated API"}}
```

**Named (multi-line, fenced NestedText):**
```markdown
{{Person
---
name: Valentino Garavani
born: May 11, 1932
job:
  title: Fashion designer
  tenure: 1959-present
awards:
  - Legion of Honour
  - CFDA Award
---
}}
```

Parser rule: single line = positional args, multi-line with `---` fences = NestedText params.

### Core Widget Invocation

The `core:` prefix invokes a disk template directly:

```markdown
{{core:infobox
---
name: Valentino Garavani
born: May 11, 1932
accent: #4a7c59
---
}}
```

This executes `templates/_render/widget/infobox.html` with the params as its Go template data context.

### Block Syntax (Templates with Body Content)

Templates can wrap content. The body is accessible as `.body` in the template.

> **OPEN QUESTION:** Closing tag syntax — `{{/Name}}` or `{{end}}`?

**Option A — Matching close tag:**
```markdown
{{core:callout "warning"}}
This is the body content with **markdown**.
{{/core:callout}}
```

**Option B — Generic end tag:**
```markdown
{{core:callout "warning"}}
This is the body content with **markdown**.
{{end}}
```

Option A is more explicit and supports nesting unambiguously. Option B is simpler.

### Param References

User templates use **dot notation** to reference passed parameters. The params object is the root context (`.`):

- `.` — the entire params object (for pass-through)
- `.name` — a specific param
- `.job.title` — nested access
- `.body` — reserved key for block content (injected by the system)

NestedText naturally produces nested structures (maps, lists, strings), so params are `map[string]any`, not flat key-value pairs.

> **OPEN QUESTION:** Should `@` be shorthand for `.`? e.g., `@.name` instead of `.name`. May improve readability but adds a synonym. Alternatively, `@` could be reserved specifically for "spread all params" in invocations, with `.name` for individual access.

### Pass-Through and Mapping

When a user template wraps a core widget:

**Full pass-through (interfaces match):**
```
{{core:infobox .
accent: #4a7c59
}}
```

`.` spreads all params into the core widget, with `accent` as a static override.

**Selective mapping (interfaces differ):**
```
{{core:infobox
---
name: .name
born: .born
occupation: .job.title
accent: #4a7c59
---
}}
```

Dot references are substituted before the inner invocation is resolved.

> **OPEN QUESTION:** Exact syntax for mixing dot references with literal values inside fenced NestedText. Does `.name` as a NestedText value get special treatment? Need to define when a value is a literal string vs a param reference. Possible convention: values starting with `.` are references, everything else is literal. Or require a sigil like `@.name`.

### Positional Param Mapping

For positional invocations, the template's frontmatter declares parameter order:

```markdown
---
params:
  - first_name
  - last_name
---
.first_name .last_name
```

`{{FullName "Valentino" "Garavani"}}` maps to `{first_name: "Valentino", last_name: "Garavani"}`.

## Template Interface Schema

A `Template:*` article's frontmatter defines its calling interface. The schema needs to express:

- Which params the template accepts
- Which are required vs optional
- Default values
- Descriptions (for documentation and editor hints)
- Nested structure (maps, lists)
- Positional order

> **OPEN QUESTION:** Schema format. Two options explored:

**Option A — Verbose (explicit metadata per param):**
```
---
widget: infobox
params:
  name:
    required: true
    description: Full name of the person
    position: 1
  born:
    description: Birth date
    default: Unknown
    position: 2
  job:
    description: Employment information
    children:
      title:
      salary:
      tenure:
  awards:
    description: List of notable awards
    type: list
---
```

Pro: Explicit, self-documenting. Con: Heavy, fights NestedText's simplicity (no native booleans or types).

**Option B — Lean (conventions encode metadata):**
```
---
widget: infobox
params:
  name!: Full name of the person
  born: Birth date
  job:
    title: Job title
    salary:
    tenure:
  awards[]: List of notable awards
defaults:
  born: Unknown
  job:
    tenure: present
---
```

- `name!` — trailing `!` means required
- `awards[]` — trailing `[]` means list type
- Leaf values are descriptions
- Nested maps declare their shape inline
- `defaults:` is a separate section matching the params shape
- Positional order follows declaration order

Pro: Compact, works with NestedText naturally. Con: Convention-based, less discoverable.

### The `widget:` Field

The `widget:` frontmatter field links a user template to a disk widget. Its value maps to `templates/_render/widget/{name}.html`.

- **Has `widget:`** — The template wraps a core widget. Its body maps params through to the widget.
- **No `widget:`** — Pure markdown transclusion. The template body is markdown with param substitution.

## Full Example

### Disk widget: `templates/_render/widget/infobox.html`

```html
<aside class="widget-infobox" style="--accent: {{.accent}}">
  <h3>{{.name}}</h3>
  {{if .born}}<dl><dt>Born</dt><dd>{{.born}}</dd></dl>{{end}}
  {{if .occupation}}<dl><dt>Occupation</dt><dd>{{.occupation}}</dd></dl>{{end}}
  {{if .awards}}
  <ul>
    {{range .awards}}<li>{{.}}</li>{{end}}
  </ul>
  {{end}}
</aside>
```

### User template: `Template:Person` (wiki article)

```
---
widget: infobox
params:
  name!: Full name
  born: Birth date
  job:
    title: Job title
    salary:
    tenure:
  awards[]: Notable awards
defaults:
  job:
    tenure: present
---
{{core:infobox .
accent: #4a7c59
}}

.body
```

### Article using the template

```markdown
---
title: Valentino Garavani
---
{{Person
---
name: Valentino Garavani
born: May 11, 1932
job:
  title: Fashion designer
  tenure: 1959-present
awards:
  - Legion of Honour
  - CFDA Award
---
}}

Valentino Garavani is an Italian fashion designer...
```

### Resolution trace

```
1. Goldmark encounters {{Person ---...---}}
2. Parse NestedText params into map
3. Fetch Template:Person article
4. Substitute .name, .born, etc. into template body
5. Result contains {{core:infobox ...}}
6. Recursive pass: resolve core:infobox
7. Execute templates/_render/widget/infobox.html with params
8. HTML fragment emitted into document
9. No more {{...}} invocations — done
```

## Template CSS

> **OPEN QUESTION:** Two approaches under consideration.

**Option A — Full CSS blocks in disk widgets:**
CSS defined alongside the Go template, collected and deduplicated at render time, injected as `<style>` in page `<head>`. Scoped via `data-template` attributes.

**Option B — CSS custom property overrides only:**
Base widget styles live in the site theme CSS (`static/style.css`). Disk widget templates only emit inline `style="--var: value"` for parameterized aspects. No CSS collection/injection needed.

```html
<!-- Widget outputs: -->
<aside class="widget-infobox" style="--accent: #4a7c59">...</aside>
```

```css
/* In theme CSS: */
.widget-infobox {
  border: 1px solid var(--border-color);
  border-top: 3px solid var(--accent, var(--theme-accent));
  float: right;
  width: 300px;
}
```

Option B is simpler (no build-time CSS pipeline) and keeps all CSS in one place. Custom properties provide the parameterization hook. Leaning toward Option B.

## Security

### Structural Safety

The two-layer architecture provides security by construction:

1. **Core widgets** are Go templates on disk — reviewed code, not user input
2. **User templates** are content, not code — they map params and produce markdown
3. **Parameters are always data** — passed as `map[string]any` to Go templates, never parsed as template source
4. **No raw HTML from editor** — user-authored content goes through Goldmark + Bluemonday

### Sanitization

Core widget HTML output needs to survive Bluemonday sanitization.

> **OPEN QUESTION:** How to handle this. Options:
> - Bluemonday allowlist tuned for known widget elements/attributes
> - Different sanitization policy that recognizes trusted widget output
> - Core widget output marked as pre-sanitized and bypassed
>
> Needs investigation. The existing Bluemonday policy already allows `class`, some `data-*` attributes, and `style` on specific elements.

### Template Injection Prevention

- Parameters containing `{{` sequences must not trigger template expansion
- Template names in invocations must resolve to known articles or core widgets
- Circular references detected and halted with error placeholder

### Resource Limits

- **Recursion depth:** 100 (no loops — just a hard ceiling)
- **Output size cap:** TBD
- **No execution timeout needed** — user templates are substitution only, core widgets are simple Go templates

## Error Handling

| Error | Behavior |
|-------|----------|
| Template not found | Render placeholder: `[Template:Name not found]` |
| Core widget not found | Render placeholder: `[core:name not found]` |
| Missing required param | Render warning: `[Template:Name: missing required param 'title']` |
| Invalid param | Render warning with details |
| Circular reference | Render placeholder: `[Circular template reference detected]` |
| Depth limit exceeded | Render placeholder: `[Template nesting too deep]` |

Errors render visibly in the article output as inline warnings.

## Dependency Tracking

Blocked on a **backlink system** (template dependencies operate the same way — article A depends on Template B). When the backlink system exists:

- Track which articles invoke which templates
- Re-render dependent articles when a template changes
- Show dependent count in template edit view
- Deletion safeguards (warn before deleting a template with dependents)

TODO: Design backlink system first, then extend for template dependencies.

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
- Goldmark extension: parse `{{core:name ...}}` syntax
- Execute disk widget templates with params
- Positional and fenced NestedText param parsing
- Error placeholders for missing widgets / bad params

### Phase 2: User Template Transclusion

- Goldmark extension: parse `{{Name ...}}` syntax (wiki template lookup)
- Fetch `Template:*` article content
- Param substitution (`.name`, `.job.title`, etc.)
- Recursive expansion with depth limit
- Block syntax with `.body` support
- Plain markdown transclusion (no `widget:` field)
- Error placeholders for missing templates

### Phase 3: Template Interface Schema

- Frontmatter schema definition (format TBD — see open questions)
- Required/optional param validation
- Default value injection
- Schema-aware error messages

### Phase 4: Template CSS

- Decide on CSS approach (full blocks vs custom property overrides)
- Implement chosen approach
- Theme integration

### Phase 5: Dependency Tracking

- Requires backlink system (separate design)
- Re-render triggers on template change
- Deletion safeguards
- Usage stats in template editor

### Future / Out of Scope

- Page-level layout templates (frontmatter `template:` directive)
- Template revision pinning
- Special page hybrid mode (template + handler data)
- Template visibility (public/private)
- Schema API route (`Template:Name/Schema`)

## Testing Strategy

### Unit Tests

- Invocation syntax parsing (positional and NestedText)
- Param substitution (flat, nested, lists, pass-through)
- Recursive expansion with depth limiting
- Schema validation (required params, defaults)
- Error placeholder generation

### Integration Tests

- Full rendering pipeline with templates + existing extensions
- Core widget execution with various param shapes
- User template wrapping core widget (two-pass resolution)
- Block syntax with body content
- Multiple template invocations in one article

### Security Tests

- Parameters containing `{{` sequences (must not trigger expansion)
- Deeply nested templates (depth limit enforcement)
- Circular template references
- Large output generation
- Template names from user input (must resolve to known templates only)

### Regression Tests

- Existing article rendering unchanged
- WikiLink and footnote extensions still work
- Frontmatter parsing unaffected
- Special pages render correctly

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

- [ ] **Close tag syntax:** `{{/Name}}` vs `{{end}}` for block templates
- [ ] **Param reference syntax:** bare `.name` vs `@.name` — how to distinguish references from literal strings in NestedText values
- [ ] **Schema format:** Verbose (explicit metadata) vs lean (conventions like `!`, `[]`)
- [ ] **CSS approach:** Full CSS blocks with collection/dedup vs custom property overrides in theme CSS
- [ ] **Sanitization strategy:** How core widget HTML survives Bluemonday
- [ ] **Conditionals in user templates:** Should user templates support `{{if .born}}...{{end}}` or stay pure data mapping?
- [ ] **Tail call optimization:** When a user template's entire output is a single `{{core:...}}` invocation, skip the re-parse
- [ ] **Type coercion:** NestedText produces strings — core widgets may just work with strings directly (text in, text out). Coercion to Go types (dates, ints) only needed if widgets do type-specific logic. May not be necessary at all. See nestedtext.org/en/latest/schemas.html for the design philosophy.
