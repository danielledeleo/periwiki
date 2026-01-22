package special

import (
	"net/http"

	"github.com/danielledeleo/periwiki/wiki"
)

// RandomArticleGetter is the interface needed by RandomPage.
type RandomArticleGetter interface {
	GetRandomArticleURL() (string, error)
}

// RandomPage handles Special:Random requests.
type RandomPage struct {
	getter RandomArticleGetter
}

// NewRandomPage creates a new Random special page handler.
func NewRandomPage(getter RandomArticleGetter) *RandomPage {
	return &RandomPage{getter: getter}
}

// Handle redirects to a random article.
func (p *RandomPage) Handle(rw http.ResponseWriter, req *http.Request) {
	url, err := p.getter.GetRandomArticleURL()
	if err != nil {
		if err == wiki.ErrNoArticles {
			http.Redirect(rw, req, "/", http.StatusSeeOther)
			return
		}
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(rw, req, "/wiki/"+url, http.StatusSeeOther)
}
