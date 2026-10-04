// Command frames is a chromedp example demonstrating how to reach elements
// inside an iframe and inside a shadow root. It starts two local servers that
// have different host names, so a page of the first server can embed a page of
// the second one as a cross-site iframe. The program reads and clicks an
// element inside an iframe of the same site with the option FromNode. It
// reaches an element inside an open shadow root with a JSPath selector, and it
// waits for that element with WaitNotVisible and WaitVisible. At the end, it
// tries the same query on the cross-site iframe and shows that it finds
// nothing. Then it attaches to the iframe as a target of its own. It starts
// local servers and needs no internet. Use -v to print the protocol messages
// and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// The page comes from 127.0.0.1, and the iframe of the other site comes
	// from the name localhost on another server. Chrome treats the two hosts
	// as different sites, although they are the same computer.
	other := httptest.NewServer(childMux())
	defer other.Close()
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	srv := httptest.NewServer(newMux(otherURL))
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

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		log.Fatal(err)
	}
	if err := sameSite(ctx); err != nil {
		log.Fatal(err)
	}
	if err := shadow(ctx); err != nil {
		log.Fatal(err)
	}
	if err := crossSite(ctx); err != nil {
		log.Fatal(err)
	}
}

// sameSite reads and clicks an element inside an iframe of the same site.
func sameSite(ctx context.Context) error {
	// A query on the page does not look inside an iframe, so first get the
	// node of the iframe element.
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSS("#same")))
	if err != nil {
		return fmt.Errorf("finding the iframe: %w", err)
	}
	frame := nodes[0]

	// FromNode makes a query start at the node. When the node is an iframe,
	// chromedp uses the document inside it. The option works with the CSS
	// and the CSSAll selectors. A Search or a JSPath selector ignores it.
	title, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("#title"), chromedp.FromNode(frame)))
	if err != nil {
		return fmt.Errorf("reading the title in the iframe: %w", err)
	}
	fmt.Printf("same-site iframe, title: %q\n", title)

	// The same selector on the page finds nothing, because the element is in
	// another document. AtLeast(0) lets the query return at once.
	outside, err := chromedp.Run(ctx, chromedp.QueryNodeIDs(chromedp.CSSAll("#title"), chromedp.AtLeast(0)))
	if err != nil {
		return fmt.Errorf("querying the page: %w", err)
	}
	fmt.Printf("the same selector on the page finds %d elements\n", len(outside))

	if err := chromedp.Do(ctx, chromedp.Click(chromedp.CSS("#button"), chromedp.FromNode(frame))); err != nil {
		return fmt.Errorf("clicking in the iframe: %w", err)
	}
	label, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("#button"), chromedp.FromNode(frame)))
	if err != nil {
		return fmt.Errorf("reading the button in the iframe: %w", err)
	}
	fmt.Printf("same-site iframe, button after the click: %q\n", label)
	return nil
}

// shadow reaches an element inside an open shadow root. The root of the page
// hides the elements of a shadow root from DOM.querySelector, so a CSS
// selector cannot find them. A JSPath selector runs a JavaScript expression
// that returns the element, and the expression can go through the shadowRoot
// property of each host.
func shadow(ctx context.Context) error {
	const (
		spinner = `document.getElementById("host").shadowRoot.querySelector(".spinner")`
		button  = `document.getElementById("host").shadowRoot.querySelector("button")`
		message = `document.getElementById("host").shadowRoot.querySelector("p")`
	)

	// The component shows a spinner, and after 300 ms it hides the spinner and
	// shows a button. WaitNotVisible waits for the spinner to go. Do not use
	// WaitNotPresent here. When the expression gives null, the lookup of a
	// JSPath selector fails in the browser, and chromedp retries until the
	// context ends.
	if err := chromedp.Do(ctx, chromedp.WaitNotVisible(chromedp.JSPath(spinner))); err != nil {
		return fmt.Errorf("waiting for the spinner to hide: %w", err)
	}
	if err := chromedp.Do(ctx, chromedp.WaitVisible(chromedp.JSPath(button))); err != nil {
		return fmt.Errorf("waiting for the button: %w", err)
	}
	if err := chromedp.Do(ctx, chromedp.Click(chromedp.JSPath(button))); err != nil {
		return fmt.Errorf("clicking the button in the shadow root: %w", err)
	}
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.JSPath(message)))
	if err != nil {
		return fmt.Errorf("reading the message in the shadow root: %w", err)
	}
	fmt.Printf("shadow root, message after the click: %q\n", text)

	// A CSS selector does not see into the shadow root.
	inside, err := chromedp.Run(ctx, chromedp.QueryNodeIDs(chromedp.CSSAll("#host button"), chromedp.AtLeast(0)))
	if err != nil {
		return fmt.Errorf("querying the page: %w", err)
	}
	fmt.Printf("the selector %q finds %d elements\n", "#host button", len(inside))
	return nil
}

// crossSite tries to read an element inside an iframe of another site. Chrome
// runs such an iframe in a process of its own, and the document of the iframe
// is not part of the node tree of the page. So the query that worked for the
// same-site iframe finds nothing. The iframe is a target of its own. A program
// can attach to that target, and then it queries the iframe as a page.
func crossSite(ctx context.Context) error {
	nodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSS("#cross")))
	if err != nil {
		return fmt.Errorf("finding the cross-site iframe: %w", err)
	}
	frame := nodes[0]
	fmt.Printf("cross-site iframe, document in the node tree: %t\n", frame.ContentDocument != nil)

	// The query waits for the element until its context ends, so give it a
	// time limit.
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := chromedp.Run(queryCtx, chromedp.Text(chromedp.CSS("#title"), chromedp.FromNode(frame))); err != nil {
		fmt.Printf("cross-site iframe, the query with FromNode failed: %v\n", err)
	}

	// An iframe in another process has the type "iframe" in the list of
	// targets. Attach to it with WithTargetID.
	infos, err := chromedp.Targets(ctx)
	if err != nil {
		return fmt.Errorf("listing the targets: %w", err)
	}
	for _, info := range infos {
		if info.Type != "iframe" {
			continue
		}
		frameCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(info.TargetID))
		defer cancel()
		title, err := chromedp.Run(frameCtx, chromedp.Text(chromedp.CSS("#title")))
		if err != nil {
			return fmt.Errorf("reading the title in the iframe target: %w", err)
		}
		fmt.Printf("cross-site iframe, title from its own target: %q\n", title)
	}
	return nil
}

// newMux returns the handlers of the main server. The page holds an iframe of
// the same site, an iframe of the other site, and an open shadow root.
func newMux(otherURL string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><head><title>frames</title></head><body>
<iframe id="same" src="/child"></iframe>
<iframe id="cross" src="%s/child"></iframe>
<div id="host"></div>
<script>
const root = document.getElementById("host").attachShadow({mode: "open"});
root.innerHTML = '<p>waiting</p><div class="spinner">loading</div>';
setTimeout(() => {
  root.querySelector(".spinner").hidden = true;
  const button = document.createElement("button");
  button.textContent = "Start";
  button.onclick = () => { root.querySelector("p").textContent = "started"; };
  root.appendChild(button);
}, 300);
</script></body></html>`, otherURL)
	})
	mux.Handle("/child", childMux())
	return mux
}

// childMux returns the handler of the page that the iframes show.
func childMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		fmt.Fprintf(w, `<html><body>
<h1 id="title">Page of %s</h1>
<button id="button" onclick="this.textContent = 'clicked'">click me</button>
</body></html>`, host)
	})
}
