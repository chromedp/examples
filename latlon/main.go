// Command latlon is a chromedp example demonstrating how to retrieve the
// latitude and the longitude of a map from the URL of the page with the
// navigation events of the page. It starts a local server and needs no
// internet. The program drives the map, with the zoom button, a drag and the
// keyboard, and it prints each new position when the URL changes. Use -v to
// print the protocol messages and -visible to show the browser window and leave
// it open. The flag -url reads another page instead. The selectors are written
// for the local site, so a live site can differ.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
)

// latlonRE extracts the latitude, the longitude and the zoom from the part of
// the URL after the #. It has the style of the URLs of Google Maps.
var latlonRE = regexp.MustCompile(`@(-?\d+\.\d+),(-?\d+\.\d+),(\d+(?:\.\d+)?)z`)

// position is a place on the map, as the URL says.
type position struct {
	lat, lon, zoom string
}

func (p position) String() string {
	return fmt.Sprintf("%s,%s zoom %s", p.lat, p.lon, p.zoom)
}

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	timeout := flag.Duration("timeout", 30*time.Second, "time limit of the program")
	urlstr := flag.String("url", "", "full URL of the map page to read, for example a live site (default: the map of the local test site, and the selectors are written for it)")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *timeout, *urlstr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose, visible bool, timeout time.Duration, urlstr string) error {
	// start the local test site, unless the flag -url names a page
	if urlstr == "" {
		site := testsite.New()
		defer site.Close()
		urlstr = site.URL + "/map"
	}

	// create context
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()
	if visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// The page changes its URL with history.replaceState after each move, so
	// the browser sends the event Page.navigatedWithinDocument. Subscribe
	// before the navigation, so that no event is lost. A goroutine reads the
	// events, and it sends each position that it finds in the URL.
	positions := make(chan position)
	events := chromedp.Events(ctx, page.NavigatedWithinDocument)
	go func() {
		for ev, err := range events {
			if err != nil {
				// The context ended, so the program is done.
				return
			}
			if verbose {
				log.Printf("%T: %+v\n", ev, ev)
			}
			if m := latlonRE.FindStringSubmatch(ev.URL); m != nil {
				select {
				case positions <- position{m[1], m[2], m[3]}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Each step runs an action on the page, and prints the position that the
	// URL has afterwards.
	steps := []struct {
		name   string
		action chromedp.Action[chromedp.Void]
	}{
		{"loaded the map", chromedp.Navigate(urlstr)},
		{"zoomed in", chromedp.Click(`#zoom-in`)},
		{"zoomed in again", chromedp.Click(`#zoom-in`)},
		{"dragged the map", chromedp.Func(dragMap)},
		{"pressed the arrow right key", chromedp.SendKeys(`#map`, kb.ArrowRight)},
		{"zoomed out", chromedp.Click(`#zoom-out`)},
	}
	for _, step := range steps {
		if err := chromedp.Do(ctx, step.action); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		pos, err := nextPosition(ctx, positions)
		if err != nil {
			return fmt.Errorf("%s: waiting for the new URL: %w", step.name, err)
		}
		fmt.Fprintf(os.Stdout, "%s: %s\n", step.name, pos)
	}
	return nil
}

// nextPosition waits for the next position, and then for a short quiet time. A
// move can change the URL more than once, so it returns the last position.
func nextPosition(ctx context.Context, positions <-chan position) (position, error) {
	var pos position
	select {
	case pos = <-positions:
	case <-ctx.Done():
		return pos, ctx.Err()
	}
	for {
		select {
		case pos = <-positions:
		case <-time.After(300 * time.Millisecond):
			return pos, nil
		case <-ctx.Done():
			return pos, ctx.Err()
		}
	}
}

// dragMap presses the left mouse button on the middle of the map, moves the
// mouse 150 pixels to the left and 60 pixels down, and releases the button.
func dragMap(ctx context.Context, t *chromedp.Target) error {
	// Mouse events work on the viewport, so scroll the map into view first, and
	// then get its middle.
	if err := chromedp.Do(ctx, chromedp.ScrollIntoView(`#map`)); err != nil {
		return fmt.Errorf("scrolling to the map: %w", err)
	}
	center, err := chromedp.Run(ctx, chromedp.Evaluate[struct{ X, Y float64 }](`(function() {
  const r = document.querySelector('#map').getBoundingClientRect();
  return {X: r.x + r.width / 2, Y: r.y + r.height / 2};
})()`))
	if err != nil {
		return fmt.Errorf("finding the middle of the map: %w", err)
	}
	// press, move in small steps as a hand does, and release. During the
	// moves the left button is down, and the event says so with Buttons. The
	// map ignores a move without it.
	held := func(p *input.DispatchMouseEventParams) { p.Buttons = 1 }
	const steps = 10
	const dx, dy = -150, 60
	actions := []chromedp.Action[chromedp.Void]{
		chromedp.MouseEvent(input.DispatchMouseEventTypeMousePressed, center.X, center.Y, chromedp.ButtonLeft, held, chromedp.ClickCount(1)),
	}
	for i := 1; i <= steps; i++ {
		x := center.X + dx*float64(i)/steps
		y := center.Y + dy*float64(i)/steps
		actions = append(actions, chromedp.MouseEvent(input.DispatchMouseEventTypeMouseMoved, x, y, chromedp.ButtonLeft, held))
	}
	actions = append(actions, chromedp.MouseEvent(input.DispatchMouseEventTypeMouseReleased, center.X+dx, center.Y+dy, chromedp.ButtonLeft, chromedp.ClickCount(1)))
	return chromedp.Do(ctx, actions...)
}
