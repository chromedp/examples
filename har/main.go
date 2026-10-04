// Command har is a chromedp example demonstrating how to generate a HAR file
// from the network events of a page. It loads a local page that has an image, a
// script and a fetch request. Then it builds a HAR 1.2 document from the events
// of the network domain, writes it to the file named by -out and prints a
// summary. It starts a local server and needs no internet. A full HAR also
// needs the cookies, the request bodies, the redirect chains, the cache data,
// the DNS, connect and TLS timings, the sizes of the headers and the text of
// each response. This program leaves them out. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/har"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// requestCount is the number of requests that the page makes: the page, the
// image, the script and the fetch request of the script.
const requestCount = 4

func main() {
	out := flag.String("out", "out.har", "name of the HAR file to write")
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
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	if err := run(ctx, srv.URL, *out); err != nil {
		log.Fatal(err)
	}
}

// run records the network events of one page load, and writes the HAR file.
func run(ctx context.Context, pageURL, out string) error {
	// The first call with a context starts the browser and opens the tab, and
	// the browser stops when that context ends. Do it with ctx, before the
	// listeners use a context that stop cancels. Then read the version of the
	// browser, which the HAR file records.
	ver, err := chromedp.CallBrowser(ctx, browser.GetVersion, cdp.Empty{})
	if err != nil {
		return fmt.Errorf("reading the browser version: %w", err)
	}
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("opening the tab: %w", err)
	}

	// Start the listeners before the navigation. The iterator of
	// chromedp.Events subscribes when Events returns, so no event is lost,
	// even if the goroutine that reads it starts later.
	rec := newRecorder()
	listenCtx, stop := context.WithCancel(ctx)
	defer stop()
	var wg sync.WaitGroup
	listen(listenCtx, &wg, network.RequestWillBeSent, rec.requestSent)
	listen(listenCtx, &wg, network.ResponseReceived, rec.responseReceived)
	listen(listenCtx, &wg, network.LoadingFinished, rec.loadingFinished)
	listen(listenCtx, &wg, network.LoadingFailed, rec.loadingFailed)
	listen(listenCtx, &wg, page.DomContentEventFired, func(ev page.EventDomContentEventFired) {
		rec.setPageTiming(ev.Timestamp, false)
	})
	listen(listenCtx, &wg, page.LoadEventFired, func(ev page.EventLoadEventFired) {
		rec.setPageTiming(ev.Timestamp, true)
	})

	if err := chromedp.Do(ctx, chromedp.Navigate(pageURL)); err != nil {
		return fmt.Errorf("loading %s: %w", pageURL, err)
	}
	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		return fmt.Errorf("reading the title: %w", err)
	}

	// The load event fires before the script of the page sends its fetch
	// request, and the events arrive in goroutines. Wait until every request
	// has finished, then stop the listeners.
	if err := rec.waitFinished(ctx, requestCount); err != nil {
		return err
	}
	stop()
	wg.Wait()

	doc, err := rec.build(ctx, title, ver.Product)
	if err != nil {
		return err
	}

	// Write the document. The file is JSON with an indent, so that a person
	// can read it.
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the HAR document: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", out, err)
	}
	summarize(doc, out, len(data))
	return nil
}

// listen starts a goroutine that calls handle for each event, until the
// context ends. Events subscribes before listen returns.
func listen[E any](ctx context.Context, wg *sync.WaitGroup, ev cdp.Event[E], handle func(E)) {
	events := chromedp.Events(ctx, ev)
	wg.Go(func() {
		for e, err := range events {
			if err != nil {
				return
			}
			handle(e)
		}
	})
}

// call holds the events of one request. The events of one request arrive in
// order, but the goroutines of the listeners do not run in order. Each field
// can be nil until its event arrives.
type call struct {
	sent     *network.EventRequestWillBeSent
	received *network.EventResponseReceived
	finished *network.EventLoadingFinished
	failed   *network.EventLoadingFailed
}

// done is true when the request has ended, with or without an error.
func (c *call) done() bool {
	return c.sent != nil && (c.finished != nil || c.failed != nil)
}

