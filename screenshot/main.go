// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport. It reads
// pkg.go.dev and brank.as. Use -v to print the protocol messages and -visible
// to show the browser window and leave it open.
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
	verbose := flag.Bool("v", false, "verbose")
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

	// capture screenshot of an element
	buf, err := elementScreenshot(ctx, `https://pkg.go.dev/`, `img.Homepage-logo`)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("elementScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}

	// capture the entire browser viewport with the quality 90
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
// chromedp.FullScreenshot overrides the emulation settings of the device. To
// restore the emulation and the viewport settings, use device.Reset.
func fullScreenshot(ctx context.Context, urlstr string, quality int) ([]byte, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return nil, err
	}
	return chromedp.Run(ctx, chromedp.FullScreenshot(quality))
}
