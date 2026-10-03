// Command tabs is a chromedp example demonstrating how to use several tabs of
// one browser. It opens three tabs at the same time, and then shows that tabs
// share cookies, unless a tab has its own browser context. It starts a local
// server and needs no internet. Use -v to print the protocol messages and
// -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

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

	// The first Run starts the browser, and uses the tab that it opens. A
	// context that you create from this context with NewContext uses the same
	// browser, and its first Run opens a new tab in a new window.
	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/page?name=first&delay=0")); err != nil {
		log.Fatal(err)
	}

	if err := parallel(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
	if err := cookies(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
}

// parallel opens one tab for each page, at the same time. Each page waits for
// 500 ms on the server, so the total time is close to 500 ms, not to 1500 ms.
func parallel(ctx context.Context, host string) error {
	names := []string{"one", "two", "three"}
	titles := make([]string, len(names))
	errs := make([]error, len(names))

	start := time.Now()
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			// A new context from the context of the browser is a new tab.
			// Cancel closes the tab.
			tab, cancel := chromedp.NewContext(ctx)
			defer cancel()
			url := host + "/page?delay=500&name=" + name
			if errs[i] = chromedp.Do(tab, chromedp.Navigate(url)); errs[i] != nil {
				return
			}
			titles[i], errs[i] = chromedp.Run(tab, chromedp.Title())
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	log.Printf("%d tabs loaded in %s: %q", len(names), time.Since(start).Round(100*time.Millisecond), titles)
	return nil
}

// cookies sets a cookie in one tab, and reads it in a second tab and in a third
// tab. The second tab shares the cookies of the browser. The third tab has its
// own browser context, like a private window, and does not see the cookie.
func cookies(ctx context.Context, host string) error {
	tab1, cancel := chromedp.NewContext(ctx)
	defer cancel()
	if err := chromedp.Do(tab1, chromedp.Navigate(host+"/login")); err != nil {
		return err
	}

	tab2, cancel := chromedp.NewContext(ctx)
	defer cancel()
	shared, err := whoami(tab2, host)
	if err != nil {
		return err
	}
	log.Printf("a tab in the same browser context sees: %s", shared)

	tab3, cancel := chromedp.NewContext(ctx, chromedp.WithNewBrowserContext())
	defer cancel()
	private, err := whoami(tab3, host)
	if err != nil {
		return err
	}
	log.Printf("a tab in a new browser context sees: %s", private)
	return nil
}

// whoami reads the text of the page that shows the cookies of the request.
func whoami(ctx context.Context, host string) (string, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/whoami")); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(chromedp.CSS("body")))
}

// newMux returns the handlers of the test server. The page /page waits for
// delay milliseconds and uses name as its title. The page /login sets a cookie.
// The page /whoami shows the cookies of the request.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		delay, _ := strconv.Atoi(r.URL.Query().Get("delay"))
		time.Sleep(time.Duration(delay) * time.Millisecond)
		name := r.URL.Query().Get("name")
		fmt.Fprintf(w, "<html><head><title>%s</title></head><body>%s</body></html>", name, name)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		fmt.Fprint(w, "<html><body>logged in</body></html>")
	})
	mux.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		text := "no cookie"
		if c, err := r.Cookie("session"); err == nil {
			text = c.Name + "=" + c.Value
		}
		fmt.Fprintf(w, "<html><body>%s</body></html>", text)
	})
	return mux
}