// recorder collects the events of the listeners. It has a mutex because the
// listeners run in separate goroutines.
type recorder struct {
	mu    sync.Mutex
	calls map[network.RequestID]*call

	// The times of the events of the page, in monotonic seconds.
	domContent, load cdp.MonotonicTime
}

func newRecorder() *recorder {
	return &recorder{calls: make(map[network.RequestID]*call)}
}

// get returns the call of a request, and makes it when it is new. The caller
// must hold the mutex.
func (r *recorder) get(id network.RequestID) *call {
	c := r.calls[id]
	if c == nil {
		c = new(call)
		r.calls[id] = c
	}
	return c
}

func (r *recorder) requestSent(ev network.EventRequestWillBeSent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(ev.RequestID).sent = &ev
}

func (r *recorder) responseReceived(ev network.EventResponseReceived) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(ev.RequestID).received = &ev
}

func (r *recorder) loadingFinished(ev network.EventLoadingFinished) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(ev.RequestID).finished = &ev
}

func (r *recorder) loadingFailed(ev network.EventLoadingFailed) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(ev.RequestID).failed = &ev
}

func (r *recorder) setPageTiming(t cdp.MonotonicTime, isLoad bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if isLoad {
		r.load = t
	} else {
		r.domContent = t
	}
}

// waitFinished waits until n requests have ended.
func (r *recorder) waitFinished(ctx context.Context, n int) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		r.mu.Lock()
		ended := 0
		for _, c := range r.calls {
			if c.done() {
				ended++
			}
		}
		r.mu.Unlock()
		if ended >= n {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %d requests, %d ended: %w", n, ended, ctx.Err())
		case <-ticker.C:
		}
	}
}

// build makes the HAR document from the calls. The listeners must have stopped.
func (r *recorder) build(ctx context.Context, title, product string) (*har.HAR, error) {
	// Sort the calls by the time of the request.
	var ids []network.RequestID
	for id, c := range r.calls {
		if c.sent != nil {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b network.RequestID) int {
		return int(r.calls[a].sent.Timestamp.Float64()*1e6 - r.calls[b].sent.Timestamp.Float64()*1e6)
	})
	if len(ids) == 0 {
		return nil, fmt.Errorf("no request was recorded")
	}

	// The page starts with its first request. The times of the page are
	// the milliseconds from that start to the events.
	first := r.calls[ids[0]].sent
	pg := &har.Page{
		StartedDateTime: wallTime(first.WallTime),
		ID:              "page_1",
		Title:           title,
		PageTimings: &har.PageTimings{
			OnContentLoad: new(since(first.Timestamp, r.domContent)),
			OnLoad:        new(since(first.Timestamp, r.load)),
		},
	}

	doc := &har.HAR{Log: &har.Log{
		Version: "1.2",
		Creator: &har.Creator{Name: "chromedp har example", Version: "1.0"},
		Browser: &har.Creator{Name: "Chrome", Version: strings.TrimPrefix(product, "Chrome/")},
		Pages:   []*har.Page{pg},
		Entries: []*har.Entry{},
	}}
	for _, id := range ids {
		entry, err := r.entry(ctx, id, pg.ID)
		if err != nil {
			return nil, err
		}
		doc.Log.Entries = append(doc.Log.Entries, entry)
	}
	return doc, nil
}

