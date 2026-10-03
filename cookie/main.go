// Command cookie is a chromedp example demonstrating how to set an HTTP cookie
// on requests. It starts a local server and needs no internet. Use -v to print
// the protocol messages and -visible to show the browser window and leave it
// open.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	port := flag.Int("port", 8544, "port of the local web server")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the cookie server
	go cookieServer(fmt.Sprintf(":%d", *port))

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

	// run the actions
	res, err := setcookies(
		ctx, fmt.Sprintf("http://localhost:%d", *port),
		"cookie1", "value1",
		"cookie2", "value2",
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("chrome received cookies: %s", res)
}

// cookieServer serves a page that shows the cookies of the request. It logs
// each cookie too.
func cookieServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		cookies := req.Cookies()
		for i, cookie := range cookies {
			log.Printf("from %s, server received cookie %d: %v", req.RemoteAddr, i, cookie)
		}
		buf, err := json.MarshalIndent(req.Cookies(), "", "  ")
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(res, indexHTML, string(buf))
	})
	return http.ListenAndServe(addr, mux)
}

// setcookies navigates to a host with the passed cookies set on the network
// request. It returns the text that the page shows.
func setcookies(ctx context.Context, host string, cookies ...string) (string, error) {
	if len(cookies)%2 != 0 {
		panic("length of cookies must be divisible by 2")
	}
	// set the cookies in the browser
	expires := cdp.TimeSinceEpoch(time.Now().Add(180 * 24 * time.Hour).Unix())
	if err := chromedp.Do(ctx,
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			for i := 0; i < len(cookies); i += 2 {
				_, err := cdp.Call(ctx, t, network.SetCookie, network.SetCookieParams{
					Name:     cookies[i],
					Value:    cookies[i+1],
					Expires:  expires,
					Domain:   "localhost",
					HTTPOnly: new(true),
				})
				if err != nil {
					return err
				}
			}
			return nil
		}),
		// navigate to the site
		chromedp.Navigate(host),
	); err != nil {
		return "", err
	}
	// read the text that the page shows
	res, err := chromedp.Run(ctx, chromedp.Text(chromedp.ID("result"), chromedp.NodeVisible))
	if err != nil {
		return "", err
	}
	// read the cookies of the browser
	got, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]*network.Cookie, error) {
		res, err := cdp.Call(ctx, t, storage.GetCookies, storage.GetCookiesParams{})
		return res.Cookies, err
	})
	if err != nil {
		return "", err
	}
	for i, cookie := range got {
		log.Printf("chrome cookie %d: %+v", i, cookie)
	}
	return res, nil
}

const (
	indexHTML = `<!doctype html>
<html>
<body>
  <div id="result">%s</div>
</body>
</html>`
)
