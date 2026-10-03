// Command download_file is a chromedp example demonstrating how to do headless
// file downloads.
//
// Note that for this technique to work, the file type must trigger the
// "Download / Save As" browser dialog. See the download_image example for how
// to save a file which would load inside the browser window without triggering
// a download.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithDebugf(log.Printf),
	)
	defer cancel()

	// create a timeout as a safety net to prevent any infinite wait loops
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// subscribe to the download events, so we can watch the download
	// progress later. This could be expanded to handle multiple downloads
	// through creating a guid map, monitor download urls via
	// browser.DownloadWillBegin, etc
	progress := chromedp.Events(ctx, browser.DownloadProgress)

	// get working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// download the zip of the chromedp/examples repo from GitHub. We use a
	// link click method here but this could also be done with a
	// chromedp.Navigate task which points directly at the file we want to
	// download, as long as you run browser.SetDownloadBehavior first
	if err := chromedp.Do(ctx,
		// navigate to the page
		chromedp.Navigate(`https://github.com/chromedp/examples`),
		// find and click "Code" button when ready
		chromedp.Click(`//button//span[text()="Code"]`, chromedp.NodeReady),
		// configure headless browser downloads. note that
		// SetDownloadBehaviorBehaviorAllowAndName is preferred here over
		// SetDownloadBehaviorBehaviorAllow so that the file will be named as
		// the GUID. please note that it only works with 92.0.4498.0 or later
		// due to issue 1204880, see https://bugs.chromium.org/p/chromium/issues/detail?id=1204880
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			_, err := cdp.Call(ctx, t, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{
				Behavior:      browser.SetDownloadBehaviorBehaviorAllowAndName,
				DownloadPath:  wd,
				EventsEnabled: new(true),
			})
			return err
		}),
		// click the "Download Zip" link when visible
		chromedp.Click(`//span[text()="Download ZIP"]`, chromedp.NodeVisible),
	); err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
		// Note: Ignoring the net::ERR_ABORTED page error is essential here
		// since downloads will cause this error to be emitted, although the
		// download will still succeed.
		log.Fatal(err)
	}

	// This will block until the download is complete
	var guid string
	for ev, err := range progress {
		if err != nil {
			log.Fatal(err)
		}
		completed := "(unknown)"
		if ev.TotalBytes != 0 {
			completed = fmt.Sprintf("%0.2f%%", ev.ReceivedBytes/ev.TotalBytes*100.0)
		}
		log.Printf("state: %s, completed: %s\n", ev.State.String(), completed)
		if ev.State == browser.DownloadProgressStateCompleted {
			guid = ev.GUID
			break
		}
	}

	// We can predict the exact file location and name here because of how we
	// configured SetDownloadBehavior and WithDownloadPath
	log.Printf("wrote %s", filepath.Join(wd, guid))
}
