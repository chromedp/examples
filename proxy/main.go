// Command proxy is a chromedp example demonstrating how to authenticate a proxy
// server which requires authentication.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
)

func main() {
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
		// Note that the username/password is not provided here.
		// Check the link below for the description of the proxy settings:
		// https://www.chromium.org/developers/design-documents/network-settings
		chromedp.ProxyServer(p.URL),
		// By default, Chrome will bypass localhost.
		// The test server is bound to localhost, so we should add the
		// following flag to use the proxy for localhost URLs.
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	)
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	// log the protocol messages to understand how it works.
	ctx, cancel = chromedp.NewContext(ctx, chromedp.WithDebugf(log.Printf))
	defer cancel()

	// 3) handle the Fetch.AuthRequired event and provide the username/password to the proxy
	// We will disable the fetch domain and cancel the event handler once the proxy is
	// authenticated to reduce the overhead. If your project needs the fetch domain to be enabled,
	// then you should change the code accordingly.
	lctx, lcancel := context.WithCancel(ctx)
	chromedp.ListenTarget(lctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *fetch.EventRequestPaused:
			go func() {
				_ = chromedp.Run(ctx, fetch.ContinueRequest(ev.RequestID))
			}()
		case *fetch.EventAuthRequired:
			if ev.AuthChallenge.Source == fetch.AuthChallengeSourceProxy {
				go func() {
					_ = chromedp.Run(ctx,
						fetch.ContinueWithAuth(ev.RequestID, &fetch.AuthChallengeResponse{
							Response: fetch.AuthChallengeResponseResponseProvideCredentials,
							Username: "u",
							Password: "p",
						}),
						// Chrome will remember the credential for the current instance,
						// so we can disable the fetch domain once credential is provided.
						// Please file an issue if Chrome does not work in this way.
						fetch.Disable(),
					)
					// and cancel the event handler too.
					lcancel()
				}()
			}
		}
	})

	if err := chromedp.Run(ctx,
		// 2) enable the fetch domain to handle the Fetch.AuthRequired event
		fetch.Enable().WithHandleAuthRequests(true),
		chromedp.Navigate(s.URL),
	); err != nil {
		log.Fatal(err)
	}

	// to show that further requests (even in new tabs) are authenticated.
	tctx, cancel := chromedp.NewContext(ctx)
	defer cancel()
	if err := chromedp.Run(tctx,
		chromedp.Navigate(s.URL+"/tab"),
	); err != nil {
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
			// hardcode username/password "u:p" (base64 encoded: dTpw ) to make it simple
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

type transport struct {
	http.RoundTripper
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if h := r.Header.Get("X-Failed"); h != "" {
		return nil, fmt.Errorf(h)
	}
	return t.RoundTripper.RoundTrip(r)
}
