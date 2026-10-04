// Command console is a chromedp example demonstrating how to read the console
// of a page and its uncaught exceptions with chromedp.Console. The program
// loads a local page that calls console.log, console.warn and console.error
// with strings and objects. After a click, the page throws an exception that
// nothing catches, rejects a promise that nothing handles, and loads an image
// that does not exist, which the browser reports as an error of the network. The
// program prints every message in order, with its type, its text, its place
// (URL, line and column) and the first lines of the stack trace of an exception.
// Then it shows how to read only some types, and how to stop reading with a
// break and with a canceled context. It starts a local server and needs no
// internet. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// endMarker is the text of the last message that the page writes. The program
// stops to read when it sees it.
const endMarker = "end of the run"

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

// run reads the console in three ways.
func run(ctx context.Context, host string) error {
	// An empty Do starts the browser. The browser sends the console messages
	// only from the moment that the tab exists, and Console starts the
	// browser if it is not running. Starting it first makes the order clear.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// Part 1: read everything. Console subscribes when it returns, and not
	// when the loop starts, so call it before the actions. The iterator keeps
	// the messages until the loop reads them.
	messages := chromedp.Console(ctx)
	if err := chromedp.Do(ctx,
		chromedp.Navigate(host+"/"),
		chromedp.Click(chromedp.ID("run")),
		chromedp.Click(chromedp.ID("missing")),
	); err != nil {
		return fmt.Errorf("running the page: %w", err)
	}
	fmt.Println("all the messages:")
	for m, err := range messages {
		// The iterator yields an error when the subscription ends. For
		// example, the context ends, or the browser cannot start.
		if err != nil {
			return fmt.Errorf("reading the console: %w", err)
		}
		show(m, host)
		// A page never closes its console, so the loop needs an end. The page
		// writes a marker last. The break ends the subscription, and a message
		// after the break is lost. Call Console again before the next action
		// to read the next messages.
		if strings.Contains(m.Text, endMarker) {
			break
		}
	}

	// Part 2: read only some types. A message has a Type, so a plain if skips
	// the others. IsException is true for an uncaught exception and for a
	// promise that nothing handles. Both have the type ConsoleException.
	messages = chromedp.Console(ctx)
	if err := chromedp.Do(ctx,
		chromedp.Click(chromedp.ID("run")),
		chromedp.Click(chromedp.ID("missing")),
	); err != nil {
		return fmt.Errorf("running the page again: %w", err)
	}
	fmt.Println("only the errors and the exceptions:")
	for m, err := range messages {
		if err != nil {
			return fmt.Errorf("reading the console: %w", err)
		}
		if strings.Contains(m.Text, endMarker) {
			break
		}
		if m.Type != chromedp.ConsoleError && !m.IsException() {
			continue
		}
		show(m, host)
	}

	// Part 3: stop with a context. The page is quiet now, so the loop will
	// wait without end. A derived context with a time limit ends the loop with
	// its error. It does not close the tab, because only a context made by
	// NewContext does that.
	quiet, stop := context.WithTimeout(ctx, 500*time.Millisecond)
	defer stop()
	fmt.Println("waiting for a message from a quiet page:")
	for m, err := range chromedp.Console(quiet) {
		if err != nil {
			fmt.Printf("  stopped: %v\n", err)
			break
		}
		show(m, host)
	}
	return nil
}

// show prints one message. The first line has the type and the text. The next
// line has the place, and an exception has the first lines of its stack.
//
// chromedp.Events(ctx, runtime.ConsoleAPICalled) gives only the calls of the
// console API. Their arguments are protocol objects, and the program must
// turn them into text itself. The uncaught exceptions come from a second
// iterator for runtime.ExceptionThrown, and the entries of the browser log, such
// as the failed image, come from a third one for log.EntryAdded. Three iterators
// give three queues, so the program cannot tell the order of a call and an
// exception. Console gives one queue with the text of the DevTools console,
// and it keeps the order.
func show(m chromedp.ConsoleMessage, host string) {
	source := ""
	if m.Source != "" {
		source = " [" + m.Source + "]"
	}
	fmt.Printf("  %-9s %s%s\n", m.Type, m.Text, source)

	// The line and the column start at 0 in the protocol, so add 1 to get the
	// numbers of an editor. The place can be empty. An entry of the browser
	// log, such as a failed image, has the URL of the resource and no line.
	switch {
	case m.Source != "":
		fmt.Printf("            for %s\n", strings.TrimPrefix(m.URL, host))
	case m.URL != "":
		fmt.Printf("            at %s:%d:%d\n", strings.TrimPrefix(m.URL, host), m.Line+1, m.Column+1)
	}

	// The stack of an exception tells which calls led to it.
	if m.IsException() && m.Stack != nil {
		for i, f := range m.Stack.CallFrames {
			if i == 3 {
				break
			}
			name := f.FunctionName
			if name == "" {
				name = "(anonymous)"
			}
			fmt.Printf("            stack: %s (%s:%d:%d)\n", name, strings.TrimPrefix(f.URL, host), f.LineNumber+1, f.ColumnNumber+1)
		}
	}
}

// newMux returns the handlers of the test server. The page / calls the console
// when it loads. The script /app.js has the code, so that the messages have a
// URL that differs from the URL of the page. The image /missing.png does not
// exist.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<html><head><title>console</title>
<script src="/app.js"></script></head><body>
<button id="run" onclick="run()">run</button>
<button id="missing" onclick="loadMissing()">missing</button>
</body></html>`)
	})
	// The browser asks for /favicon.ico by itself. Answer with no content, so
	// that the console does not get an error that the page did not cause.
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		io.WriteString(w, appJS)
	})
	return mux
}

// appJS is the script of the page.
const appJS = `console.log("page loaded", "version", 3);
console.log("user", {name: "Ada", age: 36});
console.warn("careful:", [1, 2, 3]);
console.error("saving %s failed with code %d", "draft", 7);

function level2() { throw new Error("boom"); }
function level1() { level2(); }

// A click handler that throws is an uncaught exception too, but a timer shows
// a stack that does not start in the event system.
function run() {
  setTimeout(level1, 0);
  setTimeout(() => Promise.reject(new Error("nobody handles this")), 10);
}

var count = 0;
function loadMissing() {
  const img = new Image();
  // The end marker comes after the browser has reported the failed request.
  img.onerror = () => setTimeout(() => console.log("end of the run"), 200);
  img.src = "/missing.png?n=" + (++count);
  document.body.appendChild(img);
}
`
