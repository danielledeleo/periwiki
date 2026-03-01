package periwiki_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/templater"
)

// TestAllTemplateSubdirsHaveGlobs verifies that every subdirectory under
// templates/ (except _render and layouts, which are handled separately) has
// a matching glob in templater.ContentGlobs. This catches the case where
// a developer adds templates/newdir/ but forgets to register the glob.
func TestAllTemplateSubdirsHaveGlobs(t *testing.T) {
	entries, err := os.ReadDir("templates")
	if err != nil {
		t.Fatalf("failed to read templates directory: %v", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// layouts is the base glob (LayoutGlob), _render is for extension templates
		if name == "layouts" || name == "_render" {
			continue
		}

		expectedGlob := fmt.Sprintf("templates/%s/*.html", name)
		found := false
		for _, g := range templater.ContentGlobs {
			if g == expectedGlob {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("template subdirectory %q has no matching glob in templater.ContentGlobs; add %q to the list in templater/globs.go", name, expectedGlob)
		}
	}
}

// TestExportedFunctionsHaveTests uses go/ast to find all exported functions
// and methods in key packages, then verifies each name appears in at least
// one test file. This catches the case where a developer adds a new exported
// function but forgets to write a test for it.
//
// When this test fails, either:
//  1. Write a test for the function (preferred), or
//  2. Add it to the allowList below with a reason why it's untested.
func TestExportedFunctionsHaveTests(t *testing.T) {
	// Packages to scan for exported functions.
	// Note: extensions/ is excluded — it's Goldmark plugin internals where
	// exported methods satisfy framework interfaces (Extend, RegisterFuncs,
	// Transform, etc.) and are tested through rendering output, not directly.
	packages := []string{
		"wiki",
		"wiki/service",
		"render",
		"special",
		"templater",
		"internal/storage",
		"internal/embedded",
	}

	// Functions that are intentionally not directly tested.
	// Key format: "package.FuncName" or "package.ReceiverType.MethodName"
	//
	// When this test fails, prefer writing a test over adding to this list.
	// If you do add an entry, include a reason and the test that exercises it
	// indirectly (if any).
	allowList := map[string]string{
		// Production infrastructure — only called during server startup, no test exercises these
		"internal/storage.Init":   "setup only; tested via TestDB which mirrors the same init path",
		"wiki.LoadRuntimeConfig": "setup only; composes GetOrCreateSetting which is tested directly",

		// Tested indirectly — traced to specific test functions
		"wiki/service.articleService.PostArticleWithContext":         "called by PostArticle; exercised by TestPreviewArticle, TestPostArticle*",
		"wiki/service.articleService.RerenderRevision":               "called by handleRerenderRevision; exercised by TestRerenderCurrentRevision, TestRerenderSpecificRevision",
		"wiki/service.embeddedArticleService.BackfillLinks":          "delegates to articleService.BackfillLinks; base tested by TestBackfillLinks",
		"wiki/service.embeddedArticleService.PostArticleWithContext": "delegates to articleService.PostArticleWithContext",
		"wiki/service.embeddedArticleService.RerenderRevision":       "delegates to articleService.RerenderRevision",
		"wiki/service.embeddedArticleService.QueueRerenderRevision":  "delegates to articleService.QueueRerenderRevision; base tested by TestQueueRerenderRevision",
		"wiki/service.renderingService.PreviewMarkdown":              "called by articlePreviewHandler; exercised by TestPreviewArticle",

		// User contributions — exercised indirectly via integration tests (TestSpecialContributions, TestUserNamespace_ViewProfile)
		"wiki/service.articleService.GetRevisionsByScreenName":         "called by Special:Contributions handler; exercised by TestSpecialContributions",
		"wiki/service.articleService.GetUserEditCount":                 "called by User: page handler; exercised by TestUserNamespace_ViewProfile",
		"wiki/service.embeddedArticleService.GetRevisionsByScreenName": "delegates to articleService.GetRevisionsByScreenName",
		"wiki/service.embeddedArticleService.GetUserEditCount":         "delegates to articleService.GetUserEditCount",
	}

	// Collect contents of all test files and test infrastructure.
	var testContent strings.Builder
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case ".worktrees", ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		isTestFile := strings.HasSuffix(path, "_test.go")
		isTestutil := strings.HasPrefix(path, "testutil"+string(os.PathSeparator)) && strings.HasSuffix(path, ".go")
		if isTestFile || isTestutil {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			testContent.Write(data)
			testContent.WriteByte('\n')
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk project directory: %v", err)
	}
	allTests := testContent.String()

	for _, pkg := range packages {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, pkg, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Errorf("failed to parse package %s: %v", pkg, err)
			continue
		}

		for _, p := range pkgs {
			for _, file := range p.Files {
				for _, decl := range file.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok || !fn.Name.IsExported() {
						continue
					}

					name := fn.Name.Name
					key := pkg + "." + name
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						recvType := receiverTypeName(fn.Recv.List[0].Type)
						key = pkg + "." + recvType + "." + name
					}

					if _, ok := allowList[key]; ok {
						continue
					}

					if !strings.Contains(allTests, name) {
						t.Errorf("exported function %s is not referenced in any test file; "+
							"write a test or add to allowList in meta_test.go", key)
					}
				}
			}
		}
	}
}

// receiverTypeName extracts the type name from a method receiver expression.
func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return "unknown"
	}
}
