// Command tabs is a chromedp example demonstrating how to use several tabs of
// one browser. It opens three tabs at the same time, and then shows that tabs
// share cookies, unless a tab has its own browser context. It starts a local
// server and needs no internet. By default, each tab opens in its own window.
// Use -tabs-in-one-window to open the tabs in one window and to switch between
// them with the protocol command Target.activateTarget. Use -v to print the
// protocol messages. Use -visible to show the browser window and keep every tab
// open until you close the browser.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and keep every tab open until you close the browser")
	oneWindow := flag.Bool("tabs-in-one-window", false, "open the tabs in the window of the browser, not in new windows")
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
		// Closing a tab context closes the tab. This program keeps the
		// tabs open, so it waits for the user to close the browser at the
		// end, and it does not use WithKeepOpen.
		opts = append(opts, chromedp.WithVisibleWindow())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()

	// The first Run starts the browser, and uses the tab that it opens. A
	// context that you create from this context with NewContext uses the same
	// browser, and its first Run opens a new tab in a new window.
	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/page?name=first&delay=0")); err != nil {
		log.Fatal(err)
	}

	// The option WithNewWindow(false) opens a tab in the window of the browser.
	// Without it, a new tab opens in a new window.
	var tabOpts []chromedp.ContextOption
	if *oneWindow {
		tabOpts = append(tabOpts, chromedp.WithNewWindow(false))
	}
	t := &tabs{host: srv.URL, keep: *visible, opts: tabOpts}

	if err := t.parallel(ctx); err != nil {
		log.Fatal(err)
	}
	if err := t.cookies(ctx); err != nil {
		log.Fatal(err)
	}
	pause := time.Duration(0)
	if *visible {
		pause = time.Second
	}
	if err := t.switching(ctx, pause); err != nil {
		log.Fatal(err)
	}

	// Wait until the user closes the browser. The tabs stay open until then.
	if *visible {
		infos, err := chromedp.Targets(ctx)
		if err != nil {
			log.Fatal(err)
		}
		for _, info := range infos {
			if info.Type == "page" {
				log.Printf("open tab: %s", info.URL)
			}
		}
		fmt.Fprintln(os.Stderr, "close the browser window to stop the program")
		if err := chromedp.WaitClosed(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

// tabs holds what the steps of the program share.
type tabs struct {
	host string
	// keep is true when the tabs must stay open until the user closes the
	// browser.
	keep bool
	// opts are the options of every new tab.
	opts []chromedp.ContextOption
}

// newTab creates a context for a new tab of the browser of ctx. Canceling the
// context closes the tab, so the cancel function does nothing when keep is
// true.
func (t *tabs) newTab(ctx context.Context, opts ...chromedp.ContextOption) (context.Context, context.CancelFunc) {
	tab, cancel := chromedp.NewContext(ctx, append(slices.Clone(t.opts), opts...)...)
	if t.keep {
		return tab, func() {}
	}
	return tab, cancel
}

// parallel opens one tab for each page, at the same time. Each page waits for
// 500 ms on the server, so the total time is close to 500 ms, not to 1500 ms.
func (t *tabs) parallel(ctx context.Context) error {
	names := []string{"one", "two", "three"}
	titles := make([]string, len(names))
	errs := make([]error, len(names))

	start := time.Now()
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			// A new context from the context of the browser is a new tab.
			// Cancel closes the tab.
			tab, cancel := t.newTab(ctx)
			defer cancel()
			url := t.host + "/page?delay=500&name=" + name
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
func (t *tabs) cookies(ctx context.Context) error {
	tab1, cancel := t.newTab(ctx)
	defer cancel()
	if err := chromedp.Do(tab1, chromedp.Navigate(t.host+"/login")); err != nil {
		return err
	}

	tab2, cancel := t.newTab(ctx)
	defer cancel()
	shared, err := t.whoami(tab2)
	if err != nil {
		return err
	}
	log.Printf("a tab in the same browser context sees: %s", shared)

	tab3, cancel := t.newTab(ctx, chromedp.WithNewBrowserContext())
	defer cancel()
	private, err := t.whoami(tab3)
	if err != nil {
		return err
	}
	log.Printf("a tab in a new browser context sees: %s", private)
	return nil
}

// whoami reads the text of the page that shows the cookies of the request.
func (t *tabs) whoami(ctx context.Context) (string, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(t.host+"/whoami")); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(chromedp.CSS("body")))
}

// switching opens three tabs, and then makes each one the active tab with the
// protocol command Target.activateTarget. After each switch, it asks every tab
// for its document.visibilityState. In a window of its own, a tab is always
// visible. In a window that holds several tabs, only the active tab is visible,
// and a hidden page gets no animation frames. pause is the time to look at each
// tab.
func (t *tabs) switching(ctx context.Context, pause time.Duration) error {
	names := []string{"one", "two", "three"}
	var tabs []context.Context
	for _, name := range names {
		tab, cancel := t.newTab(ctx)
		defer cancel()
		if err := chromedp.Do(tab, chromedp.Navigate(t.host+"/page?delay=0&name="+name)); err != nil {
			return err
		}
		tabs = append(tabs, tab)
	}

	// Target.activateTarget is a command of the browser, not of a tab.
	browser := chromedp.FromContext(ctx).Browser
	for i, tab := range tabs {
		id := chromedp.FromContext(tab).Target.TargetID
		if _, err := cdp.Call(ctx, browser, target.ActivateTarget, target.ActivateTargetParams{TargetID: id}); err != nil {
			return fmt.Errorf("activating the tab %q: %w", names[i], err)
		}
		time.Sleep(pause)

		states := make([]string, len(tabs))
		for j, other := range tabs {
			state, err := chromedp.Run(other, chromedp.Evaluate[string](`document.visibilityState`))
			if err != nil {
				return err
			}
			states[j] = names[j] + "=" + state
		}
		log.Printf("active tab %q: %v", names[i], states)
	}
	return nil
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
