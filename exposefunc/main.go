// Command exposefunc is a chromedp example demonstrating how to call Go
// functions from a page with chromedp.ExposeFunc. The action makes a Go
// function available as window.<name>, and a script of the page calls it like
// an async function that returns a promise. The program exposes four
// functions. The first gets an object and returns an object, with a lookup in
// a Go map. The second gets several arguments and returns a number. The third
// returns a Go error, so the promise rejects and the script catches it. The
// fourth is slow, and the page calls it five times at the same time. The
// program then navigates to a page with an iframe, and shows that the
// functions are still there, in the page and in the iframe. For each call it
// prints what the page got. It starts a local server and needs no internet.
// Use -v to print the protocol messages and -visible to show the browser window
// and leave it open.
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
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// query is the object that the page sends to getUser.
type query struct {
	ID string `json:"id"`
}

// user is the object that getUser sends back. The tags are the names of the
// properties in the page.
type user struct {
	Name  string   `json:"name"`
	City  string   `json:"city"`
	Langs []string `json:"langs"`
}

// users is the data that getUser looks up.
var users = map[string]user{
	"u1": {Name: "Ada", City: "London", Langs: []string{"go", "javascript"}},
	"u2": {Name: "Budi", City: "Jakarta", Langs: []string{"go"}},
}

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

// run exposes the functions, and calls them from the page.
func run(ctx context.Context, host string) error {
	// running and peak count the calls of slowSquare that run at the same
	// time. ExposeFunc runs the function in a new goroutine for each call, so
	// the function must be safe for that. The counters are atomic for it.
	var running, peak atomic.Int32

	// Expose the functions before the navigation. The action adds a script
	// that runs in every new document, so the functions are there when the
	// scripts of the page start. The type of the argument decides how the
	// call works. A struct takes one object. A slice takes all the arguments.
	// The first call on a context starts the browser, and ExposeFunc is the
	// first call here.
	if err := chromedp.Do(ctx,
		chromedp.ExposeFunc("getUser", func(ctx context.Context, q query) (user, error) {
			u, ok := users[q.ID]
			if !ok {
				return user{}, fmt.Errorf("no user with the id %q", q.ID)
			}
			return u, nil
		}),
		chromedp.ExposeFunc("sum", func(ctx context.Context, args []float64) (float64, error) {
			var total float64
			for _, a := range args {
				total += a
			}
			return total, nil
		}),
		chromedp.ExposeFunc("divide", func(ctx context.Context, args []float64) (float64, error) {
			if len(args) != 2 {
				return 0, fmt.Errorf("divide needs 2 arguments, got %d", len(args))
			}
			if args[1] == 0 {
				return 0, errors.New("division by zero")
			}
			return args[0] / args[1], nil
		}),
		chromedp.ExposeFunc("slowSquare", func(ctx context.Context, n float64) (float64, error) {
			cur := running.Add(1)
			defer running.Add(-1)
			for {
				old := peak.Load()
				if cur <= old || peak.CompareAndSwap(old, cur) {
					break
				}
			}
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
			return n * n, nil
		}),
		chromedp.Navigate(host+"/first"),
	); err != nil {
		return fmt.Errorf("exposing the functions: %w", err)
	}

	// The scripts below are async functions. Evaluate does not wait for a
	// promise, so the option AwaitPromise makes the browser wait for it. The
	// field is a pointer, because false is a value.
	awaitPromise := func(p *runtime.EvaluateParams) { p.AwaitPromise = new(true) }
	eval := func(what, script string) error {
		got, err := chromedp.Run(ctx, chromedp.Evaluate[string](script, awaitPromise))
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		fmt.Printf("%s: %s\n", what, got)
		return nil
	}

	// An object goes in, and an object comes out. The page sees a plain
	// object with the fields of the struct.
	if err := eval("an object in and out", `(async () => {
		const u = await getUser({id: "u1"});
		return u.name + " lives in " + u.city + " and knows " + u.langs.join(" and ");
	})()`); err != nil {
		return err
	}

	// Several arguments arrive in the slice. A call with no argument gives an
	// empty slice.
	if err := eval("several arguments", `(async () => {
		return "sum(1, 2, 3.5) = " + await sum(1, 2, 3.5) + ", sum() = " + await sum();
	})()`); err != nil {
		return err
	}

	// A Go error rejects the promise. The page gets an Error with the text of
	// the Go error, and a try and catch handles it as it handles any rejection.
	if err := eval("a Go error", `(async () => {
		const ok = await divide(10, 4);
		try {
			await divide(1, 0);
		} catch (e) {
			return "divide(10, 4) = " + ok + ", divide(1, 0) rejected with " + (e instanceof Error) + " " + JSON.stringify(e.message);
		}
		return "the promise did not reject";
	})()`); err != nil {
		return err
	}
	if err := eval("an unknown id", `(async () => {
		try {
			await getUser({id: "u9"});
		} catch (e) {
			return e.message;
		}
		return "the promise did not reject";
	})()`); err != nil {
		return err
	}

	// The page can call the same function many times at once. The five calls
	// of 300 ms take about 300 ms in all, and the program counted how many ran
	// together.
	if err := eval("five calls at the same time", `(async () => {
		const start = performance.now();
		const squares = await Promise.all([1, 2, 3, 4, 5].map(n => slowSquare(n)));
		const ms = performance.now() - start;
		return "squares " + squares.join(", ") + ", faster than one after the other: " + (ms < 1000);
	})()`); err != nil {
		return err
	}
	fmt.Printf("the largest number of calls that ran together in Go: %d\n", peak.Load())

	// A navigation destroys the old document, and the new one gets the
	// functions again, because the action runs its script in each new document.
	// The page also has an iframe, and the script of the iframe calls getUser
	// when the iframe loads. It posts what it got to its parent. The first
	// script of the parent below waits for that message.
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/second")); err != nil {
		return fmt.Errorf("navigating to the second page: %w", err)
	}
	if err := eval("after the navigation", `(async () => {
		return document.title + ": sum(4, 5) = " + await sum(4, 5);
	})()`); err != nil {
		return err
	}
	return eval("in the iframe", `window.fromChild`)
}

// newMux returns the handlers of the test server. The page /second has an
// iframe that loads /child, and it keeps the message of the iframe in the
// promise window.fromChild.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/first", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><head><title>first page</title></head><body>first</body></html>`)
	})
	mux.HandleFunc("/second", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><head><title>second page</title>
<script>
window.fromChild = new Promise(resolve => addEventListener("message", e => resolve(e.data)));
</script></head><body><iframe src="/child"></iframe></body></html>`)
	})
	mux.HandleFunc("/child", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<html><body><script>
(async () => {
  const u = await getUser({id: "u2"});
  parent.postMessage("the iframe got " + u.name + " from " + u.city + ", and its sum(1, 1) is " + await sum(1, 1), "*");
})();
</script></body></html>`)
	})
	return mux
}
