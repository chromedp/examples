// Command proxy is a chromedp example demonstrating how to authenticate to a
// proxy server that requires authentication. It starts a local proxy and a
// local web server, and needs no internet. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// create a simple proxy that requires authentication
	p := httptest.NewServer(newProxy())
	defer p.Close()

	// create a web server
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "test")
	}))
	defer s.Close()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		// 1) specify the proxy server.
		// The user name and the password are not set here.
		// The link below describes the proxy settings:
		// https://www.chromium.org/developers/design-documents/network-settings
		chromedp.ProxyServer(p.URL),
		// By default, Chrome bypasses the proxy for localhost.
		// The test server listens on localhost, so this flag makes Chrome use
		// the proxy for localhost URLs.
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	)
	if *visible {
		opts = append(opts, chromedp.VisibleWindow, chromedp.KeepOpen)
	}
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	// log the protocol messages with -v
	var copts []chromedp.ContextOption
	if *verbose {
		copts = append(copts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel = chromedp.NewContext(ctx, copts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// 3) handle the Fetch.AuthRequired event, and give the user name and the
	// password to the proxy. After the proxy accepts them, the code disables
	// the fetch domain and stops the event loops, to reduce the overhead. If
	// your project needs the fetch domain, change the code.
	// Start the browser first. A browser that Events starts lives only as long
	// as lctx, so lcancel closes it.
	if err := chromedp.Do(ctx); err != nil {
		log.Fatal(err)
	}
	lctx, lcancel := context.WithCancel(ctx)
	defer lcancel()
	paused := chromedp.Events(lctx, fetch.RequestPaused)
	authRequired := chromedp.Events(lctx, fetch.AuthRequired)
	go func() {
		for ev, err := range paused {
			if err != nil {
				return
			}
			_, _ = chromedp.Call(ctx, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: ev.RequestID})
		}
	}()
	go func() {
		for ev, err := range authRequired {
			if err != nil {
				return
			}
			if ev.AuthChallenge.Source != fetch.AuthChallengeSourceProxy {
				continue
			}
			_, _ = chromedp.Call(ctx, fetch.ContinueWithAuth, fetch.ContinueWithAuthParams{
				RequestID: ev.RequestID,
				AuthChallengeResponse: &fetch.AuthChallengeResponse{
					Response: fetch.AuthChallengeResponseResponseProvideCredentials,
					Username: "u",
					Password: "p",
				},
			})
			// Chrome remembers the credentials for the current instance, so
			// the code can disable the fetch domain after it gives them. If
			// Chrome does not work this way, file an issue.
			_, _ = chromedp.Call(ctx, fetch.Disable, cdp.Empty{})
			// stop the event loops too.
			lcancel()
			return
		}
	}()

	// 2) enable the fetch domain to handle the Fetch.AuthRequired event
	if _, err := chromedp.Call(ctx, fetch.Enable, fetch.EnableParams{HandleAuthRequests: new(true)}); err != nil {
		log.Fatal(err)
	}
	if err := chromedp.Do(ctx, chromedp.Navigate(s.URL)); err != nil {
		log.Fatal(err)
	}

	// navigate in a new tab, to show that the proxy accepts later requests
	// from other tabs too.
	tctx, cancel := chromedp.NewContext(ctx)
	defer cancel()
	if err := chromedp.Do(tctx, chromedp.Navigate(s.URL+"/tab")); err != nil {
		log.Fatal(err)
	}
}

// newProxy creates a proxy that requires authentication.
func newProxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			if dump, err := httputil.DumpRequest(r, true); err == nil {
				log.Printf("%s", dump)
			}
			// the user name and the password are "u:p" (base64: dTpw), to keep the
			// example simple
			if auth := r.Header.Get("Proxy-Authorization"); auth != "Basic dTpw" {
				r.Header.Set("X-Failed", "407")
			}
		},
		Transport: &transport{http.DefaultTransport},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if err.Error() == "407" {
				log.Println("proxy: not authorized")
				w.Header().Add("Proxy-Authenticate", `Basic realm="Proxy Authorization"`)
				w.WriteHeader(407)
			} else {
				w.WriteHeader(http.StatusBadGateway)
			}
		},
	}
}

// transport fails a request that has the header X-Failed. The proxy director
// sets this header when the credentials are wrong.
type transport struct {
	http.RoundTripper
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if h := r.Header.Get("X-Failed"); h != "" {
		return nil, errors.New(h)
	}
	return t.RoundTripper.RoundTrip(r)
}
