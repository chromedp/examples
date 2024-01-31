// Command latlon is a chromedp example demonstrating how to retrieve the
// latitude/longitude from google maps, using the browser's target events.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	timeout := flag.Duration("timeout", 1*time.Minute, "timeout")
	flag.Parse()
	if err := run(context.Background(), *verbose, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, timeout time.Duration) error {
	// regexp to extract latitude, longitude
	latlonRE := regexp.MustCompile(`maps/@(-?\d+\.\d+,-?\d+\.\d+),`)

	// create chrome instance
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// listen for the navigated event
	ch, errch := make(chan string, 1), make(chan error, 1)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if verbose {
			log.Printf("%T: %+v\n", ev, ev)
		}
		switch event := ev.(type) {
		case *page.EventNavigatedWithinDocument:
			if m := latlonRE.FindStringSubmatch(event.URL); m != nil {
				ch <- m[1]
			}
		}
	})

	// run task
	go func() {
		if err := chromedp.Run(ctx, chromedp.Navigate("https://www.google.com/maps/?hl=en")); err != nil {
			errch <- err
		}
	}()

	// wait for context closed, an error, or the result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	case res := <-ch:
		fmt.Fprintln(os.Stdout, res)
	}
	return nil
}
