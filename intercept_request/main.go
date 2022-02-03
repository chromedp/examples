// Command intercept_request is a chromedp example demonstrating how to do
// headless intercept request to download files. Useful if the download is trigger in a new tab
//
package main

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

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

	// set up a channel, so we can block later while we monitor the download
	// progress
	done := make(chan bool)

	// set up a listener to watch the fetch events and close the channel when
	// expected event are fired and the file is downloaded
	chromedp.ListenTarget(ctx, func(v interface{}) {
		switch ev := v.(type) {
		case *fetch.EventRequestPaused:
			go func() {
				c := chromedp.FromContext(ctx)
				e := cdp.WithExecutor(ctx, c.Target)

				// find the matching request to intercept it.
				// The request will be blocked and processed with go http client
				if strings.HasSuffix(ev.Request.URL, "tar.gz") {
					if err := fetch.FailRequest(ev.RequestID, network.ErrorReasonBlockedByClient).Do(e); err != nil {
						log.Printf("Failed to abort request: %v", err)
					}

					// pass the request
					downloadFile(ev.Request)

					// we're done, closing the channel
					done <- true
				} else {
					// continue others events
					if err := fetch.ContinueRequest(ev.RequestID).Do(e); err != nil {
						log.Printf("Failed to continue request: %v", err)
					}
				}
			}()
		}
	})

	// navigating to the latest chromedp releases and download
	if err := chromedp.Run(ctx,
		fetch.Enable(),
		// not needed because the download request is blocked but kept to illustration that is really no download
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorDeny),
		chromedp.Navigate("https://github.com/chromedp/chromedp/releases/latest"),
		chromedp.Click("//a[contains(@href,'tar.gz')]"),
	); err != nil {
		log.Fatal(err)
	}

	// This will block until the chromedp listener closes the channel
	<-done
}

func downloadFile(r *network.Request) {
	var req *http.Request
	var err error
	if r.HasPostData {
		req, err = http.NewRequest(r.Method, r.URL, strings.NewReader(r.PostData))
	} else {
		req, err = http.NewRequest(r.Method, r.URL, nil)
	}

	if err != nil {
		log.Fatalf("failed to create http client: %v\n", err)
	}

	// adding all request headers including cookies
	for key, val := range r.Headers {
		req.Header.Add(key, fmt.Sprintf("%v", val))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to send http request: %v\n", err)
	}

	// expect status code 200
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("bad status: %s\n", resp.Status)
	}

	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)

	// write the file to disk - since we hold the bytes we dictate the name and
	// location
	if err := ioutil.WriteFile("source.tar.gz", buf, 0644); err != nil {
		log.Fatal(err)
	}
	log.Print("wrote source.tar.gz")
}
