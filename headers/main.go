// Command headers is a chromedp example demonstrating how to add extra HTTP
// headers to browser requests. It starts a local server and needs no internet.
// Use -v to print the protocol messages, -visible to show the browser window
// and leave it open, and -visible-on-terminal to draw the page in the terminal
// with terminal graphics.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

func main() {
	var tc termcast.Flags
	port := flag.Int("port", 8544, "port of the local web server")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	tc.Register(flag.CommandLine)
	flag.Parse()

	// start the server
	go headerServer(fmt.Sprintf(":%d", *port))

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

	// run the actions
	res, err := setheaders(
		ctx,
		fmt.Sprintf("http://localhost:%d", *port),
		network.Headers{
			"X-Header": "my request header",
		},
	)
	if err != nil {
		s.Fatal(err)
	}

	log.Printf("received headers: %s", res)
}

// headerServer serves a page that shows the headers of the request.
func headerServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		buf, err := json.MarshalIndent(req.Header, "", "  ")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(res, indexHTML, string(buf))
	})
	return http.ListenAndServe(addr, mux)
}

// setheaders sets the passed headers, navigates to the host, and returns the
// text that the page shows.
func setheaders(ctx context.Context, host string, headers network.Headers) (string, error) {
	if err := chromedp.Do(ctx,
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			if _, err := cdp.Call(ctx, t, network.Enable, network.EnableParams{}); err != nil {
				return err
			}
			_, err := cdp.Call(ctx, t, network.SetExtraHTTPHeaders, network.SetExtraHTTPHeadersParams{Headers: headers})
			return err
		}),
		chromedp.Navigate(host),
	); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(chromedp.ID("result"), chromedp.NodeVisible))
}

const indexHTML = `<!doctype html>
<html>
<body>
  <div id="result">%s</div>
</body>
</html>`
