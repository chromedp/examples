// Package testsite is a local web site for the example programs. It looks like
// a real site, with long pages, images, forms, tables and scripts, and it needs
// no internet. A program starts it with New and loads its pages in the browser.
//
// The site serves a documentation page, a long article with a search, a
// repository page with a ZIP download, an image gallery, weather forecasts, an
// IP lookup, a map with tiles, a page for device emulation and a report for
// printing. The file README.md in this directory lists every route.
//
// All content is original and is generated or embedded, so every run serves
// the same bytes.
package testsite

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed images
var imageFS embed.FS

// Site is a running test site.
type Site struct {
	// URL is the address of the site, such as http://127.0.0.1:41233. It has
	// no trailing slash.
	URL string

	// OtherURL is a second address of the same server, such as
	// http://localhost:41233. A browser treats it as another origin and
	// another site, so a program can use it for a cross-site page or a
	// cross-origin request.
	OtherURL string

	srv *httptest.Server
}

// New starts the site on a free port of the loopback interface. Call Close to
// stop it.
func New() *Site {
	srv := httptest.NewServer(Handler())
	other := srv.URL
	if u, err := url.Parse(srv.URL); err == nil {
		if _, port, err := net.SplitHostPort(u.Host); err == nil {
			other = "http://" + net.JoinHostPort("localhost", port)
		}
	}
	return &Site{URL: srv.URL, OtherURL: other, srv: srv}
}

// Close stops the site and waits for its requests to finish.
func (s *Site) Close() {
	s.srv.Close()
}

// Handler returns the handler of the site, for a program that wants to serve
// it on its own listener.
func Handler() http.Handler {
	s := newServer()
	mux := http.NewServeMux()
	s.routes(mux)
	return mux
}

// page is the data that the layout template needs.
type page struct {
	Title       string
	Description string
	Lang        string
	Active      string // the path of the item of the navigation that is current
	Class       string // a class of the body element
	CSS         []string
	JS          []string
	Search      bool   // show the search form of the wiki in the header
	Query       string // the text of the search form
	Data        any
}

type server struct {
	pages map[string]*template.Template
	wiki  *wiki
	gal   []galleryImage
}

var templateFuncs = template.FuncMap{
	"add":   func(a, b int) int { return a + b },
	"seq":   seq,
	"join":  strings.Join,
	"lower": strings.ToLower,
	"safe":  func(s string) template.HTML { return template.HTML(s) },
	"fnum":  formatNumber,
	"pad2":  func(n int) string { return fmt.Sprintf("%02d", n) },
}

// seq returns the numbers from start up to but not including end.
func seq(start, end int) []int {
	var out []int
	for i := start; i < end; i++ {
		out = append(out, i)
	}
	return out
}

// formatNumber writes a number with a comma between the groups of three digits.
func formatNumber(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, s[i])
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func newServer() *server {
	s := &server{pages: make(map[string]*template.Template)}
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".gohtml")
		if name == "layout" {
			continue
		}
		s.pages[name] = template.Must(template.New(name).Funcs(templateFuncs).ParseFS(templateFS, "templates/layout.gohtml", "templates/"+e.Name()))
	}
	s.wiki = newWiki()
	s.gal = loadGallery()
	return s
}

// render executes the template of a page and writes the result with its
// length.
func (s *server) render(w http.ResponseWriter, name string, p page) {
	t, ok := s.pages[name]
	if !ok {
		http.Error(w, "no template "+name, http.StatusInternalServerError)
		return
	}
	if p.Lang == "" {
		p.Lang = "en"
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		log.Printf("testsite: rendering %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

func (s *server) notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	var buf bytes.Buffer
	t := s.pages["notfound"]
	p := page{Title: "Page not found", Lang: "en", Data: r.URL.Path}
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (s *server) routes(mux *http.ServeMux) {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheable(http.FileServerFS(static))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusFound)
	})
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /tools", s.tools)
	mux.HandleFunc("GET /studio", s.studio)
	mux.HandleFunc("GET /ua", s.whoami)
	mux.HandleFunc("GET /whoami", s.whoami)
	mux.HandleFunc("GET /viewport-test", s.viewportTest)
	mux.HandleFunc("GET /print/report", s.printReport)
	mux.HandleFunc("GET /docs/{$}", s.docsIndex)
	mux.HandleFunc("GET /docs/time", s.docsTime)
	s.newsRoutes(mux)
	s.wikiRoutes(mux)
	s.repoRoutes(mux)
	s.galleryRoutes(mux)
	s.weatherRoutes(mux)
	s.geoRoutes(mux)
	mux.HandleFunc("/", s.notFound)
}

// cacheable adds a header that lets the browser keep a static file.
func cacheable(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		h.ServeHTTP(w, r)
	})
}

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	s.render(w, "home", page{
		Title:       "Harbour Labs test site",
		Description: "A local web site with realistic pages for the chromedp example programs.",
		Active:      "/",
		CSS:         []string{"home.css"},
	})
}

func (s *server) tools(w http.ResponseWriter, r *http.Request) {
	s.render(w, "tools", page{
		Title:       "Tools",
		Description: "Pages for device emulation, printing, screenshots and lookups.",
		Active:      "/tools",
		CSS:         []string{"home.css"},
	})
}

func (s *server) studio(w http.ResponseWriter, r *http.Request) {
	s.render(w, "studio", page{
		Title:       "Northlight Studio",
		Description: "A landing page of a design studio, made for full page screenshots.",
		Active:      "/tools",
		CSS:         []string{"home.css", "studio.css"},
	})
}

func (s *server) whoami(w http.ResponseWriter, r *http.Request) {
	s.render(w, "ua", page{
		Title:       "What is my browser",
		Description: "Shows the user agent, the viewport, the pixel ratio and the touch support of the browser.",
		Active:      "/tools",
		CSS:         []string{"home.css", "ua.css"},
		JS:          []string{"ua.js"},
		Data:        r.UserAgent(),
	})
}

func (s *server) viewportTest(w http.ResponseWriter, r *http.Request) {
	s.render(w, "viewport", page{
		Title:       "Responsive layout",
		Description: "A page that changes its layout at phone, tablet and desktop widths.",
		Active:      "/tools",
		CSS:         []string{"home.css", "viewport.css"},
	})
}
