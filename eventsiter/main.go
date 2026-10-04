// Command eventsiter is a chromedp example demonstrating how to listen to the
// events of a page with iterators. chromedp.Events returns an iterator that
// the program reads with a for loop. The program loads a local page that logs
// to the console, loads an image and a script, and sends a request later with
// a script. It collects the console messages, the network requests and the
// network responses. It waits for the network to be idle with the event
// Page.lifecycleEvent. It then reads the console messages with a loop that ends with
// break, and it waits for one console message that matches a condition with
// WaitEvent. A listener stops when its context ends, so the program cancels the
// context of the network listeners. It starts a local server and needs no
// internet. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
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

	if err := run(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
}

// run loads the page, and reads the events of the load.
func run(ctx context.Context, host string) error {
	// An empty Do starts the browser. The first call on a context binds the
	// browser to it, and the program cancels some contexts below, so the
	// first call must use the main context.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// Subscribe before the navigation. An iterator starts to buffer events
	// when Events returns, and not when the loop starts.
	console := chromedp.Events(ctx, runtime.ConsoleAPICalled)

	// The network listeners run in goroutines until the program cancels their
	// context. A loop over an iterator also ends with the error of its
	// context.
	listenCtx, stop := context.WithCancel(ctx)
	defer stop()
	var (
		wg        sync.WaitGroup
		requests  []string
		responses []string
	)
	requestEvents := chromedp.Events(listenCtx, network.RequestWillBeSent)
	responseEvents := chromedp.Events(listenCtx, network.ResponseReceived)
	wg.Go(func() {
		for ev, err := range requestEvents {
			if err != nil {
				return
			}
			requests = append(requests, fmt.Sprintf("%s %s", ev.Request.Method, strings.TrimPrefix(ev.Request.URL, host)))
		}
	})
	wg.Go(func() {
		for ev, err := range responseEvents {
			if err != nil {
				return
			}
			responses = append(responses, fmt.Sprintf("%d %s %s", ev.Response.Status, ev.Type, strings.TrimPrefix(ev.Response.URL, host)))
		}
	})

	// The browser sends the lifecycle events only when the program asks for
	// them. The event networkIdle comes when the page has had no request for
	// 500 ms. It waits for the late request of the page, and the event load
	// does not. WaitEvent subscribes, runs the navigation, and returns the first
	// event that the function accepts.
	if _, err := chromedp.Call(ctx, page.SetLifecycleEventsEnabled, page.SetLifecycleEventsEnabledParams{Enabled: true}); err != nil {
		return fmt.Errorf("enabling the lifecycle events: %w", err)
	}
	start := time.Now()
	idle := func(ev page.EventLifecycleEvent) bool { return ev.Name == "networkIdle" }
	if _, err := chromedp.Run(ctx, chromedp.WaitEvent(page.LifecycleEvent, idle, chromedp.Navigate(host+"/"))); err != nil {
		return fmt.Errorf("waiting for the network to be idle: %w", err)
	}
	fmt.Printf("the network was idle after %s\n", time.Since(start).Round(100*time.Millisecond))

	// Cancel the context of the network listeners, and wait for their
	// goroutines. Then the slices are safe to read.
	stop()
	wg.Wait()
	sort.Strings(requests)
	sort.Strings(responses)
	fmt.Printf("requests (%d):\n", len(requests))
	for _, r := range requests {
		fmt.Printf("  %s\n", r)
	}
	fmt.Printf("responses (%d):\n", len(responses))
	for _, r := range responses {
		fmt.Printf("  %s\n", r)
	}

	// The console iterator has buffered the messages. Read the first three,
	// and leave the loop with break. The break ends this subscription.
	fmt.Println("the first three console messages:")
	count := 0
	for ev, err := range console {
		if err != nil {
			return fmt.Errorf("reading the console: %w", err)
		}
		fmt.Printf("  console.%s: %s\n", ev.Type, arguments(ev.Args))
		if count++; count == 3 {
			break
		}
	}

	// Wait for one event that a click causes. The click logs two messages,
	// and the function accepts only the one that starts with "clicked".
	clicked := func(ev runtime.EventConsoleAPICalled) bool {
		return len(ev.Args) > 0 && strings.Contains(string(ev.Args[0].Value), "clicked")
	}
	ev, err := chromedp.Run(ctx, chromedp.WaitEvent(runtime.ConsoleAPICalled, clicked, chromedp.Click(chromedp.ID("button"))))
	if err != nil {
		return fmt.Errorf("waiting for the message of the click: %w", err)
	}
	fmt.Printf("the message of the click: console.%s: %s\n", ev.Type, arguments(ev.Args))
	return nil
}

// arguments joins the arguments of a console call. The value of a primitive
// argument is JSON, and an object has a description.
func arguments(args []*runtime.RemoteObject) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if a.Value != nil {
			parts[i] = string(a.Value)
		} else {
			parts[i] = a.Description
		}
	}
	return strings.Join(parts, " ")
}

// newMux returns the handlers of the test server. The page logs three
// messages, loads a script and an image, and sends a request 200 ms after the
// load. The endpoint /late answers after 100 ms.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The browser asks for /favicon.ico by itself, and the server has
		// no icon.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><head><title>eventsiter</title>
<script src="/app.js"></script></head><body>
<img src="/logo.svg" width="10" height="10">
<button id="button" onclick="console.log('noise'); console.log('clicked', 7)">click</button>
</body></html>`)
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `console.log("page started");
console.warn("careful");
console.error("broken", {code: 7});
window.addEventListener("load", () => {
  setTimeout(() => fetch("/late").then(() => console.log("late request done")), 200);
});`)
	})
	mux.HandleFunc("/logo.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="tomato"/></svg>`)
	})
	mux.HandleFunc("/late", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, "late")
	})
	return mux
}
