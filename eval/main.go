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

	var res []string
	// run task list
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(`https://www.google.com/`),
		chromedp.WaitVisible(`#main`, chromedp.ByID),
		chromedp.Evaluate(`Object.keys(window);`, &res),
	}); err != nil {
		panic(err)
	}

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(ctx).Allocator.Wait()
	log.Printf("window object keys: %v", res)
}
