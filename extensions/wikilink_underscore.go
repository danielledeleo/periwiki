package extensions

import (
	"bytes"
	"regexp"
)

var underscoreRegexp = regexp.MustCompile(`\s+`)

type underscoreResolver struct{}

func (r *underscoreResolver) Resolve(original []byte) ([]byte, [][]byte) {
	return append([]byte("/wiki/"), underscoreRegexp.ReplaceAll(bytes.Trim(original, " \t"), []byte{'_'})...), nil
}

// WithUnderscoreResolver replaces all whitespace in WikiLinks with
// underscores. Contiguous spaces are merged into a single underscore.
//
// e.g.: `[[ Disambiguation (Disambiguation) ]]` becomes `Disambiguation_(Disambiguation)`
func WithUnderscoreResolver() WikiLinkerOption {
	return WithCustomResolver(&underscoreResolver{})
}
