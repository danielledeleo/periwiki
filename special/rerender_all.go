package special

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

// ArticleRerenderer is the interface needed by RerenderAllPage.
type ArticleRerenderer interface {
	GetAllArticles() ([]*wiki.ArticleSummary, error)
	QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan service.RerenderResult, error)
}

// RerenderAllTemplater renders the rerender_all template.
type RerenderAllTemplater interface {
	RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error
}

// RerenderAllPage handles Special:RerenderAll requests.
type RerenderAllPage struct {
	articles  ArticleRerenderer
	templater RerenderAllTemplater
}

// NewRerenderAllPage creates a new RerenderAll special page handler.
func NewRerenderAllPage(articles ArticleRerenderer, templater RerenderAllTemplater) *RerenderAllPage {
	return &RerenderAllPage{
		articles:  articles,
		templater: templater,
	}
}

// Handle shows the rerender form (GET) or triggers rerender (POST).
func (p *RerenderAllPage) Handle(rw http.ResponseWriter, req *http.Request) {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)

	if user.IsAnonymous() {
		http.Error(rw, "Authentication required", http.StatusUnauthorized)
		return
	}

	data := map[string]interface{}{
		"Page":    wiki.NewStaticPage("Rerender All Pages"),
		"Context": req.Context(),
	}

	if req.Method == http.MethodPost {
		p.handlePost(rw, req, data)
		return
	}

	// GET: show form
	if err := p.templater.RenderTemplate(rw, "rerender_all.html", "index.html", data); err != nil {
		slog.Error("failed to render rerender_all template", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}

func (p *RerenderAllPage) handlePost(rw http.ResponseWriter, req *http.Request, data map[string]interface{}) {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)

	articles, err := p.articles.GetAllArticles()
	if err != nil {
		slog.Error("failed to get all articles", "error", err)
		data["calloutMessage"] = "Failed to get articles: " + err.Error()
		data["calloutClasses"] = "pw-error"
		p.renderTemplate(rw, data)
		return
	}

	if len(articles) == 0 {
		data["calloutMessage"] = "No articles to rerender"
		data["calloutClasses"] = "pw-info"
		p.renderTemplate(rw, data)
		return
	}

	slog.Info("queueing rerender of all articles", "count", len(articles), "user", user.ScreenName)

	// Queue all articles and collect result channels
	var resultChannels []<-chan service.RerenderResult
	var queueErrors int

	for _, article := range articles {
		ch, err := p.articles.QueueRerenderRevision(req.Context(), article.URL, 0)
		if err != nil {
			slog.Error("failed to queue article rerender", "url", article.URL, "error", err)
			queueErrors++
			continue
		}
		resultChannels = append(resultChannels, ch)
	}

	// Spawn goroutine to wait for all results and log summary
	go func() {
		var succeeded, failed int
		for _, ch := range resultChannels {
			result := <-ch
			if result.Err != nil {
				slog.Error("article rerender failed", "url", result.URL, "error", result.Err)
				failed++
			} else {
				succeeded++
			}
		}
		slog.Info("rerender complete", "succeeded", succeeded, "failed", failed, "queueErrors", queueErrors, "user", user.ScreenName)
	}()

	queued := len(resultChannels)
	data["calloutMessage"] = fmt.Sprintf("Queued %d articles for rerendering", queued)
	if queueErrors > 0 {
		data["calloutMessage"] = fmt.Sprintf("Queued %d articles for rerendering (%d failed to queue)", queued, queueErrors)
		data["calloutClasses"] = "pw-info"
	} else {
		data["calloutClasses"] = "pw-success"
	}

	p.renderTemplate(rw, data)
}

func (p *RerenderAllPage) renderTemplate(rw http.ResponseWriter, data map[string]interface{}) {
	if err := p.templater.RenderTemplate(rw, "rerender_all.html", "index.html", data); err != nil {
		slog.Error("failed to render rerender_all template", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}
