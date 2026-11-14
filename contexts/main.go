// Command contexts is a chromedp example demonstrating how to see the
// JavaScript contexts that a tab already has, with chromedp.WithListener. When a
// program attaches to a tab that runs already, Chrome answers Runtime.enable
// with an event for each JavaScript context of the tab, and these events are
// the only list of the contexts that exist. The program loads a local page that
// holds two iframes in one tab. It attaches a second context to the tab with
// chromedp.WithTargetID and WithListener, and prints the contexts that the
// listener saw. A third context attaches the same way, but it calls
// chromedp.Listen after it attached, and it sees none of these events. At the
// end the program closes the tab and waits for chromedp.Detached to close,
// which is how a program learns that its tab is gone. It starts a local server
// and needs no internet. Use -v to print the protocol messages and -visible to
// show the browser window and leave it open. Use -visible-on-terminal to draw
// the page in the terminal with terminal graphics.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
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

// run opens a tab, attaches two more contexts to it, and prints what each one
// saw of the JavaScript contexts of the tab.
func run(ctx context.Context, out io.Writer, host string) error {
	// An empty Do starts the browser. The contexts below are its children, so
	// they must all use this one browser.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// The tab and its page. This context owns the tab, so canceling it
	// closes the tab.
	owner, cancelOwner := chromedp.NewContext(ctx)
	defer cancelOwner()
	if err := chromedp.Do(owner, chromedp.Navigate(host+"/"), chromedp.WaitVisible(chromedp.CSS("#ready"))); err != nil {
		return fmt.Errorf("loading %s: %w", host, err)
	}
	id := chromedp.FromContext(owner).Target.TargetID

	// WithTargetID attaches a new context to a tab that runs already, and it
	// opens no new tab. WithListener adds the func before the first command
	// of the context, so the func sees the answer to Runtime.enable. The
	// target calls the func before it returns the reply, so the func has all
	// of the events when Do returns.
	var early recorder
	attached, cancelAttached := chromedp.NewContext(ctx, chromedp.WithTargetID(id), chromedp.WithListener(early.handle))
	defer cancelAttached()
	if err := chromedp.Do(attached); err != nil {
		return fmt.Errorf("attaching with WithListener: %w", err)
	}

	// The same attach, but the listener starts after the first command. The
	// events of the start are gone by then.
	late, cancelLate := chromedp.NewContext(ctx, chromedp.WithTargetID(id))
	defer cancelLate()
	if err := chromedp.Do(late); err != nil {
		return fmt.Errorf("attaching to listen late: %w", err)
	}
	var after recorder
	if err := chromedp.Listen(late, after.handle); err != nil {
		return fmt.Errorf("listening after the attach: %w", err)
	}

	fmt.Fprintf(out, "WithListener saw %d JavaScript contexts of the tab:\n", len(early.lines(id)))
	for i, line := range early.lines(id) {
		fmt.Fprintf(out, "%2d  %s\n", i+1, line)
	}
	fmt.Fprintf(out, "Listen after the attach saw %d of them\n", len(after.lines(id)))

	// Detached returns a channel that closes when the browser detaches the
	// tab. It opens no tab, so it is safe to ask about a tab that is gone.
	// Closing the tab from the owner starts the detach.
	gone, err := chromedp.Detached(attached)
	if err != nil {
		return fmt.Errorf("asking for the detach: %w", err)
	}
	cancelOwner()
	select {
	case <-gone:
		fmt.Fprintln(out, "the browser detached the tab, and Detached closed")
	case <-time.After(10 * time.Second):
		return fmt.Errorf("waiting for the detach: the channel stayed open")
	case <-ctx.Done():
		return fmt.Errorf("waiting for the detach: %w", ctx.Err())
	}
	return nil
}

// recorder keeps the JavaScript contexts that a listener saw. The listener runs
// on the goroutine of the tab, and the program reads from another one.
type recorder struct {
	mu       sync.Mutex
	contexts []*runtime.EventExecutionContextCreated
}

// handle keeps the events that announce a JavaScript context, and returns at
// once as the rules of Listen require.
func (r *recorder) handle(ev any) {
	if ev, ok := ev.(*runtime.EventExecutionContextCreated); ok {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.contexts = append(r.contexts, ev)
	}
}

// lines describes the default context of each frame, the top frame first. The
// top frame has the ID of the tab. The order of the iframes is not fixed, and
// the lines do not tell them apart.
func (r *recorder) lines(tab target.ID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var lines []string
	for _, ev := range r.contexts {
		var aux struct {
			FrameID   string `json:"frameId"`
			IsDefault bool   `json:"isDefault"`
		}
		if err := json.Unmarshal(ev.Context.AuxData, &aux); err != nil || !aux.IsDefault {
			continue
		}
		line := "default context of an iframe"
		if aux.FrameID == string(tab) {
			line = "default context of the top frame"
		}
		lines = append(lines, line)
	}
	// "default context of an iframe" sorts before "default context of the top
	// frame", so reverse the order to put the top frame first.
	slices.Sort(lines)
	slices.Reverse(lines)
	return lines
}

// newMux returns the handlers of the test server. The page holds two iframes
// of the same site, so the tab has three JavaScript contexts and no second
// process.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>contexts</title><link rel="icon" href="data:,"></head><body>
<h1 id="ready">contexts</h1>
<iframe src="/child?a" width="200" height="60"></iframe>
<iframe src="/child?b" width="200" height="60"></iframe>
</body></html>`)
	})
	mux.HandleFunc("/child", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body><p>an iframe</p></body></html>`)
	})
	return mux
}
