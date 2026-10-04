// Command selectors is a chromedp example demonstrating how to choose the
// elements of a page with the typed selectors. The type of the selector
// chooses the lookup: CSS, CSSAll, ID, JSPath, NodeIDs and a plain string,
// which is a search by CSS selector, XPath or text. The program loads one local
// page and finds elements with each type, and it prints what each lookup
// returns. It uses the actions Text, Nodes and Attributes with these
// selectors. It also uses ByFunc for a lookup of its own, and the options
// AtLeast and NodeVisible, which change how long a query waits. It starts a
// local server and needs no internet. Use -v to print the protocol messages,
// -visible to show the browser window and leave it open, and
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
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

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

// out receives the results that the program prints. It is the standard output,
// or the held writer of the stream when the flag -visible-on-terminal is on,
// because the stream clears the terminal and would erase the results.
var out io.Writer = os.Stdout

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

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
	if s != nil {
		out = s.LogWriter()
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		s.Fatal(err)
	}
	if err := types(ctx); err != nil {
		s.Fatal(err)
	}
	if err := options(ctx); err != nil {
		s.Fatal(err)
	}
}

// types finds elements of the page with each selector type.
func types(ctx context.Context) error {
	// CSS selects the first element that matches a CSS selector. It is the
	// right choice for most queries. It does not look into a shadow root or
	// into an iframe.
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("li.fruit")))
	if err != nil {
		return fmt.Errorf("selecting with CSS: %w", err)
	}
	fmt.Fprintf(out, "CSS(%q): %q\n", "li.fruit", text)

	// CSSAll selects every element that matches. An action that returns
	// one value, such as Text, uses the first. Nodes and AttributesAll
	// return one value for each element.
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSSAll("li.fruit")))
	if err != nil {
		return fmt.Errorf("selecting with CSSAll: %w", err)
	}
	fmt.Fprintf(out, "CSSAll(%q): %d nodes\n", "li.fruit", len(nodes))
	attrs, err := chromedp.Run(ctx, chromedp.AttributesAll(chromedp.CSSAll("li.fruit")))
	if err != nil {
		return fmt.Errorf("reading the attributes with CSSAll: %w", err)
	}
	for _, a := range attrs {
		fmt.Fprintf(out, "  data-id %s, class %q\n", a["data-id"], a["class"])
	}

	// ID selects the element with an id. The leading # is optional. Use it
	// when the page gives the element a unique id.
	text, err = chromedp.Run(ctx, chromedp.Text(chromedp.ID("title")))
	if err != nil {
		return fmt.Errorf("selecting with ID: %w", err)
	}
	fmt.Fprintf(out, "ID(%q): %q\n", "title", text)

	// Attributes returns the attributes of the element as a map.
	link, err := chromedp.Run(ctx, chromedp.Attributes(chromedp.ID("cart")))
	if err != nil {
		return fmt.Errorf("reading the attributes: %w", err)
	}
	fmt.Fprintf(out, "Attributes(ID(%q)): href %s\n", "cart", link["href"])

	// JSPath runs a JavaScript expression that returns an element. Use it
	// for what a CSS selector cannot reach, such as an element in an open
	// shadow root, or for a choice that needs code. Pass only trusted text,
	// because the browser runs the expression.
	path := `document.getElementById("host").shadowRoot.querySelector("span")`
	text, err = chromedp.Run(ctx, chromedp.Text(chromedp.JSPath(path)))
	if err != nil {
		return fmt.Errorf("selecting with JSPath: %w", err)
	}
	fmt.Fprintf(out, "JSPath(%q): %q\n", "...shadowRoot.querySelector(\"span\")", text)

	// NodeIDs selects elements that the program found before. QueryNodeIDs
	// returns the ids. Use it to reuse a result of a query, for example to
	// act on one element of a list.
	ids, err := chromedp.Run(ctx, chromedp.QueryNodeIDs(chromedp.CSSAll("li.fruit")))
	if err != nil {
		return fmt.Errorf("reading the node ids: %w", err)
	}
	text, err = chromedp.Run(ctx, chromedp.Text(chromedp.NodeIDs(ids[1:2])))
	if err != nil {
		return fmt.Errorf("selecting with NodeIDs: %w", err)
	}
	fmt.Fprintf(out, "NodeIDs(the second of %d ids): %q\n", len(ids), text)

	// A plain string is a Search. The browser reads it as an XPath query
	// when it starts with a slash, as a CSS selector when it is one, and as
	// plain text otherwise. Use it for XPath, which CSS cannot express, such
	// as a test of the text of an element.
	for _, q := range []string{`li.fruit`, `//li[@data-id="2"]`} {
		text, err := chromedp.Run(ctx, chromedp.Text(q))
		if err != nil {
			return fmt.Errorf("searching for %q: %w", q, err)
		}
		fmt.Fprintf(out, "Search(%q): %q\n", q, text)
	}

	// A search for plain text matches the text node, which is not an element.
	// Actions such as Text and Click need an element, so read the node with
	// Nodes.
	found, err := chromedp.Run(ctx, chromedp.Nodes("Banana"))
	if err != nil {
		return fmt.Errorf("searching for text: %w", err)
	}
	fmt.Fprintf(out, "Search(%q): %d node, %s %q\n", "Banana", len(found), found[0].NodeName, found[0].NodeValue)

	// ByFunc replaces the lookup of the selector with a function of the
	// program. The function gets the node where the query starts, and it
	// returns node ids. The selector is only a label for error messages. Use
	// it for a rule that no selector states, such as the last element.
	last := func(ctx context.Context, t *chromedp.Target, root *chromedp.Node) ([]cdp.NodeID, error) {
		res, err := cdp.Call(ctx, t, dom.QuerySelectorAll, dom.QuerySelectorAllParams{NodeID: root.NodeID, Selector: "li.fruit"})
		if err != nil {
			return nil, err
		}
		if len(res.NodeIDs) == 0 {
			return nil, nil
		}
		return res.NodeIDs[len(res.NodeIDs)-1:], nil
	}
	// The last item is the hidden one. Text reads the rendered text, which is
	// empty for a hidden element, so this query uses TextContent.
	text, err = chromedp.Run(ctx, chromedp.TextContent(chromedp.CSS("last fruit"), chromedp.ByFunc(last)))
	if err != nil {
		return fmt.Errorf("selecting with ByFunc: %w", err)
	}
	fmt.Fprintf(out, "ByFunc(the last li.fruit): %q\n", text)
	return nil
}

