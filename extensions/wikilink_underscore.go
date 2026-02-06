package extensions

import (
	"github.com/danielledeleo/periwiki/wiki"
)

type underscoreResolver struct{}

func (r *underscoreResolver) Resolve(original []byte) ([]byte, [][]byte) {
	return []byte("/wiki/" + wiki.TitleToSlug(string(original))), nil
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
	url := []byte("/wiki/" + wiki.TitleToSlug(string(original)))

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
