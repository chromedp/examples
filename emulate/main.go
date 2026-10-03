// Command emulate is a chromedp example demonstrating how to emulate a specific
// device such as an iPhone. It reads www.whatsmyua.info and writes
// screenshot1.png and screenshot2.png to the current directory. Use -v to print
// the protocol messages and -visible to show the browser window and leave it
// open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// emulate an iPhone, and capture the first screenshot
	if err := chromedp.Do(ctx,
		// emulate an iPhone 7 in landscape
		chromedp.Emulate(device.IPhone7landscape),
		chromedp.Navigate(`https://www.whatsmyua.info/`),
	); err != nil {
		log.Fatal(err)
	}
	b1, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		log.Fatal(err)
	}

	if err := chromedp.Do(ctx,
		// reset the emulation
		chromedp.Emulate(device.Reset),

		// set a large viewport
		chromedp.EmulateViewport(1920, 2000),
		chromedp.Navigate(`https://www.whatsmyua.info/?a`),
	); err != nil {
		log.Fatal(err)
	}
	b2, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("screenshot1.png", b1, 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("screenshot2.png", b2, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote screenshot1.png and screenshot2.png")
}
