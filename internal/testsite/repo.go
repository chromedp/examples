package testsite

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html/template"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// repoItem is a project in the list of repositories.
type repoItem struct {
	Owner, Name, Desc string
	Stars             int
	Lang              string
}

// repoSection groups projects under a heading, like a curated list.
type repoSection struct {
	Title string
	Slug  string
	Items []repoItem
}

var repoSections = []repoSection{
	{Title: "Selenium and browser control tools", Items: []repoItem{
		{"chromedp", "chromedp", "Drive a browser with Go through the DevTools Protocol.", 21400, "Go"},
		{"chromedp", "examples", "Larger programs that show how to use chromedp for real tasks.", 1180, "Go"},
		{"chromedp", "cdproto", "Generated Go types and commands for the DevTools Protocol.", 360, "Go"},
		{"chromedp", "remote", "Start a browser, and keep it open after the program ends.", 95, "Go"},
		{"harbour", "tideclick", "Record a browser session and replay it as a test.", 2480, "Go"},
		{"harbour", "lanternkit", "A small pool of headless browsers for parallel tests.", 740, "Go"},
	}},
	{Title: "Command line", Items: []repoItem{
		{"harbour", "fjord", "Build a command line tool with sub commands and flags.", 8120, "Go"},
		{"harbour", "pennant", "Show a progress bar and a spinner in the terminal.", 3350, "Go"},
		{"nils-orme", "tabula", "Print a table with aligned columns and colors.", 1670, "Go"},
		{"kira-pike", "shellmark", "Mark and jump between the folders that you use most.", 2210, "Rust"},
		{"tilda-wick", "hushrc", "Read a configuration from files, flags and the environment.", 4390, "Go"},
	}},
	{Title: "Databases", Items: []repoItem{
		{"oakhill", "burrow", "An embedded key and value store with transactions.", 9840, "Go"},
		{"oakhill", "ledgerdb", "An append-only log with fast range queries.", 3120, "Go"},
		{"ines-carver", "pgbridge", "A connection pool with a health check for Postgres.", 2760, "Go"},
		{"ellery", "sqlforge", "Generate type safe Go code from SQL queries.", 11200, "Go"},
		{"pellam", "cachet", "A cache with a time limit and a size limit for each key.", 1940, "Go"},
	}},
	{Title: "Web frameworks", Items: []repoItem{
		{"garrow", "quayside", "A small router with middleware and path parameters.", 14500, "Go"},
		{"garrow", "lanterns", "Server side templates with partial reloads.", 2630, "Go"},
		{"mara-hale", "sessionkeep", "Signed cookies and server side sessions.", 1290, "Go"},
		{"hollin", "restwright", "Describe an API once and get a server and a client.", 6710, "Go"},
		{"jonas-kemp", "fastlane", "A static file server with range requests and compression.", 880, "Go"},
	}},
	{Title: "Testing", Items: []repoItem{
		{"stanway", "assertly", "Assertions with readable failure messages.", 7430, "Go"},
		{"stanway", "mockery-lite", "Generate mocks from interfaces without a runtime cost.", 3980, "Go"},
		{"rosa-dunn", "goldfile", "Compare the output of a program with a golden file.", 1560, "Go"},
		{"felix-mercer", "fuzzbox", "Seed corpora and helpers for fuzz tests.", 940, "Go"},
		{"vexford", "clockwork-test", "A fake clock for tests that depend on time.", 5120, "Go"},
	}},
	{Title: "Images and graphics", Items: []repoItem{
		{"lowmoor", "pixelmill", "Resize, crop and convert images with a simple API.", 6330, "Go"},
		{"lowmoor", "svgdraw", "Draw charts and shapes as SVG from code.", 2870, "Go"},
		{"greta-rudd", "paletta", "Extract the main colors of a picture.", 1130, "Go"},
		{"hugo-tarn", "tilecache", "Render and cache map tiles on demand.", 1760, "Go"},
		{"underhill", "thumbnailer", "Make thumbnails in a pool of workers.", 2240, "Go"},
	}},
	{Title: "Text processing", Items: []repoItem{
		{"ravensmoor", "lexis", "Split text into words and sentences in many languages.", 4810, "Go"},
		{"ravensmoor", "mdweave", "Turn Markdown into HTML with custom blocks.", 9260, "Go"},
		{"clara-ingram", "diffly", "Show the difference of two texts, line by line.", 3470, "Go"},
		{"dmitri-nash", "slugify-all", "Make a clean URL slug from any text.", 1020, "Go"},
		{"thorne", "fuzzmatch", "Find the closest match of a word in a long list.", 2680, "Go"},
	}},
	{Title: "Networking", Items: []repoItem{
		{"netherby", "relay", "A reverse proxy with retries and a circuit breaker.", 12900, "Go"},
		{"netherby", "wirewatch", "Record and replay HTTP traffic for tests.", 3640, "Go"},
		{"olga-brook", "dnsmith", "A tiny DNS server for local development.", 2150, "Go"},
		{"quinn-sutton", "pingwell", "Measure latency and loss from many places.", 1480, "Go"},
		{"eastwick", "socketry", "A framework for long lived connections with heartbeats.", 5980, "Go"},
	}},
}

