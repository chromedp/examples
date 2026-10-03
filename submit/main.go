// Command submit is a chromedp example demonstrating how to fill out and submit
// a form. It searches en.wikipedia.org. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// run the actions. The legacy skin (useskin=vector) keeps the search box
	// visible in a narrow window, where the default skin hides it in an icon
	res, err := submit(ctx, `https://en.wikipedia.org/wiki/Main_Page?useskin=vector`, `#searchInput`, `Go programming language`)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("got: `%s`", strings.TrimSpace(res))
}

// submit searches Wikipedia for q. It types q in the input that sel selects,
// submits the form, and returns the first paragraph of the page that opens.
func submit(ctx context.Context, urlstr, sel, q string) (string, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(sel),
		chromedp.SendKeys(sel, q),
		chromedp.Submit(sel),
		chromedp.WaitVisible(`#firstHeading`),
	); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(`#mw-content-text p:not(.mw-empty-elt)`))
}
