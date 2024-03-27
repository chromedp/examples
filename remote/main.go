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
	nav := flag.String("nav", "https://www.duckduckgo.com/", "nav")
	d := flag.Duration("d", 1*time.Second, "wait duration")
	flag.Parse()
	if err := run(context.Background(), *verbose, *urlstr, *nav, *d); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, urlstr, nav string, d time.Duration) error {
	if urlstr == "" {
		return errors.New("invalid remote devtools url")
	}
	// create allocator context for use with creating a browser context later
	allocatorContext, _ := chromedp.NewRemoteAllocator(context.Background(), urlstr)
	// defer cancel()

	// build context options
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}

	// create context
	ctx, _ = chromedp.NewContext(allocatorContext, opts...)
	// defer cancel()

	// run task list
	var body string
	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(nav),
		chromedp.Sleep(d),
		chromedp.OuterHTML("html", &body),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return fmt.Errorf("Failed getting body of %s: %v", nav, err)
	}
	fmt.Printf("Body of %s starts with:\n", nav)
	fmt.Println(body[0:100])
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	return rasterm.Encode(os.Stdout, img)
}
