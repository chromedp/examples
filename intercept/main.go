// Command intercept is a chromedp example demonstrating how to block, mock and
// change the requests of a page with the Fetch domain. It loads a local page
// that asks for an image, a script of an analytics service and two API
// endpoints. The program pauses these requests, and it answers each one in a
// different way. It fails the image and the analytics script with the reason
// BlockedByClient. It answers the first API request with a JSON body of its
// own, so the request never reaches the server. It adds a header to the second
// API request and lets the request continue. At the end, it prints the text
// that the page built from these answers and the number of requests that it
// blocked, mocked and continued. Every paused request must be continued, failed
// or fulfilled. A request that gets no answer waits forever, and the page
// stops. It starts a local server and needs no internet. Use -v to print the
// protocol messages, -visible to show the browser window and leave it open, and
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

func main() {
	var tc termcast.Flags
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
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

	// draw the page in the terminal if the flag -visible-on-terminal is set
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	stdout := io.Writer(os.Stdout)
	if s != nil {
		stdout = s.LogWriter()
	}

	if err := run(ctx, srv.URL, stdout); err != nil {
		s.Fatal(err)
	}
}

// counts holds what the handler did to the paused requests. Only the handler
// writes it, and the program reads it after the handler ends.
type counts struct {
	blocked, mocked, continued []string
}

// run loads the page with the interception on, and prints the result.
func run(ctx context.Context, host string, stdout io.Writer) error {
	// An empty Do starts the browser. The handler below uses a context that
	// the program cancels at the end, and the first call on a context binds
	// the browser to it. So the first call must use the main context.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// Subscribe before Fetch.enable, so that no paused request is lost.
	handlerCtx, stop := context.WithCancel(ctx)
	defer stop()
	paused := chromedp.Events(handlerCtx, fetch.RequestPaused)

	// Only the requests that match a pattern are paused. The page itself
	// matches no pattern, so it loads as usual. The wildcard * means zero or
	// more characters.
	if _, err := chromedp.Call(ctx, fetch.Enable, fetch.EnableParams{
		Patterns: []*fetch.RequestPattern{
			{URLPattern: "*.png", ResourceType: network.ResourceTypeImage},
			{URLPattern: "*/analytics.js"},
			{URLPattern: "*/api/*"},
		},
	}); err != nil {
		return fmt.Errorf("enabling the Fetch domain: %w", err)
	}

	// The handler must run in its own goroutine. The navigation below waits
	// for the page, and the page waits for the answers of the handler.
	var c counts
	done := make(chan error, 1)
	go func() { done <- handle(ctx, paused, &c) }()

	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/")); err != nil {
		return fmt.Errorf("loading the page: %w", err)
	}

	// The script of the page sets the title when it has all its answers.
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](`document.title === "done"`)); err != nil {
		return fmt.Errorf("waiting for the page: %w", err)
	}
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("#out")))
	if err != nil {
		return fmt.Errorf("reading the page text: %w", err)
	}

	// Stop the handler and wait for it, so that the counts are complete.
	stop()
	if err := <-done; err != nil {
		return err
	}
	sort.Strings(c.blocked)
	fmt.Fprintf(stdout, "page text:\n%s\n\n", indent(text))
	fmt.Fprintf(stdout, "blocked %d: %s\n", len(c.blocked), strings.Join(c.blocked, ", "))
	fmt.Fprintf(stdout, "mocked %d: %s\n", len(c.mocked), strings.Join(c.mocked, ", "))
	fmt.Fprintf(stdout, "continued %d: %s\n", len(c.continued), strings.Join(c.continued, ", "))
	return nil
}

// handle answers each paused request until the context of the events ends.
func handle(ctx context.Context, paused iter.Seq2[fetch.EventRequestPaused, error], c *counts) error {
	for ev, err := range paused {
		if err != nil {
			// The program stops the handler by canceling the context.
			return nil
		}
		u, err := url.Parse(ev.Request.URL)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", ev.Request.URL, err)
		}
		path := u.Path
		switch {
		case ev.ResourceType == network.ResourceTypeImage || path == "/analytics.js":
			// The page sees the same error as for a blocked request of an
			// extension, so it loads the rest as usual.
			_, err = chromedp.Call(ctx, fetch.FailRequest, fetch.FailRequestParams{
				RequestID:   ev.RequestID,
				ErrorReason: network.ErrorReasonBlockedByClient,
			})
			c.blocked = append(c.blocked, path)

		case path == "/api/user":
			// The body is a byte slice. cdproto encodes it as base64 for
			// the protocol. The server never sees this request.
			_, err = chromedp.Call(ctx, fetch.FulfillRequest, fetch.FulfillRequestParams{
				RequestID:    ev.RequestID,
				ResponseCode: 200,
				ResponseHeaders: []*fetch.HeaderEntry{
					{Name: "Content-Type", Value: "application/json"},
					{Name: "X-Mocked", Value: "yes"},
				},
				Body: []byte(`{"name": "Ada Lovelace", "source": "the program"}`),
			})
			c.mocked = append(c.mocked, path)

		default:
			// A new list of headers replaces the old one, so copy the
			// headers of the request first.
			headers := make([]*fetch.HeaderEntry, 0, len(ev.Request.Headers)+1)
			for name, value := range ev.Request.Headers {
				headers = append(headers, &fetch.HeaderEntry{Name: name, Value: fmt.Sprint(value)})
			}
			sort.Slice(headers, func(i, j int) bool { return headers[i].Name < headers[j].Name })
			headers = append(headers, &fetch.HeaderEntry{Name: "X-Token", Value: "added by the program"})
			_, err = chromedp.Call(ctx, fetch.ContinueRequest, fetch.ContinueRequestParams{
				RequestID: ev.RequestID,
				Headers:   headers,
			})
			c.continued = append(c.continued, path)
		}
		if err != nil {
			return fmt.Errorf("answering the request for %s: %w", path, err)
		}
	}
	return nil
}

// indent puts two spaces before each line of the text.
func indent(text string) string {
	return "  " + strings.ReplaceAll(text, "\n", "\n  ")
}

// newMux returns the handlers of the test server. The page asks for an image,
// a script and two API endpoints. The endpoint /api/echo returns the header
// X-Token of the request.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>intercept</title>
<script src="/analytics.js"></script></head><body>
<img id="logo" src="/logo.png">
<pre id="out"></pre>
<script>
(async () => {
  const img = document.getElementById("logo");
  await new Promise(done => {
    if (img.complete) { done(); return; }
    img.onload = img.onerror = done;
  });
  const user = await (await fetch("/api/user")).json();
  const echo = await (await fetch("/api/echo")).text();
  document.getElementById("out").textContent = [
    "user: " + user.name + " (from " + user.source + ")",
    "header X-Token that the server saw: " + echo,
    "analytics script ran: " + (window.tracked === true),
    "image loaded: " + (img.naturalWidth > 0),
  ].join("\n");
  document.title = "done";
})();
</script></body></html>`)
	})
	mux.HandleFunc("/analytics.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, "window.tracked = true;")
	})
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		// a PNG of 1 pixel
		w.Header().Set("Content-Type", "image/png")
		png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg==")
		w.Write(png)
	})
	mux.HandleFunc("/api/user", func(w http.ResponseWriter, r *http.Request) {
		// The program mocks this endpoint, so the page must not get this answer.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name": "nobody", "source": "the server"}`)
	})
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Header.Get("X-Token"))
	})
	return mux
}
