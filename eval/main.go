// Command eval is a chromedp example demonstrating how to evaluate javascript
// and retrieve the result.
package main

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run the steps
	if err := chromedp.Do(ctx, chromedp.Navigate(`https://www.google.com/`)); err != nil {
		log.Fatal(err)
	}
	res, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Object.keys(window);`))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("window object keys: %v", res)
}
