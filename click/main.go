// Command click is a chromedp example demonstrating how to use a selector to
// click on an element. It reads pkg.go.dev.
package main

import (
	"context"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		// chromedp.WithDebugf(log.Printf),
	)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// navigate to a page, wait for an element, click
	err := chromedp.Do(ctx,
		chromedp.Navigate(`https://pkg.go.dev/time`),
		// wait until the footer is visible. The page is then loaded
		chromedp.WaitVisible(`body > footer`),
		// find the link of the example, and click it
		chromedp.Click(`#example-After`, chromedp.NodeVisible),
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
