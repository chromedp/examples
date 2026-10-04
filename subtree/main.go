// Command subtree is a chromedp example demonstrating how to populate and travel
// a subtree of the DOM. It starts a local server and needs no internet. Use -v
// to print the protocol messages, -visible to show the browser window and leave
// it open, and -visible-on-terminal to draw the page in the terminal with
// terminal graphics.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// create a test server for the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Title</title>
</head>
<body>
<h1 id="title" class="link">
    <a href="https://test.com/helloworld">
        content of h1 1
    </a>
    <span>hello</span> world
</h1>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

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

	// While the stream runs, it holds what the program writes to out, and the
	// stream prints it when it stops.
	out := io.Writer(os.Stdout)
	if s != nil {
		out = s.LogWriter()
	}

	// run the actions
	err = travelSubtree(ctx, out, ts.URL, chromedp.ID("title"))
	if err != nil {
		s.Fatal(err)
	}
}

// travelSubtree shows how to make chromedp populate the subtree of a node.
//
// Users ask why node.Children is empty while node.ChildNodeCount is greater
// than 0. In the issue below, @mvdan explains that chromedp gets nodes from the
// browser only on demand. A program that holds the whole DOM tree in memory
// uses much more CPU and memory. chromedp.FromNode retrieves the child nodes.
// This example shows an easier way to travel a subtree of the DOM.
//
// https://github.com/chromedp/chromedp/issues/632#issuecomment-654213589
func travelSubtree[S chromedp.Selectable](ctx context.Context, w io.Writer, urlstr string, sel S, opts ...chromedp.QueryOption) error {
	// add the populate option to the passed opts
	opts = append(opts, chromedp.Populate(-1, true, chromedp.PopulateWait(1*time.Second)))

	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return err
	}
	// retrieve the nodes. The code added the [chromedp.Populate] option to
	// opts, so the [chromedp.Nodes] action waits until the
	// [chromedp.PopulateWait] timeout passes
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(sel, opts...))
	if err != nil {
		return err
	}
	printNodes(w, nodes, "", "  ")
	return nil
}

// printNodes prints the nodes as a tree. It calls itself for the children of
// each node.
//
// The chromedp package has the same function as [chromedp.Dump] and
// [chromedp.DumpTo].
func printNodes(w io.Writer, nodes []*chromedp.Node, padding, indent string) {
	for _, node := range nodes {
		switch {
		case node.NodeName == "#text":
			fmt.Fprintf(w, "%s#text: %q\n", padding, node.NodeValue)
		default:
			fmt.Fprintf(w, "%s%s:\n", padding, strings.ToLower(node.NodeName))
			if n := len(node.Attributes); n > 0 {
				fmt.Fprintf(w, "%sattributes:\n", padding+indent)
				for i := 0; i < n; i += 2 {
					fmt.Fprintf(w, "%s%s: %q\n", padding+indent+indent, node.Attributes[i], node.Attributes[i+1])
				}
			}
		}
		if node.ChildNodeCount > 0 {
			fmt.Fprintf(w, "%schildren:\n", padding+indent)
			printNodes(w, node.Children, padding+indent+indent, indent)
		}
	}
}
