// Command popups is a chromedp example demonstrating how to work with the
// popups of a page and with several targets. A target is a tab, a popup or
// another page that the browser runs. The program loads a local page, and it
// clicks a link with target="_blank" and a button that calls window.open. For
// each click it finds the new target with WaitNewTarget, attaches a context to
// it with WithTargetID, and prints the title of the popup. It then shows that a
// browser context made with WithNewBrowserContext does not share the cookies of
// the other tabs. A popup starts to load before the program can attach to it,
// so the program cannot pause the first request of a popup. The last part shows
// the way around: create the tab, enable the Fetch domain, and then navigate.
// It starts a local server and needs no internet. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
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

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		log.Fatal(err)
	}
	if err := popups(ctx); err != nil {
		log.Fatal(err)
	}
	if err := isolated(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
	if err := paused(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
}

// popups clicks the link and the button, and reads the title of each popup.
func popups(ctx context.Context) error {
	for _, id := range []string{"link", "button"} {
		// Start to wait before the click. WaitNewTarget watches the targets
		// that the current target opens, and it sends the ID of a new one
		// on the channel. The function chooses which targets count.
		found := chromedp.WaitNewTarget(ctx, func(info *target.Info) bool {
			return info.Type == "page"
		})
		if err := chromedp.Do(ctx, chromedp.Click(chromedp.ID(id))); err != nil {
			return fmt.Errorf("clicking %s: %w", id, err)
		}

		var targetID target.ID
		select {
		case targetID = <-found:
		case <-time.After(10 * time.Second):
			return fmt.Errorf("waiting for the popup of %s: no new target", id)
		case <-ctx.Done():
			return ctx.Err()
		}

		// A context with WithTargetID attaches to the existing target, and
		// it does not open a new tab. The first Run attaches. Wait for the
		// heading, because the popup can still load.
		popup, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(targetID))
		defer cancel()
		if err := chromedp.Do(popup, chromedp.WaitVisible(chromedp.CSS("h1"))); err != nil {
			return fmt.Errorf("waiting for the popup of %s: %w", id, err)
		}
		title, err := chromedp.Run(popup, chromedp.Title())
		if err != nil {
			return fmt.Errorf("reading the title of the popup of %s: %w", id, err)
		}
		fmt.Printf("popup of the %s: title %q, target %s\n", id, title, targetID)
	}
	return nil
}

// isolated sets a cookie in the page, and reads the cookies of a popup and of a
// tab in a new browser context.
func isolated(ctx context.Context, host string) error {
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/login")); err != nil {
		return fmt.Errorf("logging in: %w", err)
	}

	// A new tab of the same browser context sees the cookie.
	same, cancel := chromedp.NewContext(ctx)
	defer cancel()
	text, err := whoami(same, host)
	if err != nil {
		return err
	}
	fmt.Printf("a tab in the same browser context sees: %s\n", text)

	// A browser context is like a private window. It has its own cookies and
	// its own storage. The context disposes of it when the program cancels
	// the context.
	private, cancel := chromedp.NewContext(ctx, chromedp.WithNewBrowserContext())
	defer cancel()
	text, err = whoami(private, host)
	if err != nil {
		return err
	}
	fmt.Printf("a tab in a new browser context sees: %s\n", text)
	return nil
}

// whoami reads the text of the page that shows the cookie of the request.
func whoami(ctx context.Context, host string) (string, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/whoami")); err != nil {
		return "", fmt.Errorf("loading the page that shows the cookie: %w", err)
	}
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("body")))
	if err != nil {
		return "", fmt.Errorf("reading the page that shows the cookie: %w", err)
	}
	return text, nil
}

// paused pauses the first request of a page that the program opens itself.
// A popup that a click opens has already sent its first request when the
// program learns about the target. So create the tab with the command
// Target.createTarget, attach to it, enable the Fetch domain, and only then
// navigate. The command goes to the browser, because a tab does not create
// tabs.
func paused(ctx context.Context, host string) error {
	created, err := chromedp.CallBrowser(ctx, target.CreateTarget, target.CreateTargetParams{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("creating the tab: %w", err)
	}
	tab, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(created.TargetID))
	defer cancel()

	// Attach first. The first call on a context starts the work, and the
	// events below need the context to be attached.
	if err := chromedp.Do(tab); err != nil {
		return fmt.Errorf("attaching to the tab: %w", err)
	}
	requests := chromedp.Events(tab, fetch.RequestPaused)
	if _, err := chromedp.Call(tab, fetch.Enable, fetch.EnableParams{}); err != nil {
		return fmt.Errorf("enabling the Fetch domain: %w", err)
	}

	// The goroutine answers each paused request. Here it answers with a page
	// of its own, so the server never sees the request.
	urls := make(chan string, 1)
	go func() {
		for ev, err := range requests {
			if err != nil {
				return
			}
			_, err := chromedp.Call(tab, fetch.FulfillRequest, fetch.FulfillRequestParams{
				RequestID:    ev.RequestID,
				ResponseCode: 200,
				ResponseHeaders: []*fetch.HeaderEntry{
					{Name: "Content-Type", Value: "text/html"},
				},
				Body: []byte(`<html><body><h1>answered by the program</h1></body></html>`),
			})
			if err != nil {
				log.Printf("answering the request for %s: %v", ev.Request.URL, err)
			}
			select {
			case urls <- ev.Request.URL:
			default:
			}
		}
	}()

	if err := chromedp.Do(tab, chromedp.Navigate(host+"/popup?name=paused")); err != nil {
		return fmt.Errorf("loading the page in the tab: %w", err)
	}
	text, err := chromedp.Run(tab, chromedp.Text(chromedp.CSS("h1")))
	if err != nil {
		return fmt.Errorf("reading the page in the tab: %w", err)
	}
	fmt.Printf("the first request of the new tab, %s, got the page %q\n", <-urls, text)
	return nil
}

// newMux returns the handlers of the test server. The page / has a link and a
// button that open popups. The page /popup uses name as its title.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>opener</title></head><body>
<a id="link" href="/popup?name=link" target="_blank">open with a link</a>
<button id="button" onclick="window.open('/popup?name=button')">open with a script</button>
</body></html>`)
	})
	mux.HandleFunc("/popup", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		fmt.Fprintf(w, `<html><head><title>popup of the %s</title></head><body><h1>%s</h1></body></html>`, name, name)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Path: "/"})
		fmt.Fprint(w, `<html><body>logged in</body></html>`)
	})
	mux.HandleFunc("/whoami", func(w http.ResponseWriter, r *http.Request) {
		text := "no cookie"
		if c, err := r.Cookie("session"); err == nil {
			text = c.Name + "=" + c.Value
		}
		fmt.Fprintf(w, `<html><body>%s</body></html>`, text)
	})
	return mux
}
