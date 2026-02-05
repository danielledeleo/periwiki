# Render Pipeline Fuzzer Design

## Goal

Find subtle bugs in the full rendering pipeline through fuzz testing:
- Parser crashes, panics, nil dereferences
- Pathological inputs causing slowness (algorithmic complexity attacks)
- XSS vectors that survive the bluemonday sanitization policy

## Architecture

Two fuzz targets using production-equivalent setup via `testutil.SetupTestApp`:

### FuzzRenderCrash

Tests the full render+sanitize pipeline for stability:
1. Uses `testutil.SetupTestApp` for production-equivalent renderer
2. Calls `app.Rendering.Render(input)` (full pipeline)
3. Automatic panic detection (Go fuzzer built-in)
4. Explicit timeout check: fail if render takes >100ms

### FuzzRenderXSS

Tests if dangerous content survives sanitization:
1. Uses same setup as FuzzRenderCrash
2. Renders input through full pipeline
3. Uses regex patterns to detect actual XSS vectors (not escaped content)
4. Catches: script tags, dangerous elements, event handlers, javascript: URLs

Key insight: Simple string matching causes false positives on escaped content like `&lt;script&gt;`. Regex patterns match actual HTML elements only.

## Seed Corpus

Seeds are embedded in the test file via `f.Add()`:

### Crash fuzzer seeds:
- Basic markdown (headings, lists, formatting)
- WikiLinks with various syntax
- Footnotes
- Tables
- Frontmatter with edge cases
- Deep nesting, pathological patterns
- Unicode edge cases

### XSS fuzzer seeds:
- Script injection attempts
- Event handler injection
- JavaScript URLs
- Encoding tricks
- Template injection via frontmatter
- WikiLink/footnote XSS attempts

## Usage

```bash
# Run crash fuzzer for 60 seconds
go test -fuzz=FuzzRenderCrash -fuzztime=60s ./render/

# Run XSS fuzzer for 60 seconds
go test -fuzz=FuzzRenderXSS -fuzztime=60s ./render/

# Run indefinitely (until crash found)
go test -fuzz=FuzzRenderCrash ./render/
```

## Files Changed

- `render/renderer_fuzz_test.go` - Fuzz test implementations (new)
- `testutil/testutil.go` - Changed `*testing.T` to `testing.TB` for fuzz compatibility

## Implementation Notes

- Uses `package render_test` (external test package) to avoid import cycle with testutil
- `testutil.SetupTestApp` now accepts `testing.TB` interface, compatible with `*testing.T`, `*testing.B`, and `*testing.F`
- Regex-based XSS detection avoids false positives on HTML-escaped content
