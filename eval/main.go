// Command eval is a chromedp example demonstrating how to evaluate JavaScript
// and retrieve the result. It reads www.google.com. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
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

	// run the actions
	if err := chromedp.Do(ctx, chromedp.Navigate(`https://www.google.com/`)); err != nil {
		log.Fatal(err)
	}
	res, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Object.keys(window);`))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("window object keys: %v", res)
}
