// Command head (as opposed to headless) is a chromedp example demonstrating
// how to use a chrome flag to display the browser window.
package main

import (
	"context"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	// create chrome instance
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		// Set the headless flag to false to display the browser window
		chromedp.Flag("headless", false),
	)

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	// navigate to a page, wait for some time and then exit.
	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://xkcd.com/353/`),
		chromedp.Sleep(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
}
