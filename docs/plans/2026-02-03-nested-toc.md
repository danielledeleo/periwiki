# Nested Table of Contents Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add h3 and h4 depth levels to the TOC with hierarchical numbering (1, 1.1, 1.1.1).

**Architecture:** Build a tree of TOC entries in Go from the flat list of h2/h3/h4 nodes found via goquery, pass the tree to the template, and render nested `<ol>` lists. The CSS already supports nested counter numbering via `counters(item, ".")`.

**Tech Stack:** Go, goquery, Go `html/template`, CSS counters (already in place)

---

### Task 1: Add TOCEntry type and tree-building logic to renderer

**Files:**
- Modify: `render/renderer.go`

**Step 1: Write the failing test**

Create `render/renderer_test.go`:

```go
package render

import (
	"strings"
	"testing"
)

func TestTOCNestedHeadings(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "## Section One\n\n### Sub One\n\n### Sub Two\n\n#### Deep\n\n## Section Two\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Should have nested ol for sub-items
	if !strings.Contains(html, "<ol>") {
		t.Error("expected nested <ol> in TOC")
	}

	// h3 items should appear as nested list items
	if !strings.Contains(html, "Sub One") {
		t.Error("expected h3 'Sub One' in TOC")
	}
	if !strings.Contains(html, "Sub Two") {
		t.Error("expected h3 'Sub Two' in TOC")
	}

	// h4 item should appear
	if !strings.Contains(html, "Deep") {
		t.Error("expected h4 'Deep' in TOC")
	}
}

func TestTOCFlatH2Only(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "## First\n\n## Second\n\n## Third\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(html, "First") {
		t.Error("expected 'First' in TOC")
	}
	if !strings.Contains(html, "Second") {
		t.Error("expected 'Second' in TOC")
	}
}

func TestTOCNoHeaders(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "Just a paragraph.\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if strings.Contains(html, `id="toc"`) {
		t.Error("expected no TOC when no h2+ headings")
	}
}

func TestTOCH1Excluded(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "# Title\n\n## Section\n\n### Sub\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// TOC should contain Section and Sub but not Title
	if !strings.Contains(html, "Section") {
		t.Error("expected 'Section' in TOC")
	}
	if !strings.Contains(html, "Sub") {
		t.Error("expected 'Sub' in TOC")
	}
}
```

**Step 2: Run the tests to verify they fail**

Run: `go test ./render/ -v -run TestTOC`
Expected: `TestTOCNestedHeadings` fails (h3/h4 not in TOC yet).

**Step 3: Add TOCEntry struct and buildTOCTree function**

In `render/renderer.go`, add a `TOCEntry` struct and a function that walks a flat slice of heading nodes and builds a tree:

```go
// TOCEntry represents a heading in the table of contents.
type TOCEntry struct {
	ID       string
	Text     string
	Children []TOCEntry
}

// buildTOCTree constructs a nested TOC from a flat list of heading nodes.
// Expects h2, h3, and h4 nodes. h2 is top-level, h3 nests under h2, h4 under h3.
func buildTOCTree(nodes []*html.Node) []TOCEntry {
	var root []TOCEntry

	for _, n := range nodes {
		level := headingLevel(n)
		if level < 2 || level > 4 {
			continue
		}

		entry := TOCEntry{
			ID:   getAttr(n, "id"),
			Text: textContent(n),
		}

		switch level {
		case 2:
			root = append(root, entry)
		case 3:
			if len(root) > 0 {
				root[len(root)-1].Children = append(root[len(root)-1].Children, entry)
			}
		case 4:
			if len(root) > 0 {
				parent := &root[len(root)-1]
				if len(parent.Children) > 0 {
					parent.Children[len(parent.Children)-1].Children = append(
						parent.Children[len(parent.Children)-1].Children, entry)
				}
			}
		}
	}

	return root
}

func headingLevel(n *html.Node) int {
	switch n.Data {
	case "h2":
		return 2
	case "h3":
		return 3
	case "h4":
		return 4
	default:
		return 0
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += textContent(c)
	}
	return s
}
```

**Step 4: Update Render() to use the new tree**

Replace the `headers` finding and template execution in `Render()`:

