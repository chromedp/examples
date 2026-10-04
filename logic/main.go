// Command logic is a chromedp example demonstrating how to combine actions and
// Go code in a function that reads a list from a page. It reads a page of the
// local test site and needs no internet. It reads the list of 41 projects of
// the page /repo/, prints the sections with the number of projects and prints
// the first projects of one section. Use -url to give the full URL of a page to
// read, for example a live site, and the program does not start the local site
// then. The selectors are written for the local site and a live site can
// differ. Use -v to print the protocol messages, -visible to show the browser
// window and leave it open, and -visible-on-terminal to draw the page in the
// terminal with terminal graphics.
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
	var tc termcast.Flags
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	urlstr := flag.String("url", "", "full URL of the page to read, for example a live site (the selectors are written for the local site and a live site can differ); when empty, the program starts the local test site and reads /repo/")
	tc.Register(flag.CommandLine)
	flag.Parse()

	// start the local test site, unless the user gives a page to read
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/repo/"
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

	// draw the page in the terminal if the flag -visible-on-terminal is set
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())

	// limit the retrieval and the processing of the data to 20 seconds
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// navigate to the page and wait until the first section is visible
	err = chromedp.Do(ctx,
		chromedp.Navigate(*urlstr),
		chromedp.WaitVisible(`div > h3`),
	)
	if err != nil {
		s.Fatal(err)
	}

	// the names of the sections. Each one is the text of an h3 element
	headings, err := chromedp.Run(ctx, chromedp.Nodes(`//div/h3/text()`))
	if err != nil {
		s.Fatal(err)
	}

	// a Go loop that runs actions: read the projects of each section
	total := 0
	var first []project
	for i, h := range headings {
		sect := strings.TrimSpace(h.NodeValue)
		projects, err := listProjects(ctx, sect)
		if err != nil {
			s.Fatal(fmt.Sprintf("could not list the projects of %q: %v", sect, err))
		}
		log.Printf("section %d, %s: %d projects", i+1, sect, len(projects))
		total += len(projects)
		if i == 0 {
			first = projects
		}
	}
	log.Printf("the page has %d sections and %d projects", len(headings), total)

	// print the first projects of the first section
	for _, p := range first[:min(len(first), 3)] {
		log.Printf("project %s (%s, %s stars): %s", p.Name, p.URL, p.Stars, p.Description)
	}
}

// project holds the name, the URL, the number of stars and the description of
// a project.
type project struct {
	Name, URL, Stars, Description string
}

// listProjects finds the section sect of the page that the browser shows, and
// returns the projects of that section in the order of the page.
func listProjects(ctx context.Context, sect string) ([]project, error) {
	// the xpath expression selects the heading, goes up to its div and goes
	// to the list that follows the div
	sel := fmt.Sprintf(`//h3[text()[contains(., '%s')]]`, sect)
	sib := sel + `/parent::div/following-sibling::ul[1]/li`

	// wait until the section is visible
	if err := chromedp.Do(ctx, chromedp.WaitVisible(sel)); err != nil {
		return nil, fmt.Errorf("getting section: %w", err)
	}

	// get the items of the list. The data-stars attribute is in the item
	items, err := chromedp.Run(ctx, chromedp.Nodes(sib))
	if err != nil {
		return nil, fmt.Errorf("getting items: %w", err)
	}

	// get the project names, which are the text of the links
	names, err := chromedp.Run(ctx, chromedp.Nodes(sib+`/child::a/text()`))
	if err != nil {
		return nil, fmt.Errorf("getting projects: %w", err)
	}

	// get the links and the description texts. An item has two child nodes:
	// the link and the text after it
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(sib+`/child::node()`))
	if err != nil {
		return nil, fmt.Errorf("getting links and descriptions: %w", err)
	}

	// the nodes must be twice the number of the projects
	if len(items) != len(names) || 2*len(names) != len(nodes) {
		return nil, fmt.Errorf("items, names and nodes do not match (%d, %d, 2*%d != %d)", len(items), len(names), len(names), len(nodes))
	}

	// process the data. The description starts with " - "
	res := make([]project, len(names))
	for i := range names {
		stars, _ := items[i].Attribute("data-stars")
		res[i] = project{
			Name:        names[i].NodeValue,
			URL:         nodes[2*i].AttributeValue("href"),
			Stars:       stars,
			Description: strings.TrimPrefix(strings.TrimSpace(nodes[2*i+1].NodeValue), "- "),
		}
	}
	return res, nil
}
