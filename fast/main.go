// Command fast is a chromedp example demonstrating how to measure the speed of
// the internet connection and show the result in the terminal. It reads
// fast.com and needs a terminal that can show images. It is inspired by
// adhocore/fast, see https://github.com/adhocore/fast. Use -v to print the
// protocol messages, -visible to show the browser window and leave it open, and
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
//
// When the page of fast.com says that it cannot reach its servers, the program
// stops at once and prints the message of the page and the requests that
// failed, such as a host that the network blocks. It does not wait for the
// time limit.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
	"github.com/kenshaw/rasterm"
)

// ErrNoConnection is the error for a page that cannot reach the servers of
// fast.com.
var ErrNoConnection = errors.New("fast.com cannot reach its servers")

// state is what the page of fast.com shows for a test.
type state struct {
	// Result is "succeeded" or "failed".
	Result string `json:"result"`
	// NoConnection is true when the page shows that it cannot reach its servers.
	NoConnection bool `json:"noConnection"`
	// Unstable is true when the page says that the network is unstable.
	Unstable bool `json:"unstable"`
	// Message is the text of the message that the page shows.
	Message string `json:"message"`
}

// stateJS reads the state of the element with the id that the argument names.
// It returns false until the test succeeded or failed, so that Poll keeps
// waiting. The page shows a message with a style, and not with a class.
const stateJS = `(() => {
	const shown = id => {
		const e = document.getElementById(id);
		return e && getComputedStyle(e).display !== "none" ? e.textContent.trim().replace(/^\*\s*/, "") : "";
	};
	const e = document.getElementById("%s");
	const result = ["succeeded", "failed"].find(c => e && e.classList.contains(c));
	if (!result) {
		return false;
	}
	const noConnection = shown("error-results-msg");
	const unstable = shown("unstable-results-msg");
	return {result, noConnection: noConnection !== "", unstable: unstable !== "", message: noConnection || unstable};
})()`

// waitResult waits until the test of an element succeeded or failed. It
// returns ErrNoConnection, with the message of the page, when the page cannot
// reach its servers.
func waitResult(ctx context.Context, id string) (*state, error) {
	// Poll runs the predicate in the page until it returns a value that is
	// not false. The default time limit of 30 seconds does not fit a speed
	// test, so the option removes it. The context sets the time limit.
	st, err := chromedp.Run(ctx, chromedp.Poll[*state](fmt.Sprintf(stateJS, id), chromedp.WithPollingTimeout(0)))
	if err != nil {
		return nil, err
	}
	if st.NoConnection {
		return st, fmt.Errorf("%w: %s", ErrNoConnection, st.Message)
	}
	return st, nil
}

// failedRequests collects the requests of the page that failed. Wait returns a
// description of each one, one for each host and error.
type failedRequests struct {
	mu   sync.Mutex
	urls map[network.RequestID]string
	seen map[string]bool
	wg   sync.WaitGroup
}

// watchFailures starts the collection. The subscriptions start when it
// returns, so call it before the navigation. The collection ends with ctx.
func watchFailures(ctx context.Context) *failedRequests {
	f := &failedRequests{urls: make(map[network.RequestID]string), seen: make(map[string]bool)}
	requests := chromedp.Events(ctx, network.RequestWillBeSent)
	failures := chromedp.Events(ctx, network.LoadingFailed)
	f.wg.Go(func() {
		for ev, err := range requests {
			if err != nil {
				return
			}
			f.mu.Lock()
			f.urls[ev.RequestID] = ev.Request.URL
			f.mu.Unlock()
		}
	})
	f.wg.Go(func() {
		for ev, err := range failures {
			if err != nil {
				return
			}
			if ev.Canceled {
				continue
			}
			f.mu.Lock()
			host := f.urls[ev.RequestID]
			if u, err := url.Parse(host); err == nil && u.Host != "" {
				host = u.Host
			}
			f.seen[host+": "+strings.TrimPrefix(ev.ErrorText, "net::")] = true
			f.mu.Unlock()
		}
	})
	return f
}

// Wait ends the collection and returns the sorted descriptions.
func (f *failedRequests) Wait(cancel context.CancelFunc) []string {
	cancel()
	f.wg.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for d := range f.seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func main() {
	var tc termcast.Flags
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	timeout := flag.Duration("timeout", 2*time.Minute, "time limit of the program")
	scale := flag.Float64("scale", 1.5, "scale of the screenshot")
	padding := flag.Int("padding", 0, "white space around the image, in pixels")
	out := flag.String("out", "", "file to write the screenshot to")
	tc.Register(flag.CommandLine)
	flag.Parse()
	if err := run(context.Background(), &tc, *verbose, *visible, *timeout, *scale, *padding, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, tc *termcast.Flags, verbose, visible bool, timeout time.Duration, scale float64, padding int, out string) error {
	// create context
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()
	if visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// draw the page in the terminal if the flag -visible-on-terminal is set
	s, err := tc.Start(ctx, verbose)
	if err != nil {
		return err
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	stdout := io.Writer(os.Stdout)
	if s != nil {
		stdout = s.LogWriter()
	}

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// collect the requests that fail, so that the error can say why
	watchCtx, stopWatching := context.WithCancel(ctx)
	defer stopWatching()
	failures := watchFailures(watchCtx)

	start := time.Now()

	// run the speed test, and capture the result. The page shows the result of
	// the download first, and then the result of the upload.
	if err := chromedp.Do(ctx, chromedp.Navigate(`https://fast.com`)); err != nil {
		return err
	}
	for _, step := range []struct {
		id    string
		after chromedp.Action[chromedp.Void]
	}{
		{"speed-value", chromedp.Click(`#show-more-details-link`)},
		{"upload-value", nil},
	} {
		st, err := waitResult(ctx, step.id)
		if err != nil {
			s.Stop()
			for _, f := range failures.Wait(stopWatching) {
				fmt.Fprintf(os.Stderr, "failed request: %s\n", f)
			}
			return err
		}
		if st.Unstable {
			fmt.Fprintf(stdout, "warning: %s\n", st.Message)
		}
		if step.after != nil {
			if err := chromedp.Do(ctx, step.after); err != nil {
				return err
			}
		}
	}
	buf, err := chromedp.Run(ctx, chromedp.ScreenshotScale(`.speed-controls-container`, scale))
	if err != nil {
		return err
	}

	end := time.Now()

	// decode the PNG
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}

	// add white padding around the image
	if padding != 0 {
		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		dst := image.NewRGBA(image.Rect(0, 0, w+2*padding, h+2*padding))
		for x := 0; x < w+2*padding; x++ {
			for y := 0; y < h+2*padding; y++ {
				dst.Set(x, y, color.White)
			}
		}
		draw.Draw(dst, dst.Bounds(), img, image.Pt(-padding, -padding), draw.Src)
		img = dst
	}

	// write the screenshot to disk if the flag -out is set
	if out != "" {
		if err := os.WriteFile(out, buf, 0o644); err != nil {
			return err
		}
	}

	// show the image in the terminal, after the stream has stopped
	s.Stop()
	if err := rasterm.Encode(os.Stdout, img); err != nil {
		return err
	}

	// print the time of the test
	_, err = fmt.Fprintf(stdout, "time: %v\n", end.Sub(start))
	return err
}
