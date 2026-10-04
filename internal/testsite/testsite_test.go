package testsite

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The tests need no browser. They load the pages with an HTTP client and check
// the status, the landmarks and the sizes that the example programs rely on.

func get(t *testing.T, s *Site, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return resp, body
}

func page200(t *testing.T, s *Site, path string) string {
	t.Helper()
	resp, body := get(t, s, path)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, want 200", path, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET %s: content type %q, want text/html", path, ct)
	}
	return string(body)
}

func mustContain(t *testing.T, path, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("%s: missing %q", path, w)
		}
	}
}

func count(body, sub string) int { return strings.Count(body, sub) }

var (
	idRE       = regexp.MustCompile(`\sid="([^"]+)"`)
	externalRE = regexp.MustCompile(`(?:src|href)="(?:https?:)?//`)
)

func TestEveryPageIsValidAndLocal(t *testing.T) {
	s := New()
	defer s.Close()
	paths := []string{
		"/", "/tools", "/studio", "/ua", "/whoami", "/viewport-test", "/print/report", "/docs/", "/docs/time", "/wiki/",
		"/article/Harbour_Line", "/search?q=history", "/repo/", "/repo/chromedp/examples", "/gallery", "/weather/", "/weather/jakarta",
		"/geoip", "/geoip?ip=10.0.1.5", "/map", "/map?lat=51.5&lon=-0.12&zoom=9",
	}
	for _, w := range wikiTitles(s) {
		paths = append(paths, "/article/"+w)
	}
	for _, c := range cities {
		paths = append(paths, "/weather/"+c.Slug)
	}
	for _, path := range paths {
		body := page200(t, s, path)
		mustContain(t, path, body, "<!doctype html>", `<html lang="`, `<meta name="viewport"`, "<title>", `<header class="site-header">`, "<main", "<footer")
		if externalRE.MatchString(body) {
			t.Errorf("%s: the page refers to another host", path)
		}
		seen := map[string]bool{}
		for _, m := range idRE.FindAllStringSubmatch(body, -1) {
			if seen[m[1]] {
				t.Errorf("%s: the id %q appears twice", path, m[1])
			}
			seen[m[1]] = true
		}
		for _, m := range regexp.MustCompile(`<img [^>]*>`).FindAllString(body, -1) {
			if !strings.Contains(m, " alt=") {
				t.Errorf("%s: an image has no alt text: %s", path, m)
			}
		}
	}
}

func wikiTitles(s *Site) []string {
	var out []string
	for _, a := range newWiki().articles {
		out = append(out, a.Slug)
	}
	return out
}

