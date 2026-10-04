// Command pdfoptions is a chromedp example demonstrating how to print a page to
// PDF files with different options of chromedp.PrintToPDF. The program serves a
// document of five pages from a local server and prints it in several variants:
// the default, landscape, A4 with margins and backgrounds, a header and a
// footer with the title and the page number, a scale of 2, only the pages 2 to
// 3, and a file with an outline and tags that the browser sends as a stream. It
// also prints a page that sets its own paper size with the CSS rule @page, and
// uses the option PDFPreferCSSPageSize for it. The program writes the files in
// the directory of the flag -out, or in a new temporary directory that it
// prints. For each file it prints the number of pages and the size of the first
// page, which it reads from the file itself. It starts a local server and needs
// no internet. Use -v to print the protocol messages, -visible to show the
// browser window and leave it open, and -visible-on-terminal to draw the page
// in the terminal with terminal graphics.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

// variant is one way to print a page.
type variant struct {
	// Name is the name of the file, without the extension.
	Name string

	// Path is the page of the test server to print.
	Path string

	// Options are the options of PrintToPDF.
	Options []chromedp.PDFOption
}

// footer has the number of the page and the number of all the pages. The
// browser fills the elements with the classes pageNumber and totalPages. The
// browser uses a font size of 0 in a header, so the template sets one.
const footer = `<div style="font-size: 9px; width: 100%; text-align: right; margin-right: 1cm">
page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`

// header has the title of the page, which comes from its title element.
const header = `<div style="font-size: 9px; width: 100%; text-align: center"><span class="title"></span></div>`

// variants are the PDF files that the program makes. The sizes are in inches,
// because the protocol uses them.
var variants = []variant{
	// With no option, the browser prints on Letter paper, in portrait, with
	// margins of 1 cm and with no background.
	{Name: "default", Path: "/"},
	{Name: "landscape", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFLandscape(),
	}},
	// The margins go in the order top, right, bottom and left.
	{Name: "a4-margins", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFMargins(0.5, 1, 0.5, 1),
		chromedp.PDFPrintBackground(),
	}},
	// A header or a footer turns both on. The margins must be large enough
	// for them, because they are drawn in the margins.
	{Name: "header-footer", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFMargins(0.8, 0.5, 0.8, 0.5),
		chromedp.PDFHeaderTemplate(header),
		chromedp.PDFFooterTemplate(footer),
	}},
	// A scale of 2 makes the content twice as large, so a section that fitted
	// on one page needs two.
	{Name: "scale-2", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFScale(2),
	}},
	{Name: "pages-2-3", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFPageRanges("2-3"),
	}},
	// A PDFOption is a func that changes the parameters of the protocol
	// command, so a program can write one for a field that has no function.
	// This one turns the tags off. The browser of today makes a tagged PDF
	// when no option says otherwise.
	{Name: "untagged", Path: "/", Options: []chromedp.PDFOption{
		func(p *page.PrintToPDFParams) { p.GenerateTaggedPDF = new(false) },
	}},
	// The outline is the table of contents of a viewer, from the headings.
	// The tags give the structure to a screen reader. The stream changes how
	// the browser sends the file, and not the file.
	{Name: "outline-tagged", Path: "/", Options: []chromedp.PDFOption{
		chromedp.PDFOutlineAndTagged(),
		chromedp.PDFStream(),
	}},
	// The page /sized has the rule @page { size: 6in 4in }. The browser uses
	// it only with the option PDFPreferCSSPageSize.
	{Name: "css-page-size", Path: "/sized", Options: []chromedp.PDFOption{
		chromedp.PDFPreferCSSPageSize(),
	}},
	// Without the option, the paper stays Letter. The browser only turns it to
	// landscape, because the rule is wider than it is high.
	{Name: "css-page-off", Path: "/sized"},
}

func main() {
	var tc termcast.Flags
	out := flag.String("out", "", "directory for the PDF files, a new temporary directory by default")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	tc.Register(flag.CommandLine)
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// draw the page in the terminal if the flag -visible-on-terminal is set
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	stdout := io.Writer(os.Stdout)
	if s != nil {
		stdout = s.LogWriter()
	}

	if err := run(ctx, srv.URL, *out, stdout); err != nil {
		s.Fatal(err)
	}
}

