# Periwiki Code Review Report

**Date:** 2026-01-24
**Reviewer:** Claude Code (claude-opus-4-5-20251101)
**Scope:** Full codebase review for tech debt assessment and future planning

---

## Executive Summary

Periwiki demonstrates solid software engineering practices with clean separation of concerns, comprehensive integration tests, and thoughtful security considerations. The codebase is well-organized and follows Go conventions consistently.

**Overall Health: Good**

The main areas requiring attention are:
1. Infrastructure-level security features needed before production deployment
2. Code duplication in test infrastructure
3. Several incomplete features that should be finished or removed
4. Minor bugs and code quality issues

---

## Strengths

### 1. Excellent Test Coverage
The codebase has impressive test coverage across multiple layers:
- `wiki/service/*_test.go` - Service layer unit tests (article, user, rendering)
- `internal/storage/sqlite_test.go` - Database layer tests
- `handlers_integration_test.go` - Full integration tests for HTTP handlers
- `auth_integration_test.go` - Authentication flow tests
- `security_test.go` and `content_security_integration_test.go` - Dedicated security testing

### 2. Strong Security Practices
- HTML sanitization using bluemonday UGC policy (`setup.go`)
- XSS prevention in diff handler (`server.go`) - uses `html.EscapeString()`
- Safe type assertions in session middleware (`session_middleware.go`)
- Password hashing with bcrypt (`wiki/user.go`)
- Parameterized SQL queries throughout (`internal/storage/`)
- Input validation for usernames (`wiki/service/user.go`)
- WikiLink renderer checks for dangerous URLs (`extensions/wikilink.go`)

### 3. Clean Architecture
- Clear separation between handlers, services, repositories, and data access
- Well-defined service interfaces (`wiki/service/`) and repository interfaces (`wiki/repository/`)
- Extension system for markdown with template-based rendering
- Special pages registry pattern (`special/special.go`)

### 4. Good Logging Practices
- Structured logging with slog (`logger.go`)
- Consistent use of categories for audit trails (auth, article actions)
- Configurable log format and level

### 5. Well-Designed Extension System
- Goldmark extensions are cleanly implemented
- Template-based rendering for wikilinks and footnotes allows customization
- Existence-aware wikilink resolver marks dead links (`extensions/wikilink_underscore.go:27-38`)

---

## Issues Found

### Critical (Must Fix)

**None found.** The codebase is in good shape with no critical bugs or security vulnerabilities.

---

### Important (Should Fix)

#### 1. Missing Error Handling in `check()` Helper
**Location:** `server.go:295-298`

```go
func check(err error) {
    if err != nil {
        slog.Error("unexpected error", "error", err)
    }
}
```

**Problem:** Only logs errors, does not propagate them or take recovery action. Can lead to silent failures.

**Impact:** When template rendering fails, users receive potentially incomplete responses without any indication.

**Fix:** Return errors and handle them properly, or call `http.Error()` to inform the client.

---

#### 2. Hardcoded Template Path in Renderer
**Location:** `render/renderer.go:55`

```go
tmpl, err := template.ParseFiles("templates/helpers/toc.html")
```

**Problem:** ToC template path is hardcoded relative to working directory. Breaks if run from different directory.

**Fix:** Pass template directory path as configuration or embed templates.

---

#### 3. ~~Code Duplication Between `db/sqlite.go` and `testutil/testutil.go`~~ RESOLVED
**Status:** Fixed in refactor commit `8336db2`.

The test infrastructure now uses the same service implementations as production code. `TestApp` holds service instances directly, eliminating the duplicated database methods.

---

#### 4. Missing Tests for Key Packages
**Affected:**
- `render/` - No test files
- `templater/` - No test files

**Problem:** Changes to rendering logic or template functions cannot be verified in isolation.

**Fix:** Add unit tests for `HTMLRenderer.Render()` and `Templater.RenderTemplate()`.

---

#### 5. Typos in Handler Names
**Location:** `server.go:93-95`

```go
router.HandleFunc("/user/login", app.loginHander).Methods("GET")
router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")
router.HandleFunc("/user/logout", app.logoutPostHander).Methods("POST")
```

**Fix:** Rename to `loginHandler`, `loginPostHandler`, `logoutPostHandler`.

---

#### 6. Dirty Hack in Revision History
**Location:** `internal/storage/article_repo.go`

```go
rev.Markdown = fmt.Sprint(result.Length) // dirty hack
```

**Problem:** Article length is stored in Markdown field as string, which is semantically incorrect.

**Fix:** Add a `Length` field to `Revision` struct for history queries, or use a separate DTO.

---