func init() {
	for i := range repoSections {
		repoSections[i].Slug = slugOf(repoSections[i].Title)
	}
}

type repoFile struct {
	Name string
	Dir  bool
	Msg  string
	Age  string
}

type repoView struct {
	Owner, Name, Desc string
	Stars, Forks      int
	Watchers          int
	Issues, PRs       int
	Commits           int
	Lang              string
	Topics            []string
	Files             []repoFile
	Readme            template.HTML
	Examples          bool
	Branch            string
	ZIPURL            string
}

type repoListData struct {
	Sections []repoSection
	Total    int
}

// examplePrograms are the folders of the chromedp/examples repository.
var examplePrograms = []struct{ Name, Desc string }{
	{"click", "Click an element and read the text of the page."},
	{"console", "Print the messages of the console of the page."},
	{"cookie", "Set a cookie and read the cookies of the browser."},
	{"dialogs", "Answer an alert, a confirm and a prompt dialog."},
	{"download_file", "Download a file that the page offers."},
	{"download_image", "Save an image from the body of a response."},
	{"dragdrop", "Drag a slider, a list item and a card."},
	{"emulate", "Emulate a phone and take a screenshot."},
	{"eval", "Run a JavaScript expression in the page."},
	{"eventsiter", "Read the events of the page in a loop."},
	{"exposefunc", "Call a Go function from the page."},
	{"fast", "Measure the speed of a connection."},
	{"forecast", "Take a screenshot of a weather forecast."},
	{"frames", "Reach into frames and shadow roots."},
	{"geoip", "Look up an address and show its map."},
	{"har", "Write a HAR archive of the requests."},
	{"headers", "Send an extra header with every request."},
	{"intercept", "Block, mock and change requests."},
	{"keys", "Type into inputs and select an option."},
	{"latlon", "Read the position from the address of a map."},
	{"logic", "Combine actions and Go code to read a list."},
	{"multi", "Take screenshots of many pages."},
	{"pdf", "Print a page to a PDF file."},
	{"pdfoptions", "Try the options of the PDF printer."},
	{"pdfstream", "Stream a PDF from the browser."},
	{"popups", "Handle windows that a page opens."},
	{"proxy", "Use a proxy that asks for a password."},
	{"rawcall", "Send a protocol command by hand."},
	{"remote", "Attach to a browser that already runs."},
	{"screencast", "Save the frames of a screencast."},
	{"screenshot", "Take screenshots of an element and a page."},
	{"selectors", "Find nodes with every kind of selector."},
	{"session", "Save and restore the state of a session."},
	{"structeval", "Move Go values in and out of the page."},
	{"submit", "Fill a form and submit it."},
	{"subtree", "Print the tree of an element."},
	{"tabs", "Work with many tabs and windows."},
	{"text", "Read the text of an element."},
	{"upload", "Upload a file with a form."},
	{"visible", "Wait for an element to become visible."},
	{"workers", "Run jobs in a pool of tabs."},
}

