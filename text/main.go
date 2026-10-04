// Command text is a chromedp example demonstrating how to extract text from a
// specific element. It reads a page of the local test site and needs no
// internet. It prints the overview of the reference page of the package time,
// the headings of the overview and the difference between Text and
// TextContent. Use -url to give the full URL of a page to read, for example a
// live site, and the program does not start the local site then. The selectors
// are written for the local site and a live site can differ. Use -v to print
// the protocol messages, -visible to show the browser window and leave it open,
// and -visible-on-terminal to draw the page in the terminal with terminal
// graphics.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
)

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

	// start the stream before the first navigation, so that the frames show
	// the page while it loads
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// navigate to the page and wait until the overview is visible
	err = chromedp.Do(ctx,
		chromedp.Navigate(*urlstr),
		chromedp.WaitVisible(`.Documentation-overview`),
	)
	if err != nil {
		s.Fatal(err)
	}

	// Text returns the visible text of the element, as a person reads it. The
	// browser lays out the text, so a paragraph or a heading starts a new line
	// and the style sheet applies
	overview, err := chromedp.Run(ctx, chromedp.Text(`.Documentation-overview`, chromedp.NodeVisible))
	if err != nil {
		s.Fatal(err)
	}
	overview = strings.TrimSpace(overview)
	var lines []string
	for _, line := range strings.Split(overview, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	log.Printf("the overview has %d lines of text and %d characters, the start is:", len(lines), len(overview))
	for _, line := range lines[:min(len(lines), 3)] {
		log.Printf("  %s", line)
	}

	// the first heading and the first paragraph, each with one selector
	title, err := chromedp.Run(ctx, chromedp.Text(`.Documentation-overview > h2`))
	if err != nil {
		s.Fatal(err)
	}
	first, err := chromedp.Run(ctx, chromedp.Text(`.Documentation-overview > p`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the heading is %q and the first paragraph has %d words", title, len(strings.Fields(first)))

	// the headings of the overview. Nodes returns the elements, and Text reads
	// the text of one element. The selector with an index chooses the element
	headings, err := chromedp.Run(ctx, chromedp.Nodes(`.Documentation-overview h3`))
	if err != nil {
		s.Fatal(err)
	}
	for i := range headings {
		name, err := chromedp.Run(ctx, chromedp.Text(fmt.Sprintf(`.Documentation-overview h3:nth-of-type(%d)`, i+1)))
		if err != nil {
			s.Fatal(err)
		}
		log.Printf("section %d of the overview: %s", i+1, name)
	}

	// TextContent returns the text of the nodes in the DOM, with no layout.
	// The text of a program in a pre element keeps its line breaks and its
	// tabs, and the text between the elements keeps the spaces of the source
	// file
	code := `.Documentation-overview pre`
	shown, err := chromedp.Run(ctx, chromedp.Text(code))
	if err != nil {
		s.Fatal(err)
	}
	raw, err := chromedp.Run(ctx, chromedp.TextContent(code))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("the first program of the overview has %d lines", strings.Count(raw, "\n")+1)
	log.Printf("Text and TextContent are the same for the program: %t", shown == raw)

	// the difference is clear for the whole overview
	content, err := chromedp.Run(ctx, chromedp.TextContent(`.Documentation-overview`))
	if err != nil {
		s.Fatal(err)
	}
	log.Printf("Text of the overview has %d characters and TextContent has %d", len(overview), len(content))
}
