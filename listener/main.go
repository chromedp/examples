// Listen for network events to get response and status code
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var (
	flagURL = flag.String("url", "https://google.com", "url to load")
)

func Load(url string) {

	chromeContext, cancelContext := chromedp.NewContext(context.Background())
	defer cancelContext()

	var response string
	var statusCode int64
	var responseHeaders map[string]interface{}

	runError := chromedp.Run(
		chromeContext,
		chromeTask(
			chromeContext, url,
			map[string]interface{}{"User-Agent": "Mozilla/5.0"},
			&response, &statusCode, &responseHeaders))

	if runError != nil {
		panic(runError)
	}

	fmt.Printf(
		"\n\n{%s}\n\n > %s\n status: %d\n",
		response, url, statusCode)
	for header, value := range responseHeaders {
		fmt.Printf("%s = %s\n", header, value)
	}
}

func chromeTask(chromeContext context.Context, url string, requestHeaders map[string]interface{}, response *string, statusCode *int64, responseHeaders *map[string]interface{}) chromedp.Tasks {
	chromedp.ListenTarget(chromeContext, func(event interface{}) {
		switch responseReceivedEvent := event.(type) {
		case *network.EventResponseReceived:
			response := responseReceivedEvent.Response
			if response.URL == url {
				*statusCode = response.Status
				*responseHeaders = response.Headers
			}
		}
	})

	return chromedp.Tasks{
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(requestHeaders)),
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			*response, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)

			return err
		})}
}

func main() {
	// get which URL we should load
	flag.Parse()

	// load webpage and report back repsonse and status code
	Load(*flagURL)
}