#### 7. Session Cookie Security Settings Not Configured
**Location:** `server.go:172-173`

**Problem:** Session cookies don't explicitly set `Secure`, `HttpOnly`, or `SameSite` attributes.

**Impact:** Vulnerable to CSRF in production without `SameSite` setting.

**Fix:**
```go
session.Options.Secure = true  // for HTTPS
session.Options.SameSite = http.SameSiteLaxMode
```

---

#### 8. Anonymous Editing Without IP Tracking (Commented Out)
**Location:** `internal/storage/article_repo.go`

**Problem:** Anonymous edit tracking is disabled but the table exists in schema. Anonymous users can edit without accountability.

**Fix:** Either enable anonymous edit tracking or remove the `AnonymousEdit` table.

---

### Minor (Nice to Have)

| Issue | Location | Description |
|-------|----------|-------------|
| Unused model package | `model/` | Package appears unused - remove or consolidate |
| Hardcoded home page | `server.go` | Make configurable or load from wiki article |
| Inline CSS in diff | `server.go` | Use CSS classes instead |
| Unused manage router | `server.go` | Placeholder that does nothing |
| SelectPreference bug | `internal/storage/preference_repo.go` | Uses `Select()` instead of `Get()` |
| No CSRF protection | Forms | POST forms lack CSRF tokens |
| No rate limiting | Auth endpoints | Vulnerable to brute force |

---

## Technical Debt Summary

1. ~~**Test Infrastructure Duplication**~~: RESOLVED - Tests now use production services directly.

2. **Missing Unit Tests**: The `render/` and `templater/` packages lack unit tests, making refactoring risky.

3. **Incomplete Features**: Anonymous edit tracking and management pages are partially implemented but disabled/unused.

4. **Hardcoded Values**: Template paths and home page content are hardcoded instead of being configurable.

5. **Security Gaps**: Missing CSRF protection and rate limiting should be addressed before production deployment.

6. **Code Quality**: Minor issues like typos in function names, dead code, and "dirty hack" comments indicate areas needing cleanup.

---

## Recommendations for Future Work

### High Priority (Security/Stability)

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 1 | Add CSRF protection using `gorilla/csrf` middleware | Medium | High |
| 2 | Implement rate limiting for authentication endpoints | Medium | High |
| 3 | Fix cookie security settings (Secure, SameSite) for production | Low | High |
| 4 | Fix `check()` function to properly handle errors | Low | Medium |

### Medium Priority (Maintainability)

| # | Task | Effort | Impact |
|---|------|--------|--------|
| ~~5~~ | ~~Consolidate test database code to eliminate duplication~~ | ~~High~~ | DONE |
| 6 | Add unit tests for `render/` and `templater/` packages | Medium | Medium |
| 7 | Fix the `SelectPreference` bug (Select vs Get) | Low | Medium |
| 8 | Make home page configurable or loadable from wiki | Low | Low |
| 9 | Enable or remove anonymous edit tracking | Low | Medium |
| 10 | Rename misspelled handler functions | Low | Low |

### Low Priority (Code Quality)

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 11 | Remove dead code (manage router) | Low | Low |
| 12 | Move inline CSS to stylesheet for diff view | Low | Low |
| 13 | Fix revision history "dirty hack" - add Length field | Medium | Low |
| 14 | Embed templates or make paths configurable | Medium | Medium |
| 15 | Remove or implement `model/` package | Low | Low |

---

## Feature Development Suggestions

Based on the codebase structure and existing patterns, these features would be natural extensions:

1. **Management Interface**: The `/manage` router exists as a placeholder - implement admin dashboard
2. **Search Functionality**: Full-text search across wiki articles
3. **Categories/Tags**: Article categorization system
4. **File Uploads**: Image and media attachment support
5. **Export/Import**: Wiki content backup and restore
6. **Theme Support**: Multiple CSS themes (dark mode, etc.)
7. **API**: REST API for programmatic access

---

## Conclusion

Periwiki is a well-designed, well-tested codebase that follows Go best practices. The architecture is clean and extensible, with clear separation into services (`wiki/service/`), repositories (`wiki/repository/`), and storage implementations (`internal/storage/`). The primary areas for improvement are:

1. **Production Hardening**: CSRF protection, rate limiting, and cookie security
2. **Code Cleanup**: Removing dead code and fixing minor bugs

The codebase provides a solid foundation for future development. The extension system and special pages registry demonstrate thoughtful design that makes adding new features straightforward.

**Update (2026-01-24):** Test infrastructure duplication has been resolved. The codebase now uses a clean service-oriented architecture with tests living alongside their implementations.

---

*Generated by Claude Code code-reviewer agent*
