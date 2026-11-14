// Command timeline is a chromedp example demonstrating how to see the events of
// a page of every type in the order in which Chrome sent them. chromedp.Listen
// calls one func for each event of the tab, before the next message is read.
// The program loads a local page that logs to the console, loads an image and a
// script, and sends a request after the load event. It puts the navigation, the
// network, the console and the load events in one timeline. Then it runs a
// command that logs to the console, and it checks that the timeline already
// holds that console call when the command returns. The iterators of
// chromedp.Events cannot give this, because each one has its own queue and
// goroutine. It starts a local server and needs no internet. Use -v to print
// the protocol messages and -visible to show the browser window and leave it
// open. Use -visible-on-terminal to draw the page in the terminal with terminal
// graphics.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/termstream"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	tc := termstream.Register(flag.CommandLine)
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
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	// draw the page in the terminal when the user asks for it. The stream
	// starts the browser, so it starts before the first navigation
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	out := io.Writer(os.Stdout)
	if s != nil {
		out = s.LogWriter()
	}

	if err := run(ctx, out, srv.URL); err != nil {
		termstream.Fatal(s, err)
	}
}

// run loads the page, prints the timeline of its events, and checks that a
// command sees the console call that it causes.
func run(ctx context.Context, out io.Writer, host string) error {
	// An empty Do starts the browser and opens the tab, so that the listener
	// starts on a tab that exists.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// Start the listener before the navigation. The func runs in the loop that
	// reads the messages of the tab. It stores a line and returns, and it runs
	// no command.
	tl := newTimeline(host)
	if err := chromedp.Listen(ctx, tl.handle); err != nil {
		return fmt.Errorf("listening to the events: %w", err)
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/")); err != nil {
		return fmt.Errorf("loading %s: %w", host, err)
	}

	// The script of the page sends a request after the load event. Wait for
	// its response.
	select {
	case <-tl.fetched:
	case <-ctx.Done():
		return fmt.Errorf("waiting for the late request: %w", ctx.Err())
	}
	fmt.Fprintln(out, "the timeline of the page load, in the order of arrival:")
	for i, line := range tl.lines() {
		fmt.Fprintf(out, "%2d  %s\n", i+1, line)
	}

	// The console call of a command arrives before the reply to the command.
	// The listener sees an event before the command that follows it returns,
	// so the line is in the timeline now, with no wait.
	const text = "from the command"
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[any](`console.log("`+text+`"), 1`)); err != nil {
		return fmt.Errorf("running the command: %w", err)
	}
	want := "console.log: " + text
	if !tl.has(want) {
		return errors.New("the timeline has no console call when the command returns")
	}
	fmt.Fprintf(out, "\nthe command returned, and the timeline already had %q\n", want)
	return nil
}

// timeline keeps one line for each event that it knows.
type timeline struct {
	host string

	mu  sync.Mutex
	log []string

	// fetched closes when the response to the late request arrives.
	fetched chan struct{}
}

func newTimeline(host string) *timeline {
	return &timeline{host: host, fetched: make(chan struct{})}
}

// handle adds a line for an event. The events of every type come in the order
// in which Chrome sent them.
func (t *timeline) handle(ev any) {
	switch ev := ev.(type) {
	case *page.EventFrameNavigated:
		if ev.Frame.ParentID == "" {
			t.add("frame navigated: " + t.path(ev.Frame.URL))
		}
	case *network.EventRequestWillBeSent:
		t.add(fmt.Sprintf("request %s %s", ev.Request.Method, t.path(ev.Request.URL)))
	case *network.EventResponseReceived:
		t.add(fmt.Sprintf("response %d %s", ev.Response.Status, t.path(ev.Response.URL)))
		if t.path(ev.Response.URL) == "/api/data" {
			close(t.fetched)
		}
	case *runtime.EventConsoleAPICalled:
		t.add(fmt.Sprintf("console.%s: %s", ev.Type, consoleText(ev)))
	case *page.EventDomContentEventFired:
		t.add("DOMContentLoaded")
	case *page.EventLoadEventFired:
		t.add("load")
	}
}

// add appends a line to the timeline.
func (t *timeline) add(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.log = append(t.log, line)
}

// lines returns a copy of the lines.
func (t *timeline) lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.log...)
}

// has reports whether the timeline holds the line.
func (t *timeline) has(line string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, l := range t.log {
		if l == line {
			return true
		}
	}
	return false
}

// path returns a URL without the address of the test server.
func (t *timeline) path(urlstr string) string {
	return strings.TrimPrefix(urlstr, t.host)
}

// consoleText formats the arguments of a console call as text.
func consoleText(ev *runtime.EventConsoleAPICalled) string {
	var parts []string
	for _, arg := range ev.Args {
		text := arg.Description
		if len(arg.Value) > 0 {
			text = string(arg.Value)
			if s, err := strconv.Unquote(text); err == nil {
				text = s
			}
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

// newMux returns the handlers of the test server. The page logs to the console
// and holds an image and a script. The script logs and sends a request after the
// load event.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>timeline</title><link rel="icon" href="data:,"></head><body>
<script>console.log("inline script");</script>
<img src="/dot.gif" width="32" height="32">
<script src="/app.js"></script>
</body></html>`)
	})
	mux.HandleFunc("/dot.gif", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		// a gif of one pixel
		fmt.Fprint(w, "GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `console.warn("external script");
window.addEventListener("load", () => setTimeout(() => fetch("/api/data"), 50));`)
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":"hello from the server"}`)
	})
	return mux
}
