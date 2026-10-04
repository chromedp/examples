// Command click is a chromedp example demonstrating how to use a selector to
// click on an element. It reads a page of the local test site and needs no
// internet. It opens three code examples of the reference page of the package
// time and prints the first line of the function main of each one. Use -url to
// give the full URL of a page to read, for example a live site, and the program
// does not start the local site then. The selectors are written for the local
// site and a live site can differ. Use -v to print the protocol messages and
// -visible to show the browser window and leave it open. Use
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
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

	// navigate to the page and wait until the footer is visible. The page is
	// then loaded
	err = chromedp.Do(ctx,
		chromedp.Navigate(*urlstr),
		chromedp.WaitVisible(`body > footer`),
	)
	if err != nil {
		s.Fatal(err)
	}

	// open three examples, one after the other. An example is a details
	// element. A click on its summary opens it and shows its code
	for _, name := range []string{"After", "Sleep", "NewTimer"} {
		code, err := readExample(ctx, name)
		if err != nil {
			s.Fatal(err)
		}
		log.Printf("example %s has %d lines, the first line of main is %q", name, strings.Count(code, "\n")+1, firstLine(code))
		if name == "After" {
			// print the whole code of the first example
			log.Printf("the code of the example After:\n%s", code)
		}
	}
}

// readExample clicks the summary of the example with the given name, waits
// until the code is visible, and returns the code.
func readExample(ctx context.Context, name string) (string, error) {
	details := `#example-` + name
	err := chromedp.Do(ctx,
		// the example is closed, so the code is not visible yet
		chromedp.Click(details+` summary`, chromedp.NodeVisible),
		// the click opens the example. Wait until the code is visible
		chromedp.WaitVisible(details+` textarea`),
	)
	if err != nil {
		return "", fmt.Errorf("opening example %s: %w", name, err)
	}
	// the code is the value of the textarea
	code, err := chromedp.Run(ctx, chromedp.Value(details+` textarea`))
	if err != nil {
		return "", fmt.Errorf("reading example %s: %w", name, err)
	}
	return code, nil
}

// firstLine returns the first line of the function main of the code.
func firstLine(code string) string {
	_, body, _ := strings.Cut(code, "func main() {\n")
	line, _, _ := strings.Cut(body, "\n")
	return strings.TrimSpace(line)
}