// options shows how AtLeast and NodeVisible change how long a query waits.
func options(ctx context.Context) error {
	// The script adds two rows to the table 300 ms after the load. By default
	// a query waits for one element, so it can return before the second row
	// exists. AtLeast(2) waits for two.
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSSAll("#rows li"), chromedp.AtLeast(2)))
	if err != nil {
		return fmt.Errorf("waiting for two rows: %w", err)
	}
	fmt.Fprintf(out, "AtLeast(2): %d rows\n", len(nodes))

	// AtLeast(0) never waits. It is the way to ask whether elements exist.
	ids, err := chromedp.Run(ctx, chromedp.QueryNodeIDs(chromedp.CSSAll(".missing"), chromedp.AtLeast(0)))
	if err != nil {
		return fmt.Errorf("querying a missing element: %w", err)
	}
	fmt.Fprintf(out, "AtLeast(0) on .missing: %d elements\n", len(ids))

	// By default a query waits until the element is in the DOM, and it does
	// not care whether the element is visible. NodeVisible also waits until
	// the element is visible. The hidden item has display none and never
	// shows, so this query ends with the time limit.
	hidden, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSS("li.hidden")))
	if err != nil {
		return fmt.Errorf("selecting the hidden item: %w", err)
	}
	fmt.Fprintf(out, "default condition: found the hidden item, %d node\n", len(hidden))
	short, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = chromedp.Run(short, chromedp.Nodes(chromedp.CSS("li.hidden"), chromedp.NodeVisible))
	fmt.Fprintf(out, "NodeVisible on the hidden item: %v\n", err)

	// The element #later shows 300 ms after the load, and NodeVisible waits
	// for it. A click needs a visible element, and Click applies the same
	// wait.
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.ID("later"), chromedp.NodeVisible))
	if err != nil {
		return fmt.Errorf("waiting for #later: %w", err)
	}
	fmt.Fprintf(out, "NodeVisible on #later: %q\n", strings.TrimSpace(text))
	return nil
}

// newMux returns the handlers of the test server. The page has a list, a link,
// a shadow root and elements that appear after the load.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>selectors</title></head><body>
<h1 id="title">Fruit shop</h1>
<ul>
  <li class="fruit" data-id="1">Apple</li>
  <li class="fruit" data-id="2">Banana</li>
  <li class="fruit hidden" data-id="3" style="display: none">Cherry</li>
</ul>
<a id="cart" href="/cart">Cart</a>
<div id="host"></div>
<ul id="rows"></ul>
<p id="later" style="display: none">I am visible now</p>
<script>
document.getElementById("host").attachShadow({mode: "open"}).innerHTML = "<span>in the shadow</span>";
setTimeout(() => {
  for (const n of [1, 2]) {
    const li = document.createElement("li");
    li.textContent = "row " + n;
    document.getElementById("rows").appendChild(li);
  }
  document.getElementById("later").style.display = "block";
}, 300);
</script></body></html>`)
	})
	return mux
}
