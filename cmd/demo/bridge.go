//go:build js && wasm

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall/js"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

const demoBanner = `<div style="background:#fff3cd;border-bottom:2px solid #d4a800;padding:0.5em 1em;text-align:center;font-size:0.9em;color:#664d03;">` +
	`This wiki is running in-browser via WebAssembly. Changes won't persist after closing the tab. Log in as <b>demoadmin</b> / <b>demoadmin</b> to try admin features.</div>`

const demoMainPage = `---
layout: mainpage
display_title: Main Page
---
Welcome to the Periwiki demo! This wiki is running entirely in your browser — no server required.

## Try it out

- **Edit this page** by clicking the "Edit" tab above
- **Create new articles** by visiting any URL like [[Example article]] and clicking "Edit"
- **Link between articles** using ` + "`[[Article Name]]`" + ` wikilink syntax

## About this demo

Everything runs client-side: the full Go application is compiled to WebAssembly and served by a Service Worker. SQLite runs in-memory, so your changes won't survive closing the tab.

See [[Periwiki:Syntax]] for the full syntax reference.
`

// handleRequest converts a JS request object to a Go HTTP request,
// routes it through the mux, and returns a JS response object.
func handleRequest(router http.Handler, reqObj js.Value) any {
	method := reqObj.Get("method").String()
	url := reqObj.Get("url").String()

	var body io.Reader
	if bodyStr := reqObj.Get("body"); !bodyStr.IsUndefined() && !bodyStr.IsNull() {
		body = strings.NewReader(bodyStr.String())
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return errorResponse(500, "failed to create request: "+err.Error())
	}

	req.RemoteAddr = "127.0.0.1:0"

	// Copy headers from JS object
	headers := reqObj.Get("headers")
	if !headers.IsUndefined() && !headers.IsNull() {
		keys := js.Global().Get("Object").Call("keys", headers)
		for i := 0; i < keys.Length(); i++ {
			key := keys.Index(i).String()
			val := headers.Get(key).String()
			req.Header.Set(key, val)
		}
	}

	// Set Content-Type for form posts
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	result := rec.Result()
	respBody, _ := io.ReadAll(result.Body)

	// httptest.ResponseRecorder doesn't auto-detect Content-Type like
	// the real http.ResponseWriter does. Replicate that behavior.
	if result.Header.Get("Content-Type") == "" {
		result.Header.Set("Content-Type", http.DetectContentType(respBody))
	}

	// Inject demo banner into HTML responses
	ct := result.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		bodyStr := strings.Replace(string(respBody), `<nav id="navbar">`, demoBanner+`<nav id="navbar">`, 1)
		respBody = []byte(bodyStr)
	}

	// Build response headers
	respHeaders := js.Global().Get("Object").New()
	for key, vals := range result.Header {
		respHeaders.Set(key, strings.Join(vals, ", "))
	}

	// Extract Set-Cookie headers as an array
	setCookies := js.Global().Get("Array").New()
	for i, c := range result.Header["Set-Cookie"] {
		setCookies.SetIndex(i, c)
	}

	// Pass body as Uint8Array to avoid UTF-8 → UTF-16 corruption of binary data.
	jsBody := js.Global().Get("Uint8Array").New(len(respBody))
	js.CopyBytesToJS(jsBody, respBody)

	resp := js.Global().Get("Object").New()
	resp.Set("status", result.StatusCode)
	resp.Set("headers", respHeaders)
	resp.Set("body", jsBody)
	resp.Set("setCookies", setCookies)
	return resp
}

func errorResponse(status int, msg string) any {
	resp := js.Global().Get("Object").New()
	resp.Set("status", status)
	headers := js.Global().Get("Object").New()
	headers.Set("Content-Type", "text/plain")
	resp.Set("headers", headers)
	resp.Set("body", msg)
	resp.Set("setCookies", js.Global().Get("Array").New())
	return resp
}

// runDemoSetup seeds the demo admin user and Main_Page.
func runDemoSetup(users service.UserService, articles service.ArticleService) {
	// Create demo admin user (first user is auto-promoted to admin)
	admin := &wiki.User{
		ScreenName:  "demoadmin",
		Email:       "demo@localhost",
		RawPassword: "demoadmin",
	}
	if err := users.PostUser(admin); err != nil {
		panic("failed to create demo admin: " + err.Error())
	}

	// Seed Main_Page
	article := wiki.NewArticle("Main_Page", demoMainPage)
	article.Creator = admin
	if err := articles.PostArticle(article); err != nil {
		panic("failed to seed Main_Page: " + err.Error())
	}
}
