// Command headers is a chromedp example demonstrating how to set a HTTP header
// on requests.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var flagPort = flag.Int("port", 8544, "port")

func main() {
	flag.Parse()

	// run server
	go headerServer(fmt.Sprintf(":%d", *flagPort))

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run task list
	var res string
	err := chromedp.Run(ctx, setheaders(
		fmt.Sprintf("http://localhost:%d", *flagPort),
		map[string]interface{}{
			"X-Header": "my request header",
		},
		&res,
	))
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

// setheaders returns a task list that sets the passed headers.
func setheaders(host string, headers map[string]interface{}, res *string) chromedp.Tasks {
	return chromedp.Tasks{
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
		chromedp.Navigate(host),
		chromedp.Text(`#result`, res, chromedp.ByID, chromedp.NodeVisible),
	}
}

const indexHTML = `<!doctype html>
<html>
<body>
  <div id="result">%s</div>
</body>
</html>`
