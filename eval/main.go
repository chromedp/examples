// Command eval is a chromedp example demonstrating how to evaluate JavaScript
// and retrieve the result. It reads a page of the local test site and needs no
// internet. It evaluates scripts on the reference page of the package time and
// decodes the results into Go values, from a number and a list of strings to a
// struct and a list of structs. Use -url to give the full URL of a page to
// read, for example a live site, and the program does not start the local site
// then. The selectors are written for the local site and a live site can
// differ. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open. Use -visible-on-terminal to draw the page
// in the terminal with terminal graphics.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
)

// link is a link of the page. The names of the JSON fields are the names of the
// keys of the object that the script returns.
type link struct {
	Text string `json:"text"`
	Href string `json:"href"`
}

// style holds some computed style values of an element.
type style struct {
	Font    string `json:"font"`
	Color   string `json:"color"`
	Display string `json:"display"`
	Width   string `json:"width"`
}

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	urlstr := flag.String("url", "", "full URL of the page to read, for example a live site (the selectors are written for the local site and a live site can differ); when empty, the program starts the local test site and reads /docs/time")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// start the local test site, unless the user gives a page to read
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/docs/time"
	}

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

	// draw the page in the terminal when the user asks for it. The stream
	// starts the browser, so it starts before the first navigation
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// navigate to the page and wait until it is loaded
	err = chromedp.Do(ctx,
		chromedp.Navigate(*urlstr),
		chromedp.WaitVisible(`body > footer`),
	)
	if err != nil {
		s.Fatal(err)
	}

	// a script that returns a string. The type argument tells Evaluate how to
	// decode the result
	title, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.title`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the title of the page is %q", title)

	// a script that returns a number
	sections, err := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('section').length`))
	if err != nil {
		s.Fatal(err)
	}
	words, err := chromedp.Run(ctx, chromedp.Evaluate[int](`document.body.innerText.split(/\s+/).length`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the page has %d sections and about %d words", sections, words)

	// a script that returns an array of strings. The array becomes a slice
	keys, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](`Object.keys(window);`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the window object has %d keys, the first ones are %v", len(keys), keys[:min(len(keys), 5)])

	// a script that returns an array of objects. Each object becomes a struct
	links, err := chromedp.Run(ctx, chromedp.Evaluate[[]link](`
		Array.from(document.querySelectorAll('.Documentation-indexList a')).map(a => ({
			text: a.textContent.trim(),
			href: a.href,
		}))`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the index has %d links:", len(links))
	for _, l := range links[:min(len(links), 3)] {
		log.Printf("  %s -> %s", l.Text, l.Href)
	}

	// a script that reads the computed styles, which the style sheets set
	st, err := chromedp.Run(ctx, chromedp.Evaluate[style](`(() => {
		const s = getComputedStyle(document.querySelector('.Documentation-overview p'));
		return {font: s.fontFamily, color: s.color, display: s.display, width: s.width};
	})()`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the style of the first paragraph: %+v", st)

	// a script that returns a promise. The option makes Evaluate wait for the
	// result of the promise
	status, err := chromedp.Run(ctx, chromedp.Evaluate[int](
		`fetch(location.href).then(r => r.status)`,
		chromedp.EvalAwaitPromise,
	))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("a request of the page to itself gives the status %d", status)
}
