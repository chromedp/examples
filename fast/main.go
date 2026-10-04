// Command fast is a chromedp example demonstrating how to measure the speed of
// the internet connection and show the result in the terminal. It reads
// fast.com and needs a terminal that can show images. It is inspired by
// adhocore/fast, see https://github.com/adhocore/fast. Use -v to print the
// protocol messages, -visible to show the browser window and leave it open, and
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
//
// While the test runs, the program shows a progress bar for each direction. The
// bar has the time against the longest time of 30 seconds, and the speed that
// the page shows, in MiB/s. The bars do not show when the flag
// -visible-on-terminal is on, because the page shows the speed. At the end, the
// program prints the speeds, the latency, the client, the servers and the data
// that it transferred, as the page shows them.
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
	"github.com/kenshaw/rasterm"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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

// maxDuration is the longest time that the test of fast.com runs for one
// direction. The test has no fixed length. It ends when the readings are
// stable, so the bar shows the time against this maximum, and not a percentage
// of the work.
const maxDuration = 30 * time.Second

// reading is the speed that the page of fast.com shows for a test. The unit is
// Kbps, Mbps or Gbps, with the prefixes of the SI.
type reading struct {
	Value string `json:"value"`
	Units string `json:"units"`
}

// bytes returns the speed of the reading in bytes for each second. The page
// shows bits.
func (r reading) bytes() (float64, bool) {
	v, err := strconv.ParseFloat(r.Value, 64)
	if err != nil {
		return 0, false
	}
	switch {
	case strings.HasPrefix(r.Units, "K"):
		v *= 1e3
	case strings.HasPrefix(r.Units, "M"):
		v *= 1e6
	case strings.HasPrefix(r.Units, "G"):
		v *= 1e9
	}
	return v / 8, true
}

// scale divides v by base until it is smaller than base, and returns it with
// the prefix. The prefixes are the ones of the unit, for example "K", "M", "G".
func scale(v, base float64, prefixes []string) (float64, string) {
	prefix := ""
	for _, p := range prefixes {
		if v < base {
			break
		}
		v, prefix = v/base, p
	}
	return v, prefix
}

// iec returns the speed of the reading in bytes for each second, with the
// binary prefixes of the IEC, such as 51.5 MiB/s.
func (r reading) iec() string {
	v, ok := r.bytes()
	if !ok {
		return "- B/s"
	}
	v, p := scale(v, 1024, []string{"Ki", "Mi", "Gi"})
	return fmt.Sprintf("%.1f %sB/s", v, p)
}

// si returns the speed of the reading in bytes for each second, with the
// decimal prefixes of the SI, such as 54.0 MB/s.
func (r reading) si() string {
	v, ok := r.bytes()
	if !ok {
		return "- B/s"
	}
	v, p := scale(v, 1000, []string{"K", "M", "G"})
	return fmt.Sprintf("%.1f %sB/s", v, p)
}

// bar shows the progress of one test with a progress bar from mpb: the time
// against the maximum, and the speed that the page shows. Its zero value does
// nothing, so the program can call it when the stream draws the page.
type bar struct {
	bar   *mpb.Bar
	speed atomic.Value
}

// newBar adds a bar to the container.
func newBar(pb *mpb.Progress, name string) *bar {
	b := new(bar)
	b.speed.Store("")
	b.bar = pb.New(
		int64(maxDuration/tick),
		mpb.BarStyle(),
		mpb.PrependDecorators(decor.Name(name)),
		mpb.AppendDecorators(
			decor.Any(func(st decor.Statistics) string {
				return fmt.Sprintf("%4.1fs / %.0fs", float64(st.Current)*tick.Seconds(), maxDuration.Seconds())
			}),
			decor.Any(func(decor.Statistics) string {
				return fmt.Sprintf(" %-14s", b.speed.Load())
			}),
		),
	)
	return b
}

// tick is the interval between the updates of a bar.
const tick = 100 * time.Millisecond

