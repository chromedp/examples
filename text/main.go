// Command text is a chromedp example demonstrating how to extract text from a
// specific element. It reads pkg.go.dev.
package main

import (
	"context"
	"log"
	"strings"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run the actions
	if err := chromedp.Do(ctx, chromedp.Navigate(`https://pkg.go.dev/time`)); err != nil {
		log.Fatal(err)
	}
	res, err := chromedp.Run(ctx, chromedp.Text(`.Documentation-overview`, chromedp.NodeVisible))
	if err != nil {
		log.Fatal(err)
	}

	log.Println(strings.TrimSpace(res))
}
