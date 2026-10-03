// Command remote is a chromedp example demonstrating how to connect to an
// existing Chrome DevTools instance using a remote WebSocket URL. The flag -url
// names the browser and the flag -nav names the page to read, which is on the
// internet by default. The program needs a terminal that can show images. See
// README.md. Use -v to print the protocol messages. The program has no -visible
// flag, because it uses a browser that is already running.
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
	"github.com/chromedp/chromedp/remote"
	"github.com/kenshaw/rasterm"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	urlstr := flag.String("url", "ws://127.0.0.1:9222", "WebSocket URL of the running browser")
	nav := flag.String("nav", "https://www.duckduckgo.com/", "URL of the page to read")
	d := flag.Duration("d", 1*time.Second, "time to wait after the page loads")
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
	// create an allocator context, for the browser context below
	allocatorContext, cancel := remote.NewAllocator(ctx, urlstr)
	defer cancel()

	// build the context options
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}

	// create context
	ctx, cancel = chromedp.NewContext(allocatorContext, opts...)
	defer cancel()

	// run the actions
	if err := chromedp.Do(ctx,
		chromedp.Navigate(nav),
		chromedp.Sleep(d),
	); err != nil {
		return fmt.Errorf("navigating to %s: %w", nav, err)
	}
	body, err := chromedp.Run(ctx, chromedp.OuterHTML("html"))
	if err != nil {
		return fmt.Errorf("reading the html of %s: %w", nav, err)
	}
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return fmt.Errorf("taking a screenshot of %s: %w", nav, err)
	}
	fmt.Printf("Body of %s starts with:\n", nav)
	fmt.Println(body[:min(len(body), 100)])
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	return rasterm.Encode(os.Stdout, img)
}