// run updates the bar until ctx ends. The page has the speed in the element
// that the value names, and the unit in the element that the units names.
func (b *bar) run(ctx context.Context, value, units string) {
	t := time.NewTicker(tick)
	defer t.Stop()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			b.bar.SetCurrent(min(int64(now.Sub(start)/tick), int64(maxDuration/tick)-1))
			r, err := chromedp.Run(ctx, chromedp.Evaluate[reading](fmt.Sprintf(
				`({value: document.getElementById(%q).textContent.trim(), units: document.getElementById(%q).textContent.trim()})`, value, units)))
			if err == nil {
				b.speed.Store(r.iec())
			}
		}
	}
}

// summary is the result that the page of fast.com shows when the test ends.
type summary struct {
	Download, Upload  reading
	Latency, Loaded   reading
	Location, IP, ISP string
	Servers           string
	DownloadMB, UpMB  string
}

// summaryJS reads the elements of the result of the page. It reads text and
// does not need the elements to be visible.
const summaryJS = `(() => {
	const text = id => document.getElementById(id).textContent.trim();
	const reading = (value, units) => ({value: text(value), units: text(units)});
	return {
		download: reading("speed-value", "speed-units"),
		upload: reading("upload-value", "upload-units"),
		latency: reading("latency-value", "latency-units"),
		loaded: reading("bufferbloat-value", "bufferbloat-units"),
		location: text("user-location"),
		ip: text("user-ip"),
		isp: text("user-isp"),
		servers: text("server-locations"),
		downloadMB: text("down-mb-value"),
		upMB: text("up-mb-value"),
	};
})()`

// print writes the result in lines, with the speeds in the units of the IEC
// and the same speed with the decimal prefixes of the SI in brackets.
func (r *summary) print(w io.Writer, took time.Duration) {
	speed := func(v reading) string { return fmt.Sprintf("%s (%s)", v.iec(), v.si()) }
	fmt.Fprintf(w, "download     %s\n", speed(r.Download))
	fmt.Fprintf(w, "upload       %s\n", speed(r.Upload))
	fmt.Fprintf(w, "latency      %s %s unloaded, %s %s loaded\n", r.Latency.Value, r.Latency.Units, r.Loaded.Value, r.Loaded.Units)
	// The page puts spaces and a non-breaking space inside the text.
	var client []string
	for _, f := range []string{r.Location, r.IP, r.ISP} {
		if f = strings.Join(strings.Fields(f), " "); f != "" {
			client = append(client, f)
		}
	}
	fmt.Fprintf(w, "client       %s\n", strings.Join(client, ", "))
	fmt.Fprintf(w, "server(s)    %s\n", strings.Join(strings.Fields(r.Servers), " "))
	fmt.Fprintf(w, "transferred  %s MB down, %s MB up\n", r.DownloadMB, r.UpMB)
	fmt.Fprintf(w, "time         %v\n", took.Round(time.Second))
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
	// Show a bar for each test, when the standard error is a terminal and the
	// stream does not draw the page. The stream shows the speed of the page.
	var pb *mpb.Progress
	if info, err := os.Stderr.Stat(); s == nil && err == nil && info.Mode()&os.ModeCharDevice != 0 {
		pb = mpb.NewWithContext(ctx, mpb.WithWidth(48), mpb.WithOutput(os.Stderr), mpb.WithAutoRefresh())
		defer pb.Wait()
	}
	for _, step := range []struct {
		name, id, units string
		after           chromedp.Action[chromedp.Void]
	}{
		{"download ", "speed-value", "speed-units", chromedp.Click(`#show-more-details-link`)},
		{"upload   ", "upload-value", "upload-units", nil},
	} {
		var b *bar
		progress, stop := context.WithCancel(ctx)
		done := make(chan struct{})
		if pb != nil {
			b = newBar(pb, step.name)
			go func() {
				defer close(done)
				b.run(progress, step.id, step.units)
			}()
		} else {
			close(done)
		}
		st, err := waitResult(ctx, step.id)
		stop()
		<-done
		if b != nil {
			// A test that ends early completes its bar at the time that it took.
			if err != nil {
				b.bar.Abort(false)
			} else {
				b.bar.SetTotal(-1, true)
			}
			b.bar.Wait()
		}
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
	result, err := chromedp.Run(ctx, chromedp.Evaluate[*summary](summaryJS))
	if err != nil {
		return err
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

	// print the result of the test
	result.print(stdout, end.Sub(start))
	return nil
}