func examplesView() repoView {
	v := repoView{
		Owner: "chromedp", Name: "examples",
		Desc:  "Larger programs that show how to use chromedp for real tasks.",
		Stars: 1180, Forks: 214, Watchers: 37, Issues: 9, PRs: 3, Commits: 482, Lang: "Go",
		Topics:   []string{"golang", "chrome", "devtools-protocol", "automation", "examples"},
		Examples: true, Branch: "main",
	}
	ages := []string{"2 days ago", "last week", "3 weeks ago", "2 months ago", "5 months ago", "last year"}
	for i, p := range examplePrograms {
		v.Files = append(v.Files, repoFile{Name: p.Name, Dir: true, Msg: p.Desc, Age: ages[i%len(ages)]})
	}
	for _, f := range []repoFile{
		{Name: "docs", Dir: true, Msg: "Record the decision about the local test site", Age: "today"},
		{Name: "internal", Dir: true, Msg: "Add the shared test site", Age: "today"},
		{Name: ".gitignore", Msg: "Ignore the output of the programs", Age: "last year"},
		{Name: "AGENTS.md", Msg: "Describe the rules for coding agents", Age: "2 days ago"},
		{Name: "CONTRIBUTING.md", Msg: "Explain how to send a change", Age: "5 months ago"},
		{Name: "LICENSE", Msg: "Add the license", Age: "3 years ago"},
		{Name: "README.md", Msg: "Update the table of the programs", Age: "2 days ago"},
		{Name: "go.mod", Msg: "Move to the new typed API", Age: "last week"},
		{Name: "go.sum", Msg: "Move to the new typed API", Age: "last week"},
	} {
		v.Files = append(v.Files, f)
	}
	var b strings.Builder
	b.WriteString(`<h1>About chromedp examples</h1>
<p class="badges"><img src="/static/badges/build.svg" alt="build: passing" height="20"> <img src="/static/badges/coverage.svg" alt="coverage: 87%" height="20"> <img src="/static/badges/release.svg" alt="release: v0.19.0" height="20"> <img src="/static/badges/license.svg" alt="license: MIT" height="20"> <img src="/static/badges/reference.svg" alt="reference: go.dev" height="20"> <img src="/static/badges/chat.svg" alt="chat: discord" height="20"></p>
<p>This repository holds 41 example programs for <code>chromedp</code>, a Go package that drives a browser through the DevTools Protocol. The programs are larger than the examples in the documentation of the package. Each one solves a task that people ask about.</p>
<h2>Build and run</h2>
<p>The module needs a recent version of Go. Run a program from the root of the repository.</p>
<pre><code>$ go run ./click
$ go build -o /tmp/screenshot ./screenshot &amp;&amp; /tmp/screenshot</code></pre>
<p>Every program accepts the flag <code>-v</code>, which prints the protocol messages, and the flag <code>-visible</code>, which shows the browser window.</p>
<h2>The programs</h2>
<table><thead><tr><th scope="col">Program</th><th scope="col">What it shows</th></tr></thead><tbody>`)
	for _, p := range examplePrograms {
		fmt.Fprintf(&b, `<tr><td><a href="/repo/chromedp/examples/tree/main/%s"><code>%s</code></a></td><td>%s</td></tr>`, p.Name, p.Name, p.Desc)
	}
	b.WriteString(`</tbody></table>
<h2>Contributing</h2>
<p>Read <code>CONTRIBUTING.md</code> before you open a pull request. Questions go to the discussions of the main repository, and bugs go to its issues.</p>
<h2>License</h2>
<p>The programs are free software under the MIT license.</p>`)
	v.Readme = template.HTML(b.String())
	return v
}

// genericView builds the page of any other repository of the list.
func genericView(it repoItem) repoView {
	r := rand.New(rand.NewPCG(uint64(len(it.Name))*131+uint64(it.Stars), 5))
	v := repoView{
		Owner: it.Owner, Name: it.Name, Desc: it.Desc, Stars: it.Stars,
		Forks: it.Stars / 9, Watchers: it.Stars / 40, Issues: 3 + r.IntN(60), PRs: r.IntN(15), Commits: 80 + r.IntN(2400),
		Lang: it.Lang, Topics: []string{"go", "library", strings.ToLower(it.Lang), "open-source"}, Branch: "main",
	}
	names := []string{"cmd", "internal", "docs", "testdata", ".github", ".gitignore", "CHANGELOG.md", "LICENSE", "Makefile", "README.md", "go.mod", "go.sum", it.Name + ".go", it.Name + "_test.go"}
	msgs := []string{"Fix a data race in the pool", "Add a test for the empty case", "Update the dependencies", "Explain the flags in the docs", "Rename the option and keep the old one", "Handle a closed connection", "Prepare the next release"}
	ages := []string{"yesterday", "3 days ago", "last week", "2 weeks ago", "last month", "4 months ago"}
	for i, n := range names {
		v.Files = append(v.Files, repoFile{Name: n, Dir: i < 4, Msg: msgs[r.IntN(len(msgs))], Age: ages[r.IntN(len(ages))]})
	}
	v.Readme = template.HTML(fmt.Sprintf(`<h1>%s</h1>
<p>%s</p>
<h2>Install</h2>
<pre><code>go get github.com/%s/%s</code></pre>
<h2>Use</h2>
<p>Import the package and call <code>New</code> to create a value. The package has no global state, so you can create as many values as you need.</p>
<h2>License</h2>
<p>MIT.</p>`, it.Name, it.Desc, it.Owner, it.Name))
	return v
}

