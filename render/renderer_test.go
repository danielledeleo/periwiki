package render

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// The renderer uses template.ParseFiles with a relative path
	// ("templates/_render/toc.html"), so we need to run from project root.
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename))
	os.Chdir(projectRoot)
	os.Exit(m.Run())
}

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

	// Verify actual nesting: "Sub One" should be inside a nested <ol> within
	// the "Section One" <li>, not at the top level.
	// The structure should be: <li>..Section One..<ol>..Sub One..Sub Two..<ol>..Deep..
	tocStart := strings.Index(html, `id="toc"`)
	if tocStart == -1 {
		t.Fatal("TOC div not found")
	}
	toc := html[tocStart:]

	// Section One should appear before any nested <ol>
	sectionOneIdx := strings.Index(toc, "Section One")
	firstNestedOL := strings.Index(toc[sectionOneIdx:], "<ol>")
	subOneIdx := strings.Index(toc[sectionOneIdx:], "Sub One")
	if firstNestedOL == -1 || subOneIdx < firstNestedOL {
		t.Error("expected 'Sub One' to appear inside a nested <ol> after 'Section One'")
	}

	// Deep should be nested even further (inside a second-level nested <ol>)
	subTwoIdx := strings.Index(toc[sectionOneIdx:], "Sub Two")
	deepIdx := strings.Index(toc[sectionOneIdx:], "Deep")
	if deepIdx < subTwoIdx {
		t.Error("expected 'Deep' to appear after 'Sub Two' in nested structure")
	}

	// Count <ol> tags before "Deep" -- should be at least 3 (top-level + h3 nest + h4 nest)
	tocBeforeDeep := toc[:strings.Index(toc, "Deep")]
	olCount := strings.Count(tocBeforeDeep, "<ol>")
	if olCount < 3 {
		t.Errorf("expected at least 3 <ol> tags before 'Deep' (got %d), indicating proper nesting", olCount)
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

func TestTOCOrphanH3BeforeH2(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "### Orphan\n\n## Section\n\n### Sub\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	tocContent := extractTOC(t, html)

	// "Orphan" h3 before any h2 should not appear in the TOC
	if strings.Contains(tocContent, "Orphan") {
		t.Error("orphan h3 before any h2 should not appear in the TOC")
	}
	// "Section" and "Sub" should appear
	if !strings.Contains(tocContent, "Section") {
		t.Error("expected 'Section' in TOC")
	}
	if !strings.Contains(tocContent, "Sub") {
		t.Error("expected 'Sub' in TOC")
	}
}

func TestTOCOrphanH4UnderH2(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	// h4 directly under h2 with no h3 intermediary -- should be dropped
	md := "## Section\n\n#### Deep\n\n### Normal Sub\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	tocContent := extractTOC(t, html)

	// "Deep" h4 without an h3 parent should be dropped
	if strings.Contains(tocContent, "Deep") {
		t.Error("orphan h4 (no h3 parent) should not appear in the TOC")
	}
	// "Normal Sub" h3 should appear
	if !strings.Contains(tocContent, "Normal Sub") {
		t.Error("expected 'Normal Sub' in TOC")
	}
}

func TestTOCOnlyH3H4NoH2(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "### A\n\n#### B\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// No h2 means no root TOC entries, so no TOC should be generated
	if strings.Contains(html, `id="toc"`) {
		t.Error("expected no TOC when only h3/h4 headings (no h2)")
	}
}

func TestTOCHeadingWithInlineMarkup(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "## **Bold** heading\n\n## Normal\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	tocContent := extractTOC(t, html)

	// textContent should extract text through inline markup
	if !strings.Contains(tocContent, "Bold heading") {
		t.Error("expected 'Bold heading' extracted from inline markup in TOC")
	}
}

// extractTOC returns the full TOC div content from rendered HTML.
func extractTOC(t *testing.T, html string) string {
	t.Helper()
	tocStart := strings.Index(html, `<div id="toc">`)
	if tocStart == -1 {
		t.Fatal("TOC div not found")
	}
	// Find the matching closing </div> by counting nesting depth.
	depth := 0
	for i := tocStart; i < len(html); i++ {
		if strings.HasPrefix(html[i:], "<div") {
			depth++
		} else if strings.HasPrefix(html[i:], "</div>") {
			depth--
			if depth == 0 {
				return html[tocStart : i+len("</div>")]
			}
		}
	}
	t.Fatal("TOC closing div not found")
	return ""
}

func TestTOCH1Excluded(t *testing.T) {
	r := NewHTMLRenderer(nil, nil, nil)

	md := "# Title\n\n## Section\n\n### Sub\n"

	html, err := r.Render(md)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// TOC should contain Section and Sub
	if !strings.Contains(html, "Section") {
		t.Error("expected 'Section' in TOC")
	}
	if !strings.Contains(html, "Sub") {
		t.Error("expected 'Sub' in TOC")
	}

	// h1 "Title" should not appear in the TOC div
	tocStart := strings.Index(html, `id="toc"`)
	if tocStart == -1 {
		t.Fatal("TOC div not found")
	}
	// Find the closing </div> of the TOC
	tocSection := html[tocStart:]
	closingDiv := strings.Index(tocSection, "</div>")
	if closingDiv == -1 {
		t.Fatal("TOC closing div not found")
	}
	tocContent := tocSection[:closingDiv]
	if strings.Contains(tocContent, "Title") {
		t.Error("h1 'Title' should not appear in the TOC")
	}
}