// run prints each variant to a file, and reports what the file holds.
func run(ctx context.Context, host, dir string, stdout io.Writer) error {
	if dir == "" {
		d, err := os.MkdirTemp("", "pdfoptions-")
		if err != nil {
			return fmt.Errorf("creating a temporary directory: %w", err)
		}
		dir = d
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating the directory %s: %w", dir, err)
	}
	fmt.Fprintf(stdout, "writing the PDF files in %s\n", dir)

	for _, v := range variants {
		// Wait until the page has loaded, because PrintToPDF prints what
		// the browser has now.
		if err := chromedp.Do(ctx, chromedp.Navigate(host+v.Path)); err != nil {
			return fmt.Errorf("loading %s: %w", v.Path, err)
		}
		buf, err := chromedp.Run(ctx, chromedp.PrintToPDF(v.Options...))
		if err != nil {
			return fmt.Errorf("printing %s: %w", v.Name, err)
		}
		name := filepath.Join(dir, v.Name+".pdf")
		if err := os.WriteFile(name, buf, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		info, err := scan(buf)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		fmt.Fprintf(stdout, "%-19s %2d %-5s %3.0f x %3.0f pt (%.2f x %.2f in), outline %-5t tagged %t\n",
			v.Name+".pdf", info.Pages, plural(info.Pages), info.Width, info.Height, info.Width/72, info.Height/72, info.Outline, info.Tagged)
	}
	return nil
}

// plural returns the word for the pages.
func plural(n int) string {
	if n == 1 {
		return "page"
	}
	return "pages"
}

// info is what scan finds in a PDF file.
type info struct {
	Pages         int
	Width, Height float64 // in points, 1/72 of an inch
	Outline       bool
	Tagged        bool
}

var (
	pageObject = regexp.MustCompile(`/Type\s*/Page[^s]`)
	mediaBox   = regexp.MustCompile(`/MediaBox\s*\[\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\]`)
)

// scan reads the number of pages and the size of the first page from the bytes
// of a PDF file. This is a shortcut, and not a PDF parser. It counts the
// objects with /Type /Page, which is each page, and does not count /Type
// /Pages, which is the tree of the pages. It reads the first /MediaBox, which
// is the paper size in points. It works for the files of Chrome, which keeps
// these parts of the file as plain text. A file from another program can hold
// them in a compressed stream, and the scan finds nothing. A real program that
// reads PDF files uses a PDF library.
func scan(pdf []byte) (info, error) {
	var in info
	in.Pages = len(pageObject.FindAll(pdf, -1))
	m := mediaBox.FindSubmatch(pdf)
	if m == nil {
		return in, fmt.Errorf("no /MediaBox in %d bytes", len(pdf))
	}
	var err error
	if in.Width, err = strconv.ParseFloat(string(m[3]), 64); err != nil {
		return in, fmt.Errorf("reading the width of the page: %w", err)
	}
	if in.Height, err = strconv.ParseFloat(string(m[4]), 64); err != nil {
		return in, fmt.Errorf("reading the height of the page: %w", err)
	}
	in.Outline = bytes.Contains(pdf, []byte("/Outlines"))
	in.Tagged = bytes.Contains(pdf, []byte("/StructTreeRoot"))
	return in, nil
}

// newMux returns the handlers of the test server. The page / is a document of
// five sections. A section is 6 inches high and ends with a page break, so the
// document has five pages with the default options. The page /sized has one
// page with its own paper size.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, document)
	})
	mux.HandleFunc("/sized", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><head><title>sized page</title>
<style>@page { size: 6in 4in; margin: 0.5in }</style></head>
<body><h1>A page of 6 by 4 inches</h1></body></html>`)
	})
	return mux
}

// document is the HTML of the test page. The sections have colors, which show
// only when the program prints the backgrounds. The last section has no page
// break, so it does not add an empty page.
var document = func() string {
	var b bytes.Buffer
	b.WriteString(`<html><head><title>Quarterly report</title>
<style>
body { font-family: sans-serif; margin: 0 }
section { height: 6in; break-after: page; padding: 0.2in }
section:last-child { break-after: auto }
</style></head><body>
`)
	colors := []string{"#fdd", "#dfd", "#ddf", "#ffd", "#dff"}
	for i, c := range colors {
		fmt.Fprintf(&b, "<section style=\"background: %s\"><h1>Chapter %d</h1><p>Text of chapter %d.</p></section>\n", c, i+1, i+1)
	}
	b.WriteString("</body></html>")
	return b.String()
}()
