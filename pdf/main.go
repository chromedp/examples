// Command pdf is a chromedp example demonstrating how to capture a PDF of a
// page. It starts a local server and needs no internet. The program prints the
// report /print/report of the local test site, which has a cover, 8 chapters and
// 8 tables, in four ways: with no option, with the paper size and the margins
// of the page itself, as A4 in portrait with margins, a header, a footer and
// backgrounds, and as A4 in landscape. It writes the files to the directory of
// the flag -out, which is the current directory by default, and for each file it
// prints the number of pages, the paper size and the size in bytes. The flag
// -url gives the full URL of the page to print, for example a live site, and
// then the program does not start the local site. The page of the local site
// has the rules @page and @media print that suit it, so a live site can give a
// different result. Use -v to print the protocol messages and -visible to show
// the browser window and leave it open.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
)

// header has the title of the page, which the browser reads from the title
// element. The browser gives a header a font size of 0, so the template sets
// one. The browser draws the header in the top margin and the footer in the
// bottom margin, so the margins must be large enough for them.
const header = `<div style="font-size: 9px; width: 100%; text-align: center"><span class="title"></span></div>`

// footer has the number of the page and the number of all the pages. The
// browser fills the elements with the classes pageNumber and totalPages.
const footer = `<div style="font-size: 9px; width: 100%; text-align: right; margin-right: 1cm">
page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`

// variant is one way to print the page.
type variant struct {
	// Name is the name of the file, without the extension.
	Name string

	// Neutral removes the print rules of the page before it prints, see
	// neutralize.
	Neutral bool

	// Options are the options of PrintToPDF.
	Options []chromedp.PDFOption
}

// variants are the PDF files that the program makes. The sizes of the paper and
// the margins are in inches, because the protocol uses them.
var variants = []variant{
	// With no option, the browser prints on Letter paper, in portrait, with
	// margins of 1 cm and with no background. It scales the page to fit the
	// paper and ignores the rule @page of the page.
	{Name: "default"},

	// The page has the rule @page { size: A4; margin: 24mm 16mm 22mm }. The
	// browser uses it only with this option.
	{Name: "css-page", Options: []chromedp.PDFOption{
		chromedp.PDFPreferCSSPageSize(),
	}},

	// A4 in portrait. The margins go in the order top, right, bottom and
	// left. A header or a footer turns both on, and the browser draws them
	// in the margins. The background colors and images print only with
	// PDFPrintBackground. The page must not set its own paper size, so this
	// variant is neutral.
	{Name: "a4", Neutral: true, Options: []chromedp.PDFOption{
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFMargins(0.9, 0.6, 0.9, 0.6),
		chromedp.PDFPrintBackground(),
		chromedp.PDFHeaderTemplate(header),
		chromedp.PDFFooterTemplate(footer),
	}},

	// Landscape swaps the width and the height of the paper. With a rule
	// @page that sets the size, the option has no effect, so this variant is
	// neutral too.
	{Name: "a4-landscape", Neutral: true, Options: []chromedp.PDFOption{
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFLandscape(),
		chromedp.PDFMargin(0.6),
		chromedp.PDFPrintBackground(),
	}},
}

func main() {
	urlstr := flag.String("url", "", "full URL of the page to print, for example a live site. The local test site starts when it is empty. The page of the local site has its own print rules and a live site can differ")
	out := flag.String("out", ".", "directory for the PDF files")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// choose the page. Without -url, the program starts the local site
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/print/report"
	}

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

	// create a timeout, so that no wait loop can run forever
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := run(ctx, *urlstr, *out); err != nil {
		log.Fatal(err)
	}
}

// neutralize removes the rules @page of the page, and hides its own header and
// footer. The page of the local site sets the paper size and the margins with
// @page, and the browser lets such a rule win over PDFLandscape and the margins
// of the options. The page also draws a running header and footer of its own,
// which would collide with the ones of the options. The script reads the
// style sheets of the page, which is possible because they come from the same
// origin as the page. It returns the number of the rules that it removed.
const neutralize = `(() => {
	let removed = 0;
	for (const sheet of document.styleSheets) {
		for (let i = sheet.cssRules.length - 1; i >= 0; i--) {
			if (sheet.cssRules[i] instanceof CSSPageRule) {
				sheet.deleteRule(i);
				removed++;
			}
		}
	}
	const style = document.createElement('style');
	style.textContent = '.print-header, .print-footer { display: none !important }';
	document.head.append(style);
	return removed;
})()`

// run prints the page in each variant.
func run(ctx context.Context, urlstr, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating the directory %s: %w", dir, err)
	}

	for _, v := range variants {
		// Load the page again for each variant, so that a change of one
		// variant does not reach the next. Wait until the page has loaded,
		// because PrintToPDF prints what the browser has now. Navigate
		// returns after the load event, and the images of the page are
		// part of it.
		if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
			return fmt.Errorf("loading %s: %w", urlstr, err)
		}
		if v.Neutral {
			if _, err := chromedp.Run(ctx, chromedp.Evaluate[int](neutralize)); err != nil {
				return fmt.Errorf("removing the print rules of %s: %w", urlstr, err)
			}
		}
		buf, err := chromedp.Run(ctx, chromedp.PrintToPDF(v.Options...))
		if err != nil {
			return fmt.Errorf("printing %s: %w", v.Name, err)
		}
		name := filepath.Join(dir, "report-"+v.Name+".pdf")
		if err := os.WriteFile(name, buf, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		pages, width, height, err := scan(buf)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		fmt.Printf("wrote %-34s %2d pages of %3.0f x %3.0f pt (%.2f x %.2f in) %8d bytes\n",
			name, pages, width, height, width/72, height/72, len(buf))
	}
	return nil
}

var (
	pageObject = regexp.MustCompile(`/Type\s*/Page[^s]`)
	mediaBox   = regexp.MustCompile(`/MediaBox\s*\[\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\]`)
)

// scan reads the number of pages and the size of the first page in points, 1/72
// of an inch, from the bytes of a PDF file. This is a shortcut and not a PDF
// parser. It counts the objects with /Type /Page, which are the pages, and not
// /Type /Pages, which is the tree of the pages. It works for the files of
// Chrome, which keeps these parts of the file as plain text. A real program
// that reads PDF files uses a PDF library.
func scan(pdf []byte) (pages int, width, height float64, err error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return 0, 0, 0, fmt.Errorf("the data is not a PDF file")
	}
	pages = len(pageObject.FindAll(pdf, -1))
	m := mediaBox.FindSubmatch(pdf)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("no /MediaBox in %d bytes", len(pdf))
	}
	if width, err = strconv.ParseFloat(string(m[3]), 64); err != nil {
		return 0, 0, 0, fmt.Errorf("reading the width of the page: %w", err)
	}
	if height, err = strconv.ParseFloat(string(m[4]), 64); err != nil {
		return 0, 0, 0, fmt.Errorf("reading the height of the page: %w", err)
	}
	return pages, width, height, nil
}
