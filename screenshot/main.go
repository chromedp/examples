// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"log"
	"os"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		// chromedp.WithDebugf(log.Printf),
	)
	defer cancel()

	// capture screenshot of an element
	buf, err := elementScreenshot(ctx, `https://pkg.go.dev/`, `img.Homepage-logo`)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("elementScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// capture entire browser viewport, returning png with quality=90
	buf, err = fullScreenshot(ctx, `https://brank.as/`, 90)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("fullScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf("wrote elementScreenshot.png and fullScreenshot.png")
}

// elementScreenshot takes a screenshot of a specific element.
func elementScreenshot(ctx context.Context, urlstr, sel string) ([]byte, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return nil, err
	}
	return chromedp.Run(ctx, chromedp.Screenshot(sel, chromedp.NodeVisible))
}

// fullScreenshot takes a screenshot of the entire browser viewport.
//
// Note: chromedp.FullScreenshot overrides the device's emulation settings. Use
// device.Reset to reset the emulation and viewport settings.
func fullScreenshot(ctx context.Context, urlstr string, quality int) ([]byte, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return nil, err
	}
	return chromedp.Run(ctx, chromedp.FullScreenshot(quality))
}