```go
// Find h2, h3, h4 headings
headers := document.Find("h2, h3, h4")
if headers.Length() == 0 {
    return string(rawhtml), nil
}

// Build nested TOC tree (only h2+ considered)
var nodes []*html.Node
headers.Each(func(_ int, s *goquery.Selection) {
    nodes = append(nodes, s.Nodes[0])
})
tocTree := buildTOCTree(nodes)

if len(tocTree) == 0 {
    return string(rawhtml), nil
}

tmpl, err := template.ParseFiles("templates/_render/toc.html")
if err != nil {
    return "", err
}

outbuf := &bytes.Buffer{}
err = tmpl.Execute(outbuf, map[string]interface{}{"Entries": tocTree})
if err != nil {
    return "", err
}
```

Also fix the insertion point — it should still be before the first h2:

```go
firstH2 := document.Find("h2").Nodes[0]
root.InsertBefore(newnode[0], firstH2)
```

**Step 5: Run tests to verify they pass**

Run: `go test ./render/ -v -run TestTOC`
Expected: All pass (after template update in Task 2).

Note: Tests will still fail here because the template hasn't been updated yet. That's expected — continue to Task 2.

**Step 6: Commit**

```bash
git add render/renderer.go render/renderer_test.go
git commit -m "feat: build nested TOC tree from h2/h3/h4 headings"
```

---

### Task 2: Update TOC template for nested rendering

**Files:**
- Modify: `templates/_render/toc.html`

**Step 1: Replace the flat template with a recursive nested template**

The template needs to render `[]TOCEntry` with nested `<ol>` lists. Go templates support recursion via `{{template}}`:

```html
{{define "tocItems"}}{{ range . }}
        <li>
            <a href="#{{ .ID }}">{{ .Text }}</a>
            {{ if .Children }}<ol>{{template "tocItems" .Children}}</ol>{{ end }}
        </li>{{ end }}{{end}}
<div id="toc">
    <input type="checkbox" id="toc-toggle" class="toc-toggle" checked>
    <div class="toc-header">
        <strong>Contents</strong>
        <span class="toc-brackets">[</span><label for="toc-toggle"></label><span class="toc-brackets">]</span>
    </div>
    <ol>{{template "tocItems" .Entries}}</ol>
</div>
```

**Step 2: Run all tests**

Run: `go test ./render/ -v -run TestTOC`
Expected: All pass.

Also run rendering service tests to check nothing broke:

Run: `go test ./wiki/service/ -v -run TestRender`
Expected: All pass.

**Step 3: Commit**

```bash
git add templates/_render/toc.html
git commit -m "feat: nested TOC template with h3/h4 depth levels"
```

---

### Task 3: Add CSS indent for nested TOC levels

**Files:**
- Modify: `static/main.css`

**Step 1: Check existing styles hold up**

The existing CSS already uses `counters(item, ".")` which automatically numbers nested lists as 1, 1.1, 1.2, 1.2.1, etc. The counter reset and increment rules on `article #toc ol` and `article #toc ol > li` should cascade into nested `<ol>` elements.

Verify by running the server and checking a page with nested headings. If the numbering works but indentation needs adjustment, add:

```css
article #toc ol ol {
    padding-left: 1rem;
}
```

**Step 2: Run full test suite**

Run: `make test`
Expected: All pass.

**Step 3: Commit**

```bash
git add static/main.css
git commit -m "style: indent nested TOC levels"
```

---

### Task 4: Clean up dead error check in Render()

**Files:**
- Modify: `render/renderer.go`

The current code has a dead `if err != nil` check on line 73 (the `err` from `html.Parse` which was already checked on line 65). Clean it up while we're here:

```go
// Before:
if headers.Length() == 0 {
    if err != nil {
        return "", err
    }
    return string(rawhtml), nil
}

// After:
if headers.Length() == 0 {
    return string(rawhtml), nil
}
```

This was already replaced in Task 1 but call it out explicitly as something to verify.

**Step 1: Confirm tests still pass**

Run: `make test`
Expected: All pass.

**Step 2: Commit (if not already folded into Task 1)**

```bash
git add render/renderer.go
git commit -m "fix: remove dead error check in Render()"
```

---

## Notes

- The stale content detection system (see `docs/plans/2026-02-03-stale-content-detection-design.md`) will handle re-rendering existing articles when `templates/_render/toc.html` changes. No additional work needed for cache invalidation.
- h5 and h6 are intentionally excluded — three levels of TOC depth is sufficient. Can be extended later by adding cases to `buildTOCTree`.
- The `textContent` helper handles headings that contain inline markup (bold, links, etc.) by walking all child text nodes.
