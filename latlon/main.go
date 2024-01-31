// Command latlon is a chromedp example demonstrating how to retrieve the
// latitude/longitude from google maps, using the browser's target events.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	timeout := flag.Duration("timeout", 30*time.Second, "timeout")
	flag.Parse()
	if err := run(context.Background(), *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, timeout time.Duration) error {
	// create regexp to extract latitude, longitude from the url
	latlonRE := regexp.MustCompile(`maps/@(-?\d+\.\d+,-?\d+\.\d+),`)

	// create chrome instance
	ctx, cancel := chromedp.NewContext(ctx /*, chromedp.WithDebugf(log.Printf)*/)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// listen for the navigated event
	ch, errch := make(chan string, 1), make(chan error, 1)
	defer close(ch)
	defer close(errch)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch event := ev.(type) {
		case *page.EventNavigatedWithinDocument:
			if m := latlonRE.FindStringSubmatch(event.URL); m != nil {
				ch <- m[1]
			}
		}
	})
	go func() {
		if err := chromedp.Run(ctx, chromedp.Navigate("https://www.google.com/maps/?hl=en")); err != nil {
			errch <- err
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	case urlstr := <-ch:
		fmt.Fprintln(os.Stdout, urlstr)
	}
	return nil
}
