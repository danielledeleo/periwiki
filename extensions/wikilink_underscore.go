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

// deadlinkClass is the CSS class applied to links pointing to non-existent pages.
var deadlinkClass = []byte("pw-deadlink")

// existenceAwareResolver wraps the underscore resolver and checks if pages exist.
// Links to non-existent pages receive the pw-deadlink CSS class.
type existenceAwareResolver struct {
	checker ExistenceChecker
}

func (r *existenceAwareResolver) Resolve(original []byte) ([]byte, [][]byte) {
	url := append([]byte("/wiki/"), underscoreRegexp.ReplaceAll(bytes.Trim(original, " \t"), []byte{'_'})...)

	if r.checker != nil && !r.checker(string(url)) {
		return url, [][]byte{deadlinkClass}
	}
	return url, nil
}

// WithExistenceAwareResolver creates a resolver that applies the pw-deadlink
// class to links pointing to non-existent pages.
func WithExistenceAwareResolver(checker ExistenceChecker) WikiLinkerOption {
	return WithCustomResolver(&existenceAwareResolver{checker: checker})
}
