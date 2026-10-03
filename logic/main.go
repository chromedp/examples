// Command logic is a chromedp example demonstrating how to combine actions and
// Go code in a function that reads a list from a page. It reads github.com. Use
// -v to print the protocol messages and -visible to show the browser window and
// leave it open.
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
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// list the awesome go projects of the section "Selenium and browser
	// control tools."
	res, err := listAwesomeGoProjects(ctx, "Selenium and browser control tools")
	if err != nil {
		log.Fatalf("could not list awesome go projects: %v", err)
	}

	// print the values
	for k, v := range res {
		log.Printf("project %s (%s): '%s'", k, v.URL, v.Description)
	}
}

// projectDesc holds the URL and the description of a project.
type projectDesc struct {
	URL, Description string
}

// listAwesomeGoProjects opens the awesome-go page, finds the section sect, and
// returns the projects of that section.
func listAwesomeGoProjects(ctx context.Context, sect string) (map[string]projectDesc, error) {
	// limit the retrieval and the processing of the data to 15 seconds
	var cancel func()
	ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	sel := fmt.Sprintf(`//h3[text()[contains(., '%s')]]`, sect)

	// navigate
	if err := chromedp.Do(ctx, chromedp.Navigate(`https://github.com/avelino/awesome-go`)); err != nil {
		return nil, fmt.Errorf("could not navigate to github: %w", err)
	}

	// wait until the section is visible
	if err := chromedp.Do(ctx, chromedp.WaitVisible(sel)); err != nil {
		return nil, fmt.Errorf("could not get section: %w", err)
	}

	sib := sel + `/parent::div/following-sibling::ul[1]/li`

	// get project link text
	projects, err := chromedp.Run(ctx, chromedp.Nodes(sib+`/child::a/text()`))
	if err != nil {
		return nil, fmt.Errorf("could not get projects: %w", err)
	}

	// get links and description text
	linksAndDescriptions, err := chromedp.Run(ctx, chromedp.Nodes(sib+`/child::node()`))
	if err != nil {
		return nil, fmt.Errorf("could not get links and descriptions: %w", err)
	}

	// the links and the descriptions must be twice the number of the projects
	if 2*len(projects) != len(linksAndDescriptions) {
		return nil, fmt.Errorf("projects and links and descriptions lengths do not match (2*%d != %d)", len(projects), len(linksAndDescriptions))
	}

	// process data
	res := make(map[string]projectDesc)
	for i := 0; i < len(projects); i++ {
		res[projects[i].NodeValue] = projectDesc{
			URL:         linksAndDescriptions[2*i].AttributeValue("href"),
			Description: strings.TrimPrefix(strings.TrimSpace(linksAndDescriptions[2*i+1].NodeValue), "- "),
		}
	}

	return res, nil
}
