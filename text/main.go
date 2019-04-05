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

	var res string
	// run task list
	if err := chromedp.Run(ctx, text(&res)); err != nil {
		panic(err)
	}

	// wait for the resources to be cleaned up
	cancel()
	chromedp.FromContext(ctx).Allocator.Wait()

	log.Printf("overview: %s", res)
}

func text(res *string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(`https://golang.org/pkg/time/`),
		chromedp.Text(`#pkg-overview`, res, chromedp.NodeVisible, chromedp.ByID),
	}
}
