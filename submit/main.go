// Command submit is a chromedp example demonstrating how to fill out and submit
// a form. It reads a page of the local test site and needs no internet. It
// searches the encyclopedia of the site, prints the results, opens the first
// result and prints its title. Use -url to give the full URL of the page with
// the search form, for example a live site, and the program does not start the
// local site then. The selectors are written for the local site and a live site
// can differ. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	urlstr := flag.String("url", "", "full URL of the page with the search form, for example a live site (the selectors are written for the local site and a live site can differ); when empty, the program starts the local test site and reads /wiki/")
	flag.Parse()

	// start the local test site, unless the user gives a page to read
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/wiki/"
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

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// run the actions
	res, err := submit(ctx, *urlstr, `#searchInput`, `railway`)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("the search for %q found %s", res.Query, res.Summary)
	for i, r := range res.Results {
		log.Printf("result %d: %s: %s", i+1, r.Title, r.Snippet)
	}
	log.Printf("the first result opened the article %q, which starts with: %s", res.Article, res.Start)
}

// searchResult is a result of the search.
type searchResult struct {
	Title, Snippet string
}

// searchPage is what the program reads after it submits the form.
type searchPage struct {
	Query   string
	Summary string
	Results []searchResult
	Article string
	Start   string
}

// submit searches the encyclopedia for q. It types q in the input that sel
// selects, submits the form and reads the results. Then it clicks the first
// result and reads the title and the first paragraph of the article.
func submit(ctx context.Context, urlstr, sel, q string) (*searchPage, error) {
	const results = `ul.mw-search-results`
	err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(sel),
		// SendKeys types the text as a person does
		chromedp.SendKeys(sel, q),
		// Submit submits the form of the input
		chromedp.Submit(sel),
		// the results are on a new page. Wait until the list is visible
		chromedp.WaitVisible(results),
	)
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}

	page := &searchPage{Query: q}

	// the number of the results on this page, and the data of each result
	n, err := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('li.mw-search-result').length`))
	if err != nil {
		return nil, fmt.Errorf("counting results: %w", err)
	}
	summary, err := chromedp.Run(ctx, chromedp.Text(`.search-summary`))
	if err != nil {
		return nil, fmt.Errorf("reading the summary: %w", err)
	}
	page.Summary = fmt.Sprintf("%d results on the first page. The page says: %s", n, summary)
	for i := 1; i <= min(n, 3); i++ {
		item := fmt.Sprintf(`%s > li:nth-child(%d)`, results, i)
		title, err := chromedp.Run(ctx, chromedp.Text(item+` .mw-search-result-heading`))
		if err != nil {
			return nil, fmt.Errorf("reading title %d: %w", i, err)
		}
		snippet, err := chromedp.Run(ctx, chromedp.Text(item+` .searchresult`))
		if err != nil {
			return nil, fmt.Errorf("reading snippet %d: %w", i, err)
		}
		page.Results = append(page.Results, searchResult{Title: title, Snippet: snippet})
	}

	// click the link of the first result and wait for the article
	err = chromedp.Do(ctx,
		chromedp.Click(results+` > li:first-child .mw-search-result-heading a`),
		chromedp.WaitVisible(`#firstHeading`),
	)
	if err != nil {
		return nil, fmt.Errorf("opening the first result: %w", err)
	}
	page.Article, err = chromedp.Run(ctx, chromedp.Text(`#firstHeading`))
	if err != nil {
		return nil, fmt.Errorf("reading the title of the article: %w", err)
	}
	start, err := chromedp.Run(ctx, chromedp.Text(`#mw-content-text p:not(.mw-empty-elt)`))
	if err != nil {
		return nil, fmt.Errorf("reading the article: %w", err)
	}
	// keep the first 120 characters
	page.Start = strings.TrimSpace(start)
	if r := []rune(page.Start); len(r) > 120 {
		page.Start = string(r[:120]) + "..."
	}
	return page, nil
}
