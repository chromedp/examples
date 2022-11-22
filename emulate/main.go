// Command emulate is a chromedp example demonstrating how to emulate a
// specific device such as an iPhone.
package main

import (
	"context"
	"log"
	"os"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run
	var b1, b2 []byte
	if err := chromedp.Run(ctx,
		// emulate iPhone 7 landscape
		chromedp.Emulate(device.IPhone7landscape),
		chromedp.Navigate(`https://www.whatsmyua.info/`),
		chromedp.CaptureScreenshot(&b1),

		// reset
		chromedp.Emulate(device.Reset),

		// set really large viewport
		chromedp.EmulateViewport(1920, 2000),
		chromedp.Navigate(`https://www.whatsmyua.info/?a`),
		chromedp.CaptureScreenshot(&b2),
	); err != nil {
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
