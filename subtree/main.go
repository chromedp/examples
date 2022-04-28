// Command subtree is a chromedp example demonstrating how to populate and
// travel a subtree of the DOM.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
)

func main() {
	// create a test server to serve the page
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
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run task list
	err := chromedp.Run(ctx, travelSubtree(ts.URL, `title`, chromedp.ByID))
	if err != nil {
		log.Fatal(err)
	}
}

// travelSubtree illustrates how to ask chromedp to populate a subtree of a node.
//
// https://github.com/chromedp/chromedp/issues/632#issuecomment-654213589
// @mvdan explains why node.Children is almost always empty:
// Nodes are only obtained from the browser on an on-demand basis.
// If we always held the entire DOM node tree in memory,
// our CPU and memory usage in Go would be far higher.
// And chromedp.FromNode can be used to retrieve the child nodes.
//
// Users get confused sometimes (why node.Children is empty while node.ChildNodeCount > 0?).
// And some users want to travel a subtree of the DOM more easy.
// So here comes the example.
func travelSubtree(pageUrl, of string, opts ...chromedp.QueryOption) chromedp.Tasks {
	var nodes []*cdp.Node
	return chromedp.Tasks{
		chromedp.Navigate(pageUrl),
		chromedp.Nodes(of, &nodes, opts...),
		// ask chromedp to populate the subtree of a node
		chromedp.ActionFunc(func(c context.Context) error {
			// depth -1 for the entire subtree
			// do your best to limit the size of the subtree
			return dom.RequestChildNodes(nodes[0].NodeID).WithDepth(-1).Do(c)
		}),
		// wait a little while for dom.EventSetChildNodes to be fired and handled
		chromedp.Sleep(time.Second),
		chromedp.ActionFunc(func(c context.Context) error {
			printNodes(os.Stdout, nodes, "", "  ")
			return nil
		}),
	}
}

func printNodes(w io.Writer, nodes []*cdp.Node, padding, indent string) {
	// This will block until the chromedp listener closes the channel
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
