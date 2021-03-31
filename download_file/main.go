// This example demonstrates how to perform a headless file download. Note that for this technique
// to work, the file type must trigger the "Download / Save As" browser dialog. See the download_image
// example for how to save a file which would load inside the browser window without triggering a download.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/page"
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

	// this will be used to capture the file name later
	var downloadGUID string

	// set up a listener to watch the download events and close the channel when complete
	// this could be expanded to handle multiple downloads through creating a guid map,
	// monitor download urls via EventDownloadWillBegin, etc
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if evt, ok := ev.(*page.EventDownloadProgress); ok {
			fmt.Printf("current download state: %s\n", evt.State.String())
			if evt.State == page.DownloadProgressStateCompleted {
				downloadGUID = evt.GUID
				close(downloadComplete)
			}
		}
	})

	// download the zip of the chromedp/examples repo from github. we use a link click method here
	// but this could also be done with a chromedp.Navigate task which points directly at the file we
	// want to download, as long as you run browser.SetDownloadBehavior first
	if err := chromedp.Run(ctx,
		// navigate to the page
		chromedp.Navigate(`https://github.com/chromedp/examples`),
		// find and click "Code" button when ready
		chromedp.Click(`//get-repo//summary`, chromedp.NodeReady),
		// configure headless browser downloads. note that SetDownloadBehaviorBehaviorAllowAndName is
		// preferred here over SetDownloadBehaviorBehaviorAllow so that the file will be named as the GUID
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).WithDownloadPath(os.TempDir()),
		// click the "Download Zip" link when visible
		chromedp.Click(`//get-repo//a[contains(@data-ga-click, "download zip")]`, chromedp.NodeVisible),
	); err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
		// Note: Ignoring the net::ERR_ABORTED page error is essential here since downloads
		// will cause this error to be emitted, although the download will still succeed.
		log.Fatal(err)
	}

	// This will block until the chromedp listener closes the channel
	<-downloadComplete

	// We can predict the exact file location and name here because of how we configured
	// SetDownloadBehavior and WithDownloadPath
	log.Printf("Download Complete: %v/%v", os.TempDir(), downloadGUID)
}
