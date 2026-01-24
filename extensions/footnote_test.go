package extensions

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// testFootnoteTemplates creates simple templates for testing.
func testFootnoteTemplates() map[string]*template.Template {
	return map[string]*template.Template{
		"link":     template.Must(template.New("link").Parse(`<sup id="{{.RefID}}"><a href="#{{.ID}}">[{{.Index}}]</a></sup>`)),
		"backlink": template.Must(template.New("backlink").Parse(`<a href="#{{.RefID}}">^{{.RefIndex}}</a> `)),
		"list":     template.Must(template.New("list").Parse(`{{if .Entering}}<ol class="footnotes">{{else}}</ol>{{end}}`)),
		"item":     template.Must(template.New("item").Parse(`{{if .Entering}}<li id="{{.ID}}">{{else}}</li>{{end}}`)),
	}
}

func TestFootnoteBasic(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`This is text with a footnote[^1].

[^1]: This is the footnote content.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Check that the footnote link is present
	if !strings.Contains(result, `<sup id="fn-ref-1-0"><a href="#fn-1">[1]</a></sup>`) {
		t.Errorf("expected footnote link, got: %s", result)
	}

	// Check that the footnote list is present
	if !strings.Contains(result, `<ol class="footnotes">`) {
		t.Errorf("expected footnote list, got: %s", result)
	}

	// Check that footnote content exists
	if !strings.Contains(result, "This is the footnote content.") {
		t.Errorf("expected footnote content, got: %s", result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteBacklinkPosition(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`Reference one[^note].

[^note]: The footnote text here.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// The backlink should appear BEFORE the footnote text
	// Expected pattern: <li id="..."><p><a href="...">^0</a> The footnote text here.</p></li>
	backlinkIdx := strings.Index(result, `<a href="#fn-ref-1-0">^0</a>`)
	contentIdx := strings.Index(result, "The footnote text here.")

	if backlinkIdx == -1 {
		t.Errorf("backlink not found in output: %s", result)
	}
	if contentIdx == -1 {
		t.Errorf("footnote content not found in output: %s", result)
	}
	if backlinkIdx > contentIdx {
		t.Errorf("backlink should appear before content, backlink at %d, content at %d\noutput: %s",
			backlinkIdx, contentIdx, result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteMultipleReferences(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`First reference[^1] and second reference[^1] to the same footnote.

[^1]: Shared footnote content.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Check that both reference links exist with different ref IDs
	if !strings.Contains(result, `id="fn-ref-1-0"`) {
		t.Errorf("expected first reference link, got: %s", result)
	}
	if !strings.Contains(result, `id="fn-ref-1-1"`) {
		t.Errorf("expected second reference link, got: %s", result)
	}

	// Check that both backlinks exist
	if !strings.Contains(result, `href="#fn-ref-1-0"`) {
		t.Errorf("expected first backlink, got: %s", result)
	}
	if !strings.Contains(result, `href="#fn-ref-1-1"`) {
		t.Errorf("expected second backlink, got: %s", result)
	}

	// Verify backlinks appear before content
	backlink0Idx := strings.Index(result, `<a href="#fn-ref-1-0">^0</a>`)
	backlink1Idx := strings.Index(result, `<a href="#fn-ref-1-1">^1</a>`)
	contentIdx := strings.Index(result, "Shared footnote content.")

	if backlink0Idx == -1 || backlink1Idx == -1 {
		t.Errorf("backlinks not found in output: %s", result)
	}
	if backlink0Idx > contentIdx || backlink1Idx > contentIdx {
		t.Errorf("backlinks should appear before content\noutput: %s", result)
	}
	// First backlink should come before second backlink
	if backlink0Idx > backlink1Idx {
		t.Errorf("backlinks should be in order, ^0 at %d, ^1 at %d\noutput: %s",
			backlink0Idx, backlink1Idx, result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteMultipleFootnotes(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`First note[^1], second note[^2], third note[^3].

[^1]: First footnote.
[^2]: Second footnote.
[^3]: Third footnote.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Check all three footnotes are present
	for i := 1; i <= 3; i++ {
		if !strings.Contains(result, `<li id="fn-`+string('0'+byte(i))) {
			t.Errorf("expected footnote %d item, got: %s", i, result)
		}
	}

	// Check content is present
	if !strings.Contains(result, "First footnote.") {
		t.Errorf("expected first footnote content, got: %s", result)
	}
	if !strings.Contains(result, "Second footnote.") {
		t.Errorf("expected second footnote content, got: %s", result)
	}
	if !strings.Contains(result, "Third footnote.") {
		t.Errorf("expected third footnote content, got: %s", result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteIDPrefix(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(
				WithFootnoteTemplates(testFootnoteTemplates()),
				WithFootnoteIDPrefix("custom-"),
			),
		),
	)

	source := []byte(`Text[^1].

[^1]: Footnote.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Check that custom prefix is used
	if !strings.Contains(result, `id="custom-1"`) {
		t.Errorf("expected custom ID prefix 'custom-1', got: %s", result)
	}
	if !strings.Contains(result, `href="#custom-1"`) {
		t.Errorf("expected custom href '#custom-1', got: %s", result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteNoFootnotes(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`Just regular text without any footnotes.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Should not contain footnote structures
	if strings.Contains(result, `class="footnotes"`) {
		t.Errorf("should not have footnote list, got: %s", result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteInlineContent(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`Text[^1].

[^1]: Footnote with *emphasis* and **bold**.`)

	var buf bytes.Buffer
	if err := markdown.Convert(source, &buf); err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	result := buf.String()

	// Check that inline formatting is preserved
	if !strings.Contains(result, "<em>emphasis</em>") {
		t.Errorf("expected emphasis in footnote, got: %s", result)
	}
	if !strings.Contains(result, "<strong>bold</strong>") {
		t.Errorf("expected bold in footnote, got: %s", result)
	}

	t.Logf("output: %s", result)
}

func TestFootnoteASTStructure(t *testing.T) {
	markdown := goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithExtensions(
			NewFootnote(WithFootnoteTemplates(testFootnoteTemplates())),
		),
	)

	source := []byte(`Text[^1].

[^1]: Footnote content.`)

	reader := text.NewReader(source)
	doc := markdown.Parser().Parse(reader)

	// Walk the AST and find the footnote list
	var foundList *east.FootnoteList
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if list, ok := n.(*east.FootnoteList); ok {
			foundList = list
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})

	if foundList == nil {
		t.Fatal("FootnoteList not found in AST")
	}

	// Check the first footnote
	fn := foundList.FirstChild()
	if fn == nil {
		t.Fatal("No footnote found in list")
	}

	footnote, ok := fn.(*east.Footnote)
	if !ok {
		t.Fatalf("First child is not a Footnote, got %T", fn)
	}

	// Find the paragraph container
	container := footnote.LastChild()
	if container == nil || !ast.IsParagraph(container) {
		t.Fatal("Footnote has no paragraph container")
	}

	// Print out the children order
	t.Log("Children of paragraph container:")
	childIdx := 0
	for child := container.FirstChild(); child != nil; child = child.NextSibling() {
		t.Logf("  [%d] %T", childIdx, child)
		childIdx++
	}

	// Check if first child is a backlink
	firstChild := container.FirstChild()
	if _, ok := firstChild.(*east.FootnoteBacklink); !ok {
		t.Errorf("First child should be FootnoteBacklink, got %T", firstChild)
	}
}
