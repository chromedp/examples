// This example demonstrates how to perform a headless image download. Note that for this technique
// to work, the file type must load inside the browser window without triggering a download. See the
// download_file example for how to save a file that triggers the "Download / Save As" browser dialog.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func main() {
	// create chrome instance
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// create a timeout as a safety net to prevent any infinite wait loops
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// set up a channel so we can block later while we monitor the download progress
	downloadComplete := make(chan bool)

	// set the download url as the chromedp github user avatar
	downloadURL := "https://avatars.githubusercontent.com/u/33149672"

	// this will be used to capture the request id for matching network events
	var requestId network.RequestID

	// set up a listener to watch the network events and close the channel when complete
	// the request id matching is important both to filter out unwanted network events
	// and to reference the downloaded file later
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			fmt.Printf("EventRequestWillBeSent: %v: %v\n", ev.RequestID, ev.Request.URL)
			if ev.Request.URL == downloadURL {
				requestId = ev.RequestID
			}
		case *network.EventLoadingFinished:
			fmt.Printf("EventLoadingFinished: %v\n", ev.RequestID)
			if ev.RequestID == requestId {
				close(downloadComplete)
			}
		}
	})

	// all we need to do here is navigate to the download url
	if err := chromedp.Run(ctx,
		chromedp.Navigate(downloadURL),
	); err != nil {
		log.Fatal(err)
	}

	// This will block until the chromedp listener closes the channel
	<-downloadComplete

	// get the downloaded bytes for the request id
	var downloadBytes []byte
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		downloadBytes, err = network.GetResponseBody(requestId).Do(ctx)
		return err
	})); err != nil {
		log.Fatal(err)
	}

	// write the file to disk - since we hold the bytes we dictate the name and location
	downloadDest := fmt.Sprintf("%v/download.png", os.TempDir())
	if err := ioutil.WriteFile(downloadDest, downloadBytes, 0777); err != nil {
		log.Fatal(err)
	}

	log.Printf("Download Complete: %v", downloadDest)
}
