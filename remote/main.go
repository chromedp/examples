// Command remote demonstrating how to connect to an existing chromium instance using chromedp.NewRemoteAllocator
package main

import (
	"context"
	"flag"
	"github.com/chromedp/chromedp"
	"log"
)

func main() {
	var devToolWsUrl string
	flag.StringVar(&devToolWsUrl, "devtools-ws-url", "", "DevTools Websocket URL")
	flag.Parse()

	actxt, cancelActxt := chromedp.NewRemoteAllocator(context.Background(), devToolWsUrl)
	defer cancelActxt()

	ctxt, cancelCtxt := chromedp.NewContext(actxt) // create new tab
	defer cancelCtxt()                             // close tab afterwards

	var body string
	if err := chromedp.Run(ctxt,
		chromedp.Navigate("https://duckduckgo.com"),
		chromedp.WaitVisible("#logo_homepage_link"),
		chromedp.OuterHTML("html", &body),
	); err != nil {
		log.Fatalf("Failed getting body of duckduckgo.com: %v", err)
	}

	log.Println("Body of duckduckgo.com starts with:")
	log.Println(body[0:100])
}