func (s *server) repoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /repo/{$}", s.repoList)
	mux.HandleFunc("GET /repo/{owner}/{name}", s.repoPage)
	mux.HandleFunc("GET /repo/{owner}/{name}/archive/main.zip", s.repoZip)
	mux.HandleFunc("GET /repo/{owner}/{name}/tree/main/{path...}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/repo/"+r.PathValue("owner")+"/"+r.PathValue("name"), http.StatusFound)
	})
}

func findRepo(owner, name string) (repoItem, bool) {
	for _, sec := range repoSections {
		for _, it := range sec.Items {
			if it.Owner == owner && it.Name == name {
				return it, true
			}
		}
	}
	return repoItem{}, false
}

func (s *server) repoList(w http.ResponseWriter, r *http.Request) {
	total := 0
	for _, sec := range repoSections {
		total += len(sec.Items)
	}
	s.render(w, "repo_list", page{
		Title:       "Projects",
		Description: "A curated list of " + strconv.Itoa(total) + " projects in eight sections.",
		Active:      "/repo/",
		CSS:         []string{"repo.css"},
		Data:        repoListData{Sections: repoSections, Total: total},
	})
}

func (s *server) repoPage(w http.ResponseWriter, r *http.Request) {
	owner, name := r.PathValue("owner"), r.PathValue("name")
	var v repoView
	switch it, ok := findRepo(owner, name); {
	case owner == "chromedp" && name == "examples":
		v = examplesView()
	case ok:
		v = genericView(it)
	default:
		s.notFound(w, r)
		return
	}
	v.ZIPURL = "/repo/" + owner + "/" + name + "/archive/main.zip"
	s.render(w, "repo", page{
		Title:       owner + "/" + name,
		Description: v.Desc,
		Active:      "/repo/",
		CSS:         []string{"repo.css"},
		JS:          []string{"repo.js"},
		Data:        v,
	})
}

var (
	zipMu    sync.Mutex
	zipCache = map[string][]byte{}
)

func (s *server) repoZip(w http.ResponseWriter, r *http.Request) {
	owner, name := r.PathValue("owner"), r.PathValue("name")
	if _, ok := findRepo(owner, name); !ok {
		s.notFound(w, r)
		return
	}
	zipMu.Lock()
	data, ok := zipCache[owner+"/"+name]
	if !ok {
		var err error
		data, err = buildZip(name + "-main")
		if err != nil {
			zipMu.Unlock()
			http.Error(w, "zip error", http.StatusInternalServerError)
			return
		}
		zipCache[owner+"/"+name] = data
	}
	zipMu.Unlock()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-main.zip"`, name))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// buildZip makes an archive of a few real files below the folder root. It
// holds an image, a table of data and the Go programs of the documentation
// examples, so it is large enough to take a moment to download.
func buildZip(root string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	stamp := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	add := func(name string, data []byte, method uint16) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: root + "/" + name, Method: method, Modified: stamp})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err := add("README.md", []byte("# "+root+"\n\nThis archive comes from the local test site.\nIt holds a few files so that a program can download and open it.\n"), zip.Deflate); err != nil {
		return nil, err
	}
	if err := add("LICENSE", []byte("MIT License\n\nPermission is granted to use this test data for any purpose.\n"), zip.Deflate); err != nil {
		return nil, err
	}
	if err := add("go.mod", []byte("module example.com/"+root+"\n\ngo 1.27\n"), zip.Deflate); err != nil {
		return nil, err
	}
	for _, ex := range docExamples {
		if err := add("docs/examples/"+strings.ReplaceAll(strings.TrimPrefix(ex.ID, "example-"), ".", "_")+".go", []byte(ex.Code), zip.Deflate); err != nil {
			return nil, err
		}
	}
	photo, err := imageFS.ReadFile("images/photo.png")
	if err != nil {
		return nil, err
	}
	if err := add("docs/photo.png", photo, zip.Store); err != nil {
		return nil, err
	}
	if err := add("docs/data.csv", dataCSV(), zip.Deflate); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// dataCSV writes a table of 8000 rows. It compresses well.
func dataCSV() []byte {
	r := rand.New(rand.NewPCG(3, 4))
	var b bytes.Buffer
	b.WriteString("id,depot,month,parcels,late,weight_kg\n")
	for i := 1; i <= 8000; i++ {
		fmt.Fprintf(&b, "%d,%s,%d,%d,%d,%.1f\n", i, depots[r.IntN(len(depots))], 1+r.IntN(12), 100+r.IntN(9000), r.IntN(120), r.Float64()*5000)
	}
	return b.Bytes()
}
