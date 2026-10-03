// Command latlon is a chromedp example demonstrating how to retrieve the
// latitude and the longitude from Google Maps with the navigation events of
// the page. It reads www.google.com/maps.
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
	// the regular expression that extracts the latitude and the longitude
	latlonRE := regexp.MustCompile(`maps/@(-?\d+\.\d+,-?\d+\.\d+),`)

	// create context
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// navigate, and wait for the navigated event with the coordinates. The
	// event wait starts before the navigation, so no event is lost.
	ev, err := chromedp.Run(ctx, chromedp.WaitEvent(page.NavigatedWithinDocument,
		func(ev page.EventNavigatedWithinDocument) bool {
			if verbose {
				log.Printf("%T: %+v\n", ev, ev)
			}
			return latlonRE.MatchString(ev.URL)
		},
		chromedp.Navigate("https://www.google.com/maps/?hl=en"),
	))
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, latlonRE.FindStringSubmatch(ev.URL)[1])
	return nil
}
