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

	// run task list
	var res []string
	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://www.google.com/`),
		chromedp.WaitVisible(`#main`, chromedp.ByID),
		chromedp.Evaluate(`Object.keys(window);`, &res),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("window object keys: %v", res)
}
