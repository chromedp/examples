// Command download_image is a chromedp example demonstrating how to do headless
// image downloads. It reads avatars.githubusercontent.com and writes
// download.png to the current directory. For this technique to work, the file
// type must load inside the browser window without a download. See the
// download_file example for how to save a file that triggers the "Download /
// Save As" browser dialog. Use -v to print the protocol messages and -visible to
// show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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
		opts = append(opts, chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
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

	// set the download URL to the avatar of the chromedp user on GitHub
	urlstr := "https://avatars.githubusercontent.com/u/33149672"

	// subscribe to the network events, so that the program can watch them
	// after the navigation. The request ID filters out the events of other
	// requests, and it finds the downloaded file later
	requests := chromedp.Events(ctx, network.RequestWillBeSent)
	finished := chromedp.Events(ctx, network.LoadingFinished)

	// navigate to the download URL
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		log.Fatal(err)
	}

	// the request ID of the download, to match the later events
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

	// wait until the request with this ID is finished
	for ev, err := range finished {
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("EventLoadingFinished: %v", ev.RequestID)
		if ev.RequestID == requestID {
			break
		}
	}

	// get the downloaded bytes of the request
	buf, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, network.GetResponseBody, network.GetResponseBodyParams{RequestID: requestID})
		if err != nil {
			return nil, err
		}
		// the result holds the decoded body
		return res.Body, nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// write the file to disk. The program holds the bytes, so it chooses the
	// name and the location
	if err := os.WriteFile("download.png", buf, 0644); err != nil {
		log.Fatal(err)
	}
	log.Print("wrote download.png")
}