func TestStaticFiles(t *testing.T) {
	s := New()
	defer s.Close()
	resp, body := get(t, s, "/static/site.css")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") {
		t.Fatalf("site.css: status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if n := count(string(body), "\n"); n < 200 {
		t.Errorf("site.css has %d lines, want at least 200", n)
	}
	for _, path := range []string{"/static/site.js", "/static/logo.svg", "/static/favicon.svg", "/static/icons/sun.svg", "/static/badges/build.svg", "/static/docs.css", "/static/map.js"} {
		if resp, _ := get(t, s, path); resp.StatusCode != 200 {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
	}
	// No style sheet loads a font or an image from another host.
	for _, name := range []string{"site", "docs", "wiki", "repo", "weather", "map", "home", "studio", "print", "gallery", "ua", "viewport"} {
		_, css := get(t, s, "/static/"+name+".css")
		if strings.Contains(string(css), "http") || strings.Contains(string(css), "@import") {
			t.Errorf("%s.css refers to another host", name)
		}
	}
}

func TestOtherOrigin(t *testing.T) {
	s := New()
	defer s.Close()
	if s.URL == s.OtherURL || !strings.HasPrefix(s.OtherURL, "http://localhost:") {
		t.Fatalf("URL %q and OtherURL %q must name different hosts", s.URL, s.OtherURL)
	}
	resp, err := http.Get(s.OtherURL + "/ua")
	if err != nil {
		t.Skipf("localhost does not work here: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("OtherURL: status %d", resp.StatusCode)
	}
}

func TestNotFound(t *testing.T) {
	s := New()
	defer s.Close()
	for _, path := range []string{"/nothing", "/article/No_Such", "/repo/a/b", "/weather/atlantis", "/tiles/2/9/9.png", "/images/missing.png"} {
		if resp, _ := get(t, s, path); resp.StatusCode != 404 {
			t.Errorf("%s: status %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestDocsTime(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/docs/time")
	mustContain(t, "/docs/time", body, `class="Documentation-overview"`, "<body", `<textarea class="Documentation-exampleCode"`, "time.After(500 * time.Millisecond)", "Example After")
	for _, id := range []string{"After", "Sleep", "Tick", "Date", "Now", "Since", "Parse", "ParseDuration", "NewTimer", "NewTicker", "AfterFunc", "Time.Format"} {
		want := fmt.Sprintf(`<details id="example-%s" class="Documentation-exampleDetails">`, id)
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if n := count(body, "<details "); n < 12 {
		t.Errorf("%d examples, want at least 12", n)
	}
	if n := count(body, `class="Documentation-entry`); n < 50 {
		t.Errorf("%d sections, want at least 50", n)
	}
	if !strings.Contains(body, "</main>\n<footer") {
		t.Error("footer is not a child of body that follows main")
	}
}

func TestArticle(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/article/Harbour_Line")
	mustContain(t, "article", body, `id="firstHeading"`, `id="mw-content-text"`, `class="infobox"`, `id="toc"`, "<blockquote", "<ol", "<ul", `id="see-also"`, `id="searchInput"`)
	if n := count(body, "<figure"); n < 8 {
		t.Errorf("%d figures, want at least 8", n)
	}
	if n := count(body, `<li id="cite_note-`); n < 40 {
		t.Errorf("%d references, want at least 40", n)
	}
	rows := count(body[strings.Index(body, `id="ridership"`):], "<tr>")
	if rows < 50 {
		t.Errorf("the data table has %d rows, want at least 50", rows)
	}
	if words := countWords(body[strings.Index(body, `id="mw-content-text"`):]); words < 5000 {
		t.Errorf("the article has %d words, want at least 5000", words)
	}
	// Every heading of the contents has a target.
	for _, m := range regexp.MustCompile(`<a href="#(s\d+(?:-\d+)?)">`).FindAllStringSubmatch(body, -1) {
		if !strings.Contains(body, `id="`+m[1]+`"`) {
			t.Errorf("the contents link to the missing id %s", m[1])
		}
	}
	// The first paragraph of the content is outside the infobox.
	content := body[strings.Index(body, `id="mw-content-text"`):]
	if i, j := strings.Index(content, "<p>"), strings.Index(content, "</table>"); i < j {
		t.Error("a paragraph appears before the end of the infobox")
	}
}

func TestSearch(t *testing.T) {
	s := New()
	defer s.Close()
	w := newWiki()
	if len(w.articles) < 30 || len(w.topics) != 8 {
		t.Fatalf("%d articles in %d topics, want 30 or more in 8", len(w.articles), len(w.topics))
	}
	for _, a := range w.articles {
		if a.Snippet == "" || a.Words < 500 {
			t.Errorf("%s: snippet %q, %d words", a.Title, a.Snippet, a.Words)
		}
	}
	body := page200(t, s, "/search?q=history")
	mustContain(t, "search", body, `id="firstHeading"`, `<ul class="mw-search-results">`, `id="searchInput"`, `<form class="search-form header-search" action="/search" method="get"`, `name="q"`)
	if n := count(body, `<li class="mw-search-result">`); n != 10 {
		t.Errorf("page 1 has %d results, want 10", n)
	}
	if n := count(body, `<div class="mw-search-result-heading"><a href="/article/`); n != 10 {
		t.Errorf("page 1 has %d result headings, want 10", n)
	}
	body2 := page200(t, s, "/search?q=history&page=2")
	if n := count(body2, `<li class="mw-search-result">`); n != 10 {
		t.Errorf("page 2 has %d results, want 10", n)
	}
	if body == body2 {
		t.Error("page 2 equals page 1")
	}
	// The first result opens an article.
	m := regexp.MustCompile(`<div class="mw-search-result-heading"><a href="(/article/[^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no first result")
	}
	if got := page200(t, s, m[1]); !strings.Contains(got, `id="firstHeading"`) {
		t.Error("the first result is not an article")
	}
	// A query with a topic word finds the articles of the topic first.
	rb := page200(t, s, "/search?q=railways")
	if !strings.Contains(rb, "Harbour Line") {
		t.Error("searching for railways does not find the Harbour Line")
	}
	for _, q := range []string{"", "zzzzqq"} {
		b := page200(t, s, "/search?q="+q)
		if count(b, `<li class="mw-search-result">`) != 0 {
			t.Errorf("query %q gave results", q)
		}
		mustContain(t, "search "+q, b, `id="firstHeading"`, `id="mw-content-text"`, `<p class="search-summary">`)
	}
	// The special page redirects to the search.
	if got := page200(t, s, "/wiki/Special:Search?q=tern"); !strings.Contains(got, "Dunmere Tern") {
		t.Error("Special:Search does not give the results of the search")
	}
}

func TestRepoPageAndZip(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/repo/chromedp/examples")
	mustContain(t, "repo", body, `<button type="button" class="btn small code-button"`, "<span>Code</span>", `<a href="/repo/chromedp/examples/archive/main.zip" download>`, "<span>Download ZIP</span>", `id="code-dropdown" hidden`, "README.md", "/static/badges/build.svg")
	if n := count(body, `<td class="name">`); n < 40 {
		t.Errorf("%d files, want at least 40", n)
	}
	resp, data := get(t, s, "/repo/chromedp/examples/archive/main.zip")
	if resp.StatusCode != 200 {
		t.Fatalf("zip status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("zip type %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Errorf("zip disposition %q", cd)
	}
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(data)) {
		t.Errorf("zip content length %q, body %d", cl, len(data))
	}
	if len(data) < 300*1024 {
		t.Errorf("zip has %d bytes, want at least 300 KB", len(data))
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var sawPNG bool
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Errorf("reading %s: %v", f.Name, err)
		}
		rc.Close()
		sawPNG = sawPNG || strings.HasSuffix(f.Name, ".png")
	}
	if len(zr.File) < 10 || !sawPNG {
		t.Errorf("zip has %d files, png %v", len(zr.File), sawPNG)
	}
}

func TestRepoList(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/repo/")
	if n := count(body, `<li data-stars=`); n < 30 {
		t.Errorf("%d projects, want at least 30", n)
	}
	// The structure that the program logic reads: a heading in a div, then a list
	// whose items hold a link and a text.
	mustContain(t, "repo list", body, `<div class="markdown-heading"><h3 class="heading-element" id="Selenium_and_browser_control_tools">Selenium and browser control tools</h3>`, "</div>\n    <ul>\n      <li data-stars=")
	re := regexp.MustCompile(`<li data-stars="[^"]*" data-lang="[^"]*"><a href="/repo/[^"]+">[^<]+</a> - [^<]+</li>`)
	if n := len(re.FindAllString(body, -1)); n < 30 {
		t.Errorf("%d items have the shape link, dash and text, want at least 30", n)
	}
	// every project page opens
	for _, m := range regexp.MustCompile(`<a href="(/repo/[^"]+)">`).FindAllStringSubmatch(body, -1) {
		page200(t, s, m[1])
	}
}

func TestGallery(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/gallery")
	if n := count(body, "<figure>"); n != 24 {
		t.Errorf("%d figures, want 24", n)
	}
	if n := count(body, `loading="lazy"`); n != 24 {
		t.Errorf("%d lazy images, want 24", n)
	}
	if n := count(body, " srcset="); n != 24 {
		t.Errorf("%d srcset attributes, want 24", n)
	}
	gal := loadGallery()
	kinds, large := map[string]int{}, 0
	minW, maxW := 1<<30, 0
	for _, g := range gal {
		kinds[g.Kind]++
		minW, maxW = min(minW, g.W), max(maxW, g.W)
		resp, data := get(t, s, "/images/"+g.Name)
		if resp.StatusCode != 200 {
			t.Errorf("%s: status %d", g.Name, resp.StatusCode)
			continue
		}
		if resp.Header.Get("Content-Length") != strconv.Itoa(len(data)) || len(data) != g.Size {
			t.Errorf("%s: content length %q, body %d, file %d", g.Name, resp.Header.Get("Content-Length"), len(data), g.Size)
		}
		want := map[string]string{"jpg": "image/jpeg", "png": "image/png", "svg": "image/svg+xml"}[g.Kind]
		if ct := resp.Header.Get("Content-Type"); ct != want {
			t.Errorf("%s: type %q, want %q", g.Name, ct, want)
		}
		if g.Large {
			large++
		}
	}
	if minW != 400 || maxW != 2000 {
		t.Errorf("widths from %d to %d, want 400 to 2000", minW, maxW)
	}
	if kinds["jpg"] == 0 || kinds["png"] == 0 || kinds["svg"] == 0 {
		t.Errorf("formats %v, want all three", kinds)
	}
	if large != 1 {
		t.Errorf("%d images larger than 1 MB, want 1", large)
	}
	// A smaller copy is smaller and keeps its type.
	resp, small := get(t, s, "/images/gallery-01.jpg?w=400")
	_, full := get(t, s, "/images/gallery-01.jpg")
	if resp.StatusCode != 200 || len(small) >= len(full)/4 {
		t.Errorf("w=400 gave %d bytes against %d", len(small), len(full))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(small)); err != nil || cfg.Width != 400 {
		t.Errorf("w=400 gave width %d, error %v", cfg.Width, err)
	}
	// The photo of 800 by 600 pixels.
	_, photo := get(t, s, "/images/photo.png")
	if cfg, err := png.DecodeConfig(bytes.NewReader(photo)); err != nil || cfg.Width != 800 || cfg.Height != 600 {
		t.Errorf("photo.png is %dx%d, error %v", cfg.Width, cfg.Height, err)
	}
	// The charts exist.
	for _, c := range []string{"bar", "line", "pie", "area", "scatter", "stacked"} {
		if resp, _ := get(t, s, "/images/chart-"+c+".svg"); resp.StatusCode != 200 {
			t.Errorf("chart %s: status %d", c, resp.StatusCode)
		}
	}
}

func TestWeather(t *testing.T) {
	s := New()
	defer s.Close()
	if len(cities) != 8 {
		t.Fatalf("%d cities, want 8", len(cities))
	}
	for _, c := range cities {
		body := page200(t, s, "/weather/"+c.Slug)
		mustContain(t, c.Slug, body, `id="taw"`, `id="wob_wc"`, `id="wob_loc">`+c.Name, `id="wob_tm"`, `id="wob_temp"`, `id="wob_rain"`, `id="wob_wind"`, "&deg;C", "&deg;F")
		if n := count(body, `role="tab"`); n != 3 {
			t.Errorf("%s: %d tabs, want 3", c.Slug, n)
		}
		for _, label := range []string{"Temperature", "Precipitation", "Wind"} {
			if !strings.Contains(body, `aria-label="`+label+`"`) {
				t.Errorf("%s: no tab with the label %s", c.Slug, label)
			}
		}
		for d := 0; d < 8; d++ {
			if !strings.Contains(body, fmt.Sprintf(`data-wob-di="%d"`, d)) {
				t.Errorf("%s: no button for day %d", c.Slug, d)
			}
		}
		if n := count(body, "<svg viewBox"); n != 24 {
			t.Errorf("%s: %d charts, want 24 (3 kinds for 8 days)", c.Slug, n)
		}
	}
	// the search form and its parameters
	body := page200(t, s, "/weather?q=Tokyo&unit=f&type=wind&day=2")
	mustContain(t, "weather search", body, `data-unit="f" data-type="wind" data-day="2"`, `id="wob_loc">Tokyo`)
	if body := page200(t, s, "/weather?q=Nowhere"); !strings.Contains(body, "No forecast") {
		t.Error("an unknown city gives no message")
	}
	// The forecast does not change between calls.
	a, b := forecast(cities[0]), forecast(cities[0])
	if a[3].HighC != b[3].HighC || a[3].TempSVG != b[3].TempSVG {
		t.Error("the forecast is not deterministic")
	}
}

func TestGeoIP(t *testing.T) {
	s := New()
	defer s.Close()
	if len(geoRanges) != 200 {
		t.Fatalf("%d ranges, want 200", len(geoRanges))
	}
	resp, body := get(t, s, "/api/geoip?ip=10.0.1.5")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var rec geoRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.IP != "10.0.1.5" || rec.Range != "10.0.0.0/16" || rec.Country == "" || rec.City == "" || rec.Timezone == "" || rec.Organization == "" || rec.Latitude == 0 || rec.Longitude == 0 {
		t.Errorf("record %+v is not complete", rec)
	}
	if resp, _ := get(t, s, "/api/geoip?ip=8.8.8.8"); resp.StatusCode != 404 {
		t.Errorf("a public address gave status %d, want 404", resp.StatusCode)
	}
	if resp, _ := get(t, s, "/api/geoip?ip=nonsense"); resp.StatusCode != 400 {
		t.Errorf("a bad address gave status %d, want 400", resp.StatusCode)
	}
	// With no ip, the answer is for the caller, which is a loopback address.
	if resp, body := get(t, s, "/api/geoip"); resp.StatusCode != 200 || !strings.Contains(string(body), `"ip": "127.0.0.1"`) {
		t.Errorf("no ip: status %d, body %s", resp.StatusCode, body)
	}
	// Every range answers for its first address and for a documentation address.
	_, all := get(t, s, "/api/geoip/ranges")
	var recs []geoRecord
	if err := json.Unmarshal(all, &recs); err != nil || len(recs) != 200 {
		t.Fatalf("%d ranges in the list, error %v", len(recs), err)
	}
	if rec, ok := lookup(mustAddr("203.0.113.77")); !ok || rec.Range != "203.0.113.64/26" {
		t.Errorf("documentation address: %+v", rec)
	}
	if body := page200(t, s, "/geoip?ip=192.168.3.9"); !strings.Contains(body, `href="/map?lat=`) {
		t.Error("the lookup page has no link to the map")
	}
}

func TestMapAndTiles(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/map?lat=-6.2088&lon=106.8456&zoom=12.5")
	mustContain(t, "map", body, `id="app-container"`, `id="map"`, `data-lat="-6.2088"`, `data-zoom="12.5"`, `id="zoom-in"`, `id="zoom-out"`, `/static/map.js`)
	var first []byte
	for _, p := range []string{"/tiles/0/0/0.png", "/tiles/3/5/2.png", "/tiles/12/3412/2031.png"} {
		resp, data := get(t, s, p)
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" || resp.Header.Get("Content-Length") != strconv.Itoa(len(data)) {
			t.Errorf("%s: status %d, type %q", p, resp.StatusCode, resp.Header.Get("Content-Type"))
			continue
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != 256 || cfg.Height != 256 {
			t.Errorf("%s: %dx%d, error %v", p, cfg.Width, cfg.Height, err)
		}
		if first == nil {
			first = data
		} else if bytes.Equal(first, data) {
			t.Errorf("%s equals the first tile", p)
		}
	}
	for _, p := range []string{"/tiles/1/2/0.png", "/tiles/1/0/0", "/tiles/x/0/0.png", "/tiles/20/0/0.png"} {
		if resp, _ := get(t, s, p); resp.StatusCode != 404 {
			t.Errorf("%s: status %d, want 404", p, resp.StatusCode)
		}
	}
	// The script writes the position into the address in the form that the latlon program reads.
	_, js := get(t, s, "/static/map.js")
	mustContain(t, "map.js", string(js), `"@" + state.lat.toFixed(5)`, "history.replaceState", "hashchange")
}

func TestDeviceAndPrintPages(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/ua")
	mustContain(t, "ua", body, `id="ua"`, `id="viewport"`, `id="dpr"`, `id="touch"`, "/static/ua.js")
	_, js := get(t, s, "/static/ua.js")
	mustContain(t, "ua.js", string(js), "navigator.userAgent", "innerWidth", "devicePixelRatio", "maxTouchPoints")
	body = page200(t, s, "/viewport-test")
	mustContain(t, "viewport", body, "/static/viewport.css")
	_, css := get(t, s, "/static/viewport.css")
	for _, q := range []string{"max-width: 480px", "max-width: 1099px"} {
		if !strings.Contains(string(css), q) {
			t.Errorf("viewport.css has no media query %q", q)
		}
	}
	body = page200(t, s, "/print/report")
	if n := count(body, `<section class="chapter"`); n < 8 {
		t.Errorf("%d chapters, want at least 8", n)
	}
	if n := count(body, "<table>"); n < 8 {
		t.Errorf("%d tables, want at least 8", n)
	}
	_, css = get(t, s, "/static/print.css")
	mustContain(t, "print.css", string(css), "@page", "break-before: page", "position: fixed", "@media print")
	if body := page200(t, s, "/studio"); count(body, "<section") < 8 {
		t.Error("the studio page is short")
	}
}

func TestHomeLinksEverything(t *testing.T) {
	s := New()
	defer s.Close()
	body := page200(t, s, "/")
	for _, link := range []string{"/docs/time", "/wiki/", "/article/Harbour_Line", "/repo/chromedp/examples", "/repo/", "/gallery", "/weather/jakarta", "/map?", "/geoip", "/ua", "/viewport-test", "/print/report", "/studio"} {
		if !strings.Contains(body, `href="`+link) {
			t.Errorf("the home page has no link to %s", link)
		}
	}
	// Every local link of the home page works.
	for _, m := range regexp.MustCompile(`href="(/[^"#]*)"`).FindAllStringSubmatch(body, -1) {
		path := strings.ReplaceAll(m[1], "&amp;", "&")
		if resp, _ := get(t, s, path); resp.StatusCode != 200 && resp.StatusCode != 302 {
			t.Errorf("link %s: status %d", path, resp.StatusCode)
		}
	}
}

func TestEmbeddedSize(t *testing.T) {
	var total int64
	for _, fsys := range []fs.FS{imageFS, staticFS, templateFS} {
		err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if total > 8<<20 {
		t.Errorf("the embedded files hold %d bytes, want less than 8 MB", total)
	}
	t.Logf("embedded files: %d bytes", total)
}

func mustAddr(s string) netip.Addr { return netip.MustParseAddr(s) }
