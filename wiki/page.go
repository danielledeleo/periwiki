package wiki

// Page represents any page that can be displayed with a title.
type Page interface {
	DisplayTitle() string
}

// StaticPage represents a non-article page like login or error pages.
type StaticPage struct {
	title string
}

// NewStaticPage creates a new StaticPage with the given title.
func NewStaticPage(title string) *StaticPage {
	return &StaticPage{title: title}
}

// DisplayTitle returns the page's display title.
func (p *StaticPage) DisplayTitle() string {
	return p.title
}
