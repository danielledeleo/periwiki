//go:build tools

package tools

// Tool dependencies - imported here to keep them in go.mod
import (
	_ "github.com/go-git/go-git/v5"
	_ "golang.org/x/mod/modfile"
)
