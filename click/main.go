package main

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run task list
	if err := chromedp.Run(ctx, click()); err != nil {
		panic(err)
	}

	// wait for resources to be cleaned up
	cancel()
	chromedp.FromContext(ctx).Allocator.Wait()
}

func click() chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(`https://golang.org/pkg/time/`),
		chromedp.WaitVisible(`#footer`),
		chromedp.Click(`#pkg-overview`, chromedp.NodeVisible),
		chromedp.Sleep(150 * time.Second),
	}
}
