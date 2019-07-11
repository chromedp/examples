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

func Load(url string) bool {

	// get a context
	chromeContext, cancelContext := chromedp.NewContext(context.Background())
	defer cancelContext()

	// where we'll store the details of the response
	var response string
	var statusCode int64
	var responseHeaders map[string]interface{}

	// start Chrome and run given tasks
	runError := chromedp.Run(
		chromeContext,
		chromeTask(
			chromeContext, url,
			map[string]interface{}{"User-Agent": "Mozilla/5.0"},
			&response, &statusCode, &responseHeaders))

	if runError != nil {
		fmt.Println(runError)
		return false
	}

	// print details of the response
	// fmt.Printf("\n\n{%s}\n\n > %s\n status: %d\n",
	// 	response, url, statusCode)
	// for header, value := range responseHeaders {
	// 	fmt.Printf("%s = %s\n", header, value)
	// }

	return true
}

// chrome debug protocol tasks to run
func chromeTask(chromeContext context.Context, url string, requestHeaders map[string]interface{}, response *string, statusCode *int64, responseHeaders *map[string]interface{}) chromedp.Tasks {

	// setup a listener for events
	chromedp.ListenTarget(chromeContext, func(event interface{}) {

		// fmt.Printf(" msg: %T\n", event)

		// get which type of event it is
		switch msg := event.(type) {

		// just before request sent
		case *network.EventRequestWillBeSent:
			request := msg.Request
			// fmt.Printf(" request url: %s\n", request.URL)

			// see if we have been redirected
			// if so, change the URL that we are tracking
			if msg.RedirectResponse != nil {
				url = request.URL
				fmt.Printf(" got redirect: %s\n", msg.RedirectResponse.URL)
			}

		// once we have the full response
		case *network.EventResponseReceived:

			response := msg.Response

			// is the request we want the status/headers on?
			if response.URL == url {
				*statusCode = response.Status
				*responseHeaders = response.Headers

				fmt.Printf(" url: %s\n", response.URL)
				fmt.Printf(" status code: %d\n", *statusCode)
				fmt.Printf(" # headers: %d\n", len(*responseHeaders))
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
