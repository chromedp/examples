// Command remote is a chromedp example demonstrating how to connect to an
// existing Chrome DevTools instance using a remote WebSocket URL. The flag -url
// names the browser and the flag -nav names the page to read. By default the
// program starts a local server and reads its home page, so it needs no
// internet. The program needs a terminal that can show images. The browser must
// run before the program starts, with a debugging port. Start it with
// "google-chrome --remote-debugging-port=9222", or let the program start a
// headless Chrome with the flag -start. See README.md. Use -v to print the
// protocol messages. Use -visible-on-terminal to draw the page in the terminal
// with terminal graphics. The program has no -visible flag, because it uses a
// browser that is already running. The browser must reach the page, so a
// browser in a container needs the flag -nav with a page that it can reach.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
	"github.com/kenshaw/rasterm"
)

// defaultURL is the WebSocket URL that Chrome uses with the flag
// --remote-debugging-port=9222.
const defaultURL = "ws://127.0.0.1:9222"

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	urlstr := flag.String("url", defaultURL, "WebSocket URL of the running browser")
	nav := flag.String("nav", "", "URL of the page to read (default: the home page of the local test site, and the program starts the site)")
	d := flag.Duration("d", 0, "extra time to wait after the page loads, for a page that builds itself late")
	start := flag.Bool("start", false, "start a headless Chrome with a debugging port, and connect to it (not with -url)")
	timeout := flag.Duration("timeout", 30*time.Second, "time limit of the program")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()
	urlSet := false
	flag.Visit(func(f *flag.Flag) { urlSet = urlSet || f.Name == "url" })
	if *start && urlSet {
		fmt.Fprintln(os.Stderr, "error: use the flag -start or the flag -url, not both")
		os.Exit(1)
	}
	if err := run(context.Background(), &tc, *verbose, *urlstr, *nav, *d, *start, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, tc *termcast.Flags, verbose bool, urlstr, nav string, d time.Duration, start bool, timeout time.Duration) error {
	if urlstr == "" {
		return errors.New("invalid remote devtools url")
	}

	// create a timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// if the flag -start is set, start a local headless Chrome
	if start {
		wsURL, stop, err := startChrome(ctx)
		if err != nil {
			return fmt.Errorf("starting a headless chrome: %w", err)
		}
		defer stop()
		urlstr = wsURL
	}

	// start the local test site, unless the flag -nav names a page
	if nav == "" {
		site := testsite.New()
		defer site.Close()
		nav = site.URL + "/"
	}

	// create an allocator context, for the browser context below
	allocatorContext, cancel := remote.NewAllocator(ctx, urlstr)
	defer cancel()

	// build the context options
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}

	// create context
	ctx, cancel = chromedp.NewContext(allocatorContext, opts...)
	defer cancel()

	// Start the stream before the first navigation, so that the frames show
	// the page while it loads. It connects to the browser. While it runs, it
	// holds what the program writes to out.
	s, err := tc.Start(ctx, verbose)
	if err != nil {
		return err
	}
	defer s.Stop()
	out := io.Writer(os.Stdout)
	if s != nil {
		out = s.LogWriter()
	}

	// run the actions. The first action connects to the browser, so a
	// connection error shows up here.
	if err := chromedp.Do(ctx, chromedp.Navigate(nav)); err != nil {
		return fmt.Errorf("connecting to the browser at %s and navigating to %s: %w\n\n%s", urlstr, nav, err, startHelp)
	}
	// Wait until the body has a node, and wait the extra time if the flag -d
	// asks for it.
	if err := chromedp.Do(ctx, chromedp.Query(`body`, chromedp.NodeVisible)); err != nil {
		return fmt.Errorf("waiting for the page %s: %w", nav, err)
	}
	if d > 0 {
		if err := chromedp.Do(ctx, chromedp.Sleep(d)); err != nil {
			return fmt.Errorf("waiting after the page loads: %w", err)
		}
	}

	// read the page
	title, err := chromedp.Run(ctx, chromedp.Title())
	if err != nil {
		return fmt.Errorf("reading the title of %s: %w", nav, err)
	}
	heading, err := chromedp.Run(ctx, chromedp.Text(`h1`))
	if err != nil {
		return fmt.Errorf("reading the heading of %s: %w", nav, err)
	}
	links, err := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('a[href]').length`))
	if err != nil {
		return fmt.Errorf("counting the links of %s: %w", nav, err)
	}
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return fmt.Errorf("taking a screenshot of %s: %w", nav, err)
	}
	fmt.Fprintf(out, "Page %s\n", nav)
	fmt.Fprintf(out, "  title: %s\n  heading: %s\n  links: %d\n", title, strings.TrimSpace(heading), links)
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	// stop the stream before the image, because the stream clears the terminal
	s.Stop()
	return rasterm.Encode(os.Stdout, img)
}

// startHelp tells a person how to start a browser that the program can use.
const startHelp = `start a browser with a debugging port, for example:
  google-chrome --remote-debugging-port=9222
or run the program with the flag -start, which starts a headless Chrome`

// chromeNames are the names of the browser programs that startChrome looks for,
// in this order.
var chromeNames = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "headless-shell"}

// startChrome starts a headless Chrome with a debugging port that the system
// picks. It returns the WebSocket URL of the browser, and a function that stops
// the browser and removes its profile directory.
func startChrome(ctx context.Context) (string, func(), error) {
	var path string
	for _, name := range chromeNames {
		if p, err := exec.LookPath(name); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return "", nil, fmt.Errorf("no browser found under the names %s", strings.Join(chromeNames, ", "))
	}
	dir, err := os.MkdirTemp("", "chromedp-remote-")
	if err != nil {
		return "", nil, err
	}
	// Port 0 lets the system pick a free port. Chrome prints the WebSocket URL
	// to the standard error.
	cmd := exec.CommandContext(ctx, path,
		"--headless",
		"--remote-debugging-port=0",
		"--user-data-dir="+dir,
		"--no-first-run",
		"--no-default-browser-check",
		"about:blank",
	)
	// Ask Chrome to quit with the interrupt signal, so that it stops its own
	// child processes too.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	stop := func() {
		_ = cmd.Cancel()
		_ = cmd.Wait()
		_ = os.RemoveAll(dir)
	}

	// read the lines until Chrome says where it listens
	const prefix = "DevTools listening on "
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		if wsURL, ok := strings.CutPrefix(scanner.Text(), prefix); ok {
			// keep reading, so that Chrome never blocks on a full pipe
			go func() { _, _ = io.Copy(io.Discard, stderr) }()
			return wsURL, stop, nil
		}
	}
	stop()
	return "", nil, errors.New("the browser did not say where it listens")
}
