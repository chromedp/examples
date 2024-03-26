// Command remote is a chromedp example demonstrating how to connect to an
// existing Chrome DevTools instance using a remote WebSocket URL.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	urlstr := flag.String("url", "ws://127.0.0.1:9222", "devtools url")
	flag.Parse()
	if err := run(context.Background(), *verbose, *urlstr); err != nil {
		log.Fatal("error: %v", err)
	}
}

func run(ctx context.Context, verbose bool, urlstr string) error {
	if urlstr == "" {
		return errors.New("invalid remote devtools url")
	}
	// create allocator context for use with creating a browser context later
	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), urlstr)
	defer cancel()

	// build context options
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}

	// create context
	ctx, cancel = chromedp.NewContext(allocatorContext, opts...)
	defer cancel()

	// run task list
	var body string
	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://duckduckgo.com"),
		chromedp.Sleep(1*time.Second),
		chromedp.OuterHTML("html", &body),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return fmt.Errorf("Failed getting body of duckduckgo.com: %v", err)
	}
	fmt.Println("Body of duckduckgo.com starts with:")
	fmt.Println(body[0:100])
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	return rasterm.Encode(os.Stdout, img)
}
