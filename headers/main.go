// Command headers is a chromedp example demonstrating how to add extra HTTP
// headers to browser requests.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func main() {
	port := flag.Int("port", 8544, "port")
	flag.Parse()

	// run server
	go headerServer(fmt.Sprintf(":%d", *port))

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run the steps
	res, err := setheaders(
		ctx,
		fmt.Sprintf("http://localhost:%d", *port),
		network.Headers{
			"X-Header": "my request header",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("received headers: %s", res)
}

// headerServer is a simple HTTP server that displays the passed headers in the html.
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
