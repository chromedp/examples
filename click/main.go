// Command click is a chromedp example demonstrating how to use a selector to
// click on an element. It reads pkg.go.dev. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

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

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// navigate to a page, wait for an element, click
	err := chromedp.Do(ctx,
		chromedp.Navigate(`https://pkg.go.dev/time`),
		// wait until the footer is visible. The page is then loaded
		chromedp.WaitVisible(`body > footer`),
		// click the summary of the example. This opens it
		chromedp.Click(`#example-After summary`, chromedp.NodeVisible),
	)
	if err != nil {
		log.Fatal(err)
	}
	// retrieve the text of the textarea
	example, err := chromedp.Run(ctx, chromedp.Value(`#example-After textarea`))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Go's time.After example:\n%s", example)
}