// entry makes the HAR entry of one request.
func (r *recorder) entry(ctx context.Context, id network.RequestID, pageID string) (*har.Entry, error) {
	c := r.calls[id]
	req := &har.Request{
		Method:      c.sent.Request.Method,
		URL:         c.sent.Request.URL,
		HTTPVersion: "HTTP/1.1",
		Cookies:     []*har.Cookie{},
		Headers:     pairs(c.sent.Request.Headers),
		QueryString: []*har.NameValuePair{},
		HeadersSize: -1,
		BodySize:    -1,
	}
	// The end of the request is the time of the event that ended it.
	var end cdp.MonotonicTime
	resp := &har.Response{
		Cookies:     []*har.Cookie{},
		Headers:     []*har.NameValuePair{},
		Content:     &har.Content{},
		HeadersSize: -1,
		BodySize:    -1,
	}
	total := 0.0
	timings := &har.Timings{Blocked: new(-1.0), DNS: new(-1.0), Connect: new(-1.0), Ssl: new(-1.0)}
	switch {
	case c.failed != nil:
		// A failed request has no response. HAR uses the status 0.
		end = c.failed.Timestamp
		resp.Comment = c.failed.ErrorText
		total = since(c.sent.Timestamp, end)
		timings.Wait = total
	default:
		end = c.finished.Timestamp
		rr := c.received.Response
		resp.Status = rr.Status
		resp.StatusText = rr.StatusText
		resp.HTTPVersion = strings.ToUpper(rr.Protocol)
		resp.Headers = pairs(rr.Headers)
		resp.BodySize = int64(c.finished.EncodedDataLength)
		resp.Content.MimeType = rr.MimeType
		req.HTTPVersion = resp.HTTPVersion

		// The size of the content is the size after decoding. The command
		// Network.getResponseBody returns the decoded body.
		body, err := chromedp.Call(ctx, network.GetResponseBody, network.GetResponseBodyParams{RequestID: id})
		if err != nil {
			return nil, fmt.Errorf("reading the body of %s: %w", req.URL, err)
		}
		resp.Content.Size = int64(len(body.Body))

		// The timing of the response holds the milliseconds of each step,
		// from RequestTime. The time to receive the body is the rest.
		total = since(c.sent.Timestamp, end)
		if t := rr.Timing; t != nil {
			timings.Send = t.SendEnd - t.SendStart
			timings.Wait = t.ReceiveHeadersEnd - t.SendEnd
			headersAt := t.RequestTime*1000 + t.ReceiveHeadersEnd
			timings.Receive = max(0, end.Float64()*1000-headersAt)
		} else {
			timings.Wait = total
		}
	}
	return &har.Entry{
		Pageref:         pageID,
		StartedDateTime: wallTime(c.sent.WallTime),
		Time:            timings.Send + timings.Wait + timings.Receive,
		Request:         req,
		Response:        resp,
		Cache:           &har.Cache{},
		Timings:         timings,
	}, nil
}

// pairs converts the headers of the protocol to HAR pairs, sorted by name.
func pairs(h network.Headers) []*har.NameValuePair {
	list := []*har.NameValuePair{}
	for name, value := range h {
		list = append(list, &har.NameValuePair{Name: name, Value: fmt.Sprint(value)})
	}
	slices.SortFunc(list, func(a, b *har.NameValuePair) int { return strings.Compare(a.Name, b.Name) })
	return list
}

// wallTime formats a time in seconds since the epoch as a HAR date.
func wallTime(t cdp.TimeSinceEpoch) string {
	sec := int64(t.Float64())
	nsec := int64((t.Float64() - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// since returns the milliseconds from the start to the time t, or -1 when the
// event did not happen.
func since(start, t cdp.MonotonicTime) float64 {
	if t == 0 {
		return -1
	}
	return (t - start).Float64() * 1000
}

// path returns the path and the query of a URL.
func path(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.RequestURI()
}

// summarize prints the entries of the document.
func summarize(doc *har.HAR, out string, size int) {
	pg := doc.Log.Pages[0]
	fmt.Printf("wrote %s (%d bytes) with %d entries\n", out, size, len(doc.Log.Entries))
	fmt.Printf("page %q: DOMContentLoaded %.0f ms, load %.0f ms\n", pg.Title, *pg.PageTimings.OnContentLoad, *pg.PageTimings.OnLoad)
	for _, e := range doc.Log.Entries {
		fmt.Printf("%d %-4s %-24s %-24s content %5d bytes, %6.1f ms\n",
			e.Response.Status, e.Request.Method, path(e.Request.URL),
			e.Response.Content.MimeType, e.Response.Content.Size, e.Time)
	}
}

// newMux returns the handlers of the test server. The page holds an image and a
// script. The script sends a fetch request.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>HAR example</title><link rel="icon" href="data:,"></head><body>
<img src="/logo.png" width="32" height="32">
<p id="result">waiting</p>
<script src="/app.js"></script>
</body></html>`)
	})
	mux.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
			log.Printf("writing the image: %v", err)
		}
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `fetch("/api/data?id=1").then(r => r.json()).then(d => {
	document.getElementById("result").textContent = d.message;
});`)
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message":"hello from the server"}`)
	})
	return mux
}
