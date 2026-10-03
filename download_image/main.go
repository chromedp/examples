// Command download_image is a chromedp example demonstrating how to do
// headless image downloads.
//
// Note that for this technique to work, the file type must load inside the
// browser window without triggering a download. See the download_file example
// for how to save a file that triggers the "Download / Save As" browser
// dialog.
package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// create a timeout as a safety net to prevent any infinite wait loops
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// set the download url as the chromedp GitHub user avatar
	urlstr := "https://avatars.githubusercontent.com/u/33149672"

	// subscribe to the network events, so we can watch them after the
	// navigation. the request id matching is important both to filter out
	// unwanted network events and to reference the downloaded file later
	requests := chromedp.Events(ctx, network.RequestWillBeSent)
	finished := chromedp.Events(ctx, network.LoadingFinished)

	// all we need to do here is navigate to the download url
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		log.Fatal(err)
	}

	// this will be used to capture the request id for matching network events
	var requestID network.RequestID
	for ev, err := range requests {
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("EventRequestWillBeSent: %v: %v", ev.RequestID, ev.Request.URL)
		if ev.Request.URL == urlstr {
			requestID = ev.RequestID
			break
		}
	}

	// This will block until the request with the request id is finished
	for ev, err := range finished {
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("EventLoadingFinished: %v", ev.RequestID)
		if ev.RequestID == requestID {
			break
		}
	}

	// get the downloaded bytes for the request id
	buf, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, network.GetResponseBody, network.GetResponseBodyParams{RequestID: requestID})
		if err != nil {
			return nil, err
		}
		// the typed cdproto does not decode the body for us
		if res.Base64encoded {
			return base64.StdEncoding.DecodeString(res.Body)
		}
		return []byte(res.Body), nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// write the file to disk - since we hold the bytes we dictate the name and
	// location
	if err := os.WriteFile("download.png", buf, 0644); err != nil {
		log.Fatal(err)
	}
	log.Print("wrote download.png")
}
