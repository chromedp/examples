// Command download_file is a chromedp example demonstrating how to do headless
// file downloads. It reads github.com and writes the file to the current
// directory. For this technique to work, the file type must trigger the
// "Download / Save As" browser dialog. See the download_image example for how to
// save a file that the browser window loads without a download. Use -v to print
// the protocol messages and -visible to show the browser window and leave it
// open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

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

	// create a timeout, so that no wait loop can run forever
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// subscribe to the download events, so that the program can watch the
	// download progress later. To handle many downloads, a program can keep a
	// map of the GUID values and read the URLs from browser.DownloadWillBegin
	progress := chromedp.Events(ctx, browser.DownloadProgress)

	// get working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// download the zip of the chromedp/examples repository from GitHub. This
	// program clicks a link. A program can also navigate to the file, if it
	// runs browser.SetDownloadBehavior first
	if err := chromedp.Do(ctx,
		// navigate to the page
		chromedp.Navigate(`https://github.com/chromedp/examples`),
		// find the "Code" button, and click it when it is ready
		chromedp.Click(`//button//span[text()="Code"]`, chromedp.NodeReady),
		// configure headless browser downloads. Use
		// SetDownloadBehaviorBehaviorAllowAndName and not
		// SetDownloadBehaviorBehaviorAllow, so that Chrome names the file with
		// its GUID. It works only with Chrome 92.0.4498.0 or later, because of
		// issue 1204880, see https://bugs.chromium.org/p/chromium/issues/detail?id=1204880
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			_, err := cdp.Call(ctx, t, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{
				Behavior:      browser.SetDownloadBehaviorBehaviorAllowAndName,
				DownloadPath:  wd,
				EventsEnabled: new(true),
			})
			return err
		}),
		// click the "Download Zip" link when it is visible
		chromedp.Click(`//span[text()="Download ZIP"]`, chromedp.NodeVisible),
	); err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
		// Ignore the net::ERR_ABORTED page error. A download causes this
		// error, but the download still succeeds.
		log.Fatal(err)
	}

	// wait until the download is complete
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

	// the location and the name of the file are known, because of the download
	// path and the behavior that the program set with SetDownloadBehavior
	log.Printf("wrote %s", filepath.Join(wd, guid))
}
