// Command extension is a chromedp example demonstrating how to install a
// browser extension, uBlock Origin Lite, into the browser that chromedp starts,
// and how to show that it blocks the ads of a page. The program loads a news
// page twice in one browser, first without the extension and then with it. It
// prints the number of ad slots that a reader can see, the requests that the
// extension blocked, the height of the page, and it writes the screenshots
// news-before.png and news-after.png.
//
// Chrome 154 refuses an extension of Manifest V2, so the classic uBlock Origin
// does not load. The program loads uBlock Origin Lite, which is the Manifest V3
// build of the same project. It has fewer features than the classic build. It
// blocks with the static rules of the declarativeNetRequest API and not with
// code that runs for each request, and the filter lists are fixed when the
// extension is built. The release that the author tested, 2026.930.1227, has
// custom filters, but the lists of the build cannot change at run time. The extension
// is not in this repository, because uBlock Origin is under the license GPLv3
// and this repository is under the MIT license. The program never downloads
// it. Put it on disk yourself. See the error that the program prints when it
// finds no extension. Use -ext for the path of the unpacked directory, or of
// the ZIP file of the release. A ZIP file goes to a temporary directory that
// the program removes at the end. Without -ext, the program looks at the
// variable UBLOCK_EXTENSION, at the directory ublock in the current directory,
// and at the directory chromedp-examples/ublock in the configuration directory
// of the user.
//
// The program installs the extension with the protocol command
// Extensions.loadUnpacked. Chrome runs this command only when the program
// starts it with the flag --enable-unsafe-extension-debugging, and only over
// the pipe that chromedp uses by default for its connection. The command works
// on the browser and not on a tab, so the program calls it with CallBrowser.
// The default options of chromedp hold the flag --disable-extensions, so the
// program turns it off. Chrome 154 runs extensions in the new headless mode,
// which the flag --headless starts, so the program needs no other flag. The
// flag --load-extension does not work, because branded Chrome 137 and later
// ignores it. Because the command needs the pipe, the option KeepOpen of
// chromedp does not work with this program, as KeepOpen uses a websocket.
//
// A profile is the user data directory of the browser. The flag -profile names
// one, and the program creates it if it does not exist. Without the flag, the
// program makes a temporary profile, prints its name and removes it at the
// end. A profile keeps what the extension saves in its storage. The program
// switches the extension to the complete filtering mode, which also hides the
// generic ad elements, and the profile keeps that choice for the next run. The
// profile does not keep the installation. The command loadUnpacked loads the
// extension for this browser session only, so each start must load it again.
// Chrome makes the identifier of an unpacked extension from its path, so the
// next run finds the storage only when the extension is in the same directory.
// A ZIP file goes to a new temporary directory in each run, so use an unpacked
// directory with a profile that you keep.
//
// The test site serves the page. The page asks for scripts, images and a
// tracking pixel of the real hosts of ad networks, such as
// pagead2.googlesyndication.com and ad.doubleclick.net. The program never
// sends a request to these hosts. In the first run, it answers each request
// with the Fetch domain and a small response of its own, so that the page shows
// its ads without internet. In the second run, the same interception stays on.
// The extension decides first. It stops some requests before the Fetch domain
// sees them, and the page reports them as failed with net::ERR_BLOCKED_BY_CLIENT.
// It redirects other requests to a harmless script of its own, and the
// program does not match that address. So in the second run the program answers
// no request, and the page shows no ad. It starts a local server and needs no
// internet, but the extension must be on disk. Use -url to read another site
// with the same structure. Use -v to print the protocol messages and -visible
// to show the browser window. A visible browser closes when you close its
// window, because the program waits for you and then ends. Use
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
// The program loads both runs in one tab, so the drawing follows the page
// through both runs.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/extensions"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/termcast"

	"github.com/chromedp/examples/internal/testsite"
)

// adHosts are the hosts of the ad networks that the page asks for. The program
// pauses the requests to these hosts with the Fetch domain.
var adHosts = []string{
	"pagead2.googlesyndication.com",
	"securepubads.g.doubleclick.net",
	"ad.doubleclick.net",
	"www.googletagmanager.com",
	"www.google-analytics.com",
}

// slotSelector selects the five ad slots of the news page. A slot is visible
// when it has a size and its display is not none.
const slotSelector = `.ad-slot, .sidebar-ad, .advert-banner`

// config holds the flags.
type config struct {
	verbose bool
	visible bool
	url     string
	ext     string
	profile string
	out     string
}

func main() {
	var cfg config
	flag.BoolVar(&cfg.verbose, "v", false, "print the protocol messages")
	flag.BoolVar(&cfg.visible, "visible", false, "show the browser window and leave it open")
	flag.StringVar(&cfg.url, "url", "", "base address of a site with the structure of the test site, or empty to start the test site")
	flag.StringVar(&cfg.ext, "ext", "", "unpacked directory or ZIP file of uBlock Origin Lite on disk")
	flag.StringVar(&cfg.profile, "profile", "", "user data directory of the browser, or empty for a temporary one")
	flag.StringVar(&cfg.out, "out", "", "directory for the screenshots, or empty for a temporary one")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()
	if err := run(cfg, tc); err != nil {
		log.Fatal(err)
	}
}

// run returns an error instead of calling the method Fatal of the stream, so
// that the deferred calls stop the stream and remove the temporary files.
func run(cfg config, tc termcast.Flags) error {
	// The stream starts after the browser, so check the flags first. The
	// protocol messages of -v draw over the frames.
	if tc.Enabled && cfg.verbose {
		return fmt.Errorf("starting the stream: %w", termcast.ErrVerbose)
	}
	dir, cleanup, err := findExtension(cfg.ext)
	if err != nil {
		return err
	}
	defer cleanup()

	// start the local server
	base := cfg.url
	if base == "" {
		site := testsite.New()
		defer site.Close()
		base = site.URL
	}

	profile := cfg.profile
	if profile == "" {
		if profile, err = os.MkdirTemp("", "chromedp-profile-"); err != nil {
			return fmt.Errorf("creating the profile directory: %w", err)
		}
		// Remove the profile after the browser exits. The defers run in the
		// opposite order, and the cancel function below runs first.
		defer os.RemoveAll(profile)
	}
	out := cfg.out
	if out == "" {
		if out, err = os.MkdirTemp("", "chromedp-news-"); err != nil {
			return fmt.Errorf("creating the output directory: %w", err)
		}
	} else if err := os.MkdirAll(out, 0o755); err != nil {
		return fmt.Errorf("creating the output directory: %w", err)
	}
	fmt.Printf("profile directory: %s\nscreenshots: %s\n", profile, out)

	// Start from the default options. The program turns off disable-extensions
	// and turns on the flag that Extensions.loadUnpacked needs.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(profile),
		chromedp.Flag("disable-extensions", false),
		chromedp.Flag("enable-unsafe-extension-debugging", true),
	)
	if cfg.visible {
		// The program does not use KeepOpen. KeepOpen makes the allocator use
		// a websocket, and Extensions.loadUnpacked works only over the pipe.
		opts = append(opts, chromedp.VisibleWindow)
	}
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	var copts []chromedp.ContextOption
	if cfg.verbose {
		copts = append(copts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel = chromedp.NewContext(ctx, copts...)
	defer cancel()
	// Wait until Chrome exits, so that the profile directory has no more
	// writers when the program removes it.
	defer chromedp.Cancel(ctx)

	// An empty Do starts the browser.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// Draw the page in the terminal when the user asks for it. The stream
	// follows the tab of ctx. The program loads the page in this tab in both
	// runs, so one stream shows both runs. The results go through the held
	// writer, so that they do not draw over the frames.
	s, err := tc.Start(ctx, cfg.verbose)
	if err != nil {
		return fmt.Errorf("starting the stream: %w", err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	w := s.LogWriter()

	pageURL := strings.TrimSuffix(base, "/") + "/news/"
	before, err := visit(ctx, w, pageURL, filepath.Join(out, "news-before.png"))
	if err != nil {
		return fmt.Errorf("loading the page without the extension: %w", err)
	}
	if err := install(ctx, w, dir); err != nil {
		return err
	}
	after, err := visit(ctx, w, pageURL, filepath.Join(out, "news-after.png"))
	if err != nil {
		return fmt.Errorf("loading the page with the extension: %w", err)
	}

	fmt.Fprintf(w, "\n%-34s %-12s %s\n", "", "without", "with the extension")
	fmt.Fprintf(w, "%-34s %-12d %d\n", "ad slots that a reader can see", before.slots, after.slots)
	fmt.Fprintf(w, "%-34s %-12d %d\n", "ads drawn by the ad scripts", before.creatives, after.creatives)
	fmt.Fprintf(w, "%-34s %-12d %d\n", "page height in pixels", before.height, after.height)
	fmt.Fprintf(w, "%-34s %-12d %d\n", "requests that the program answered", len(before.answered), len(after.answered))
	fmt.Fprintf(w, "%-34s %-12d %d\n", "requests that the extension blocked", len(before.blocked), len(after.blocked))
	fmt.Fprintf(w, "%-34s %-12d %d\n", "requests that it redirected", len(before.redirected), len(after.redirected))
	printList(w, "answered by the program without the extension", before.answered)
	printList(w, "blocked by the extension", after.blocked)
	printList(w, "redirected by the extension to a script of its own", after.redirected)

	if cfg.visible {
		fmt.Fprintln(os.Stderr, "close the browser window to stop the program")
		if err := chromedp.WaitClosed(ctx); err != nil {
			return fmt.Errorf("waiting for the browser: %w", err)
		}
	}
	return nil
}

func printList(w io.Writer, title string, list []string) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, s := range list {
		fmt.Fprintf(w, "  %s\n", s)
	}
}

// install loads the extension in the browser, and switches it to the complete
// filtering mode. It prints what it did.
func install(ctx context.Context, w io.Writer, dir string) error {
	// The command works on the browser and not on a tab, so it needs
	// CallBrowser. It returns after Chrome installs the extension.
	res, err := chromedp.CallBrowser(ctx, extensions.LoadUnpacked, extensions.LoadUnpackedParams{Path: dir})
	if err != nil {
		return fmt.Errorf("loading the extension from %s: %w", dir, err)
	}
	list, err := chromedp.CallBrowser(ctx, extensions.GetExtensions, struct{}{})
	if err != nil {
		return fmt.Errorf("listing the extensions: %w", err)
	}
	for _, e := range list.Extensions {
		if e.ID == res.ID {
			fmt.Fprintf(w, "\nloaded the extension %s from %s\n", e.ID, e.Path)
		}
	}

	// An extension page can call the API of the extension. This page is the
	// dashboard of uBlock Origin Lite. It only gives a context that can send
	// messages to the service worker of the extension.
	tab, cancel := chromedp.NewContext(ctx)
	defer cancel()
	if err := chromedp.Do(tab, chromedp.Navigate("chrome-extension://"+res.ID+"/dashboard.html")); err != nil {
		return fmt.Errorf("opening the dashboard of the extension: %w", err)
	}
	// The extension has four modes: 0 for none, 1 for basic, 2 for optimal and
	// 3 for complete. The optimal mode, which is the default, hides only the
	// ads that a filter names for a site. The complete mode also hides the
	// generic ad elements, such as #ad-sidebar. The profile keeps the choice in
	// the storage of the extension.
	const message = `chrome.runtime.sendMessage({what: %q%s})`
	mode, err := chromedp.Run(tab, chromedp.Evaluate[int](fmt.Sprintf(message, "getDefaultFilteringMode", ""), chromedp.EvalAwaitPromise))
	if err != nil {
		return fmt.Errorf("reading the filtering mode: %w", err)
	}
	fmt.Fprintf(w, "filtering mode that the profile kept: %d\n", mode)
	if mode == 3 {
		return nil
	}
	// The extension registers its content scripts before it answers.
	if _, err := chromedp.Run(tab, chromedp.Evaluate[int](fmt.Sprintf(message, "setDefaultFilteringMode", ", level: 3"), chromedp.EvalAwaitPromise)); err != nil {
		return fmt.Errorf("setting the filtering mode: %w", err)
	}
	fmt.Fprintln(w, "set the filtering mode to 3")
	return nil
}

// result is what one visit of the page measured.
type result struct {
	slots      int // ad slots that a reader can see
	creatives  int // ads that the ad scripts drew
	height     int // height of the page in pixels
	answered   []string
	blocked    []string
	redirected []string
}

// measure runs in the page. A slot is visible when it has a size and its
// display is not none.
const measure = `(() => {
  const visible = [...document.querySelectorAll(%q)].filter(e => {
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0 && getComputedStyle(e).display !== "none";
  });
  return {
    slots: visible.length,
    creatives: document.querySelectorAll(".ad-creative").length,
    height: document.documentElement.scrollHeight,
  };
})()`

// visit loads the page in the tab of ctx, with the Fetch domain answering the
// requests to the ad hosts, and measures the page. It writes a screenshot of
// the whole page to shot.
func visit(ctx context.Context, w io.Writer, pageURL, shot string) (*result, error) {
	tab := ctx

	// Subscribe before the page loads, so that no event is lost. The listeners
	// use their own context, and stop ends them.
	listen, stop := context.WithCancel(tab)
	defer stop()
	var (
		mu    sync.Mutex
		res   result
		urls  = map[network.RequestID]string{}
		group sync.WaitGroup
	)
	sent := chromedp.Events(listen, network.RequestWillBeSent)
	failed := chromedp.Events(listen, network.LoadingFailed)
	paused := chromedp.Events(listen, fetch.RequestPaused)
	group.Go(func() {
		for ev, err := range sent {
			if err != nil {
				return
			}
			mu.Lock()
			urls[ev.RequestID] = ev.Request.URL
			// A redirect to the address of an extension is an answer of the
			// extension. The event holds the address that it replaced.
			if ev.RedirectResponse != nil && strings.HasPrefix(ev.Request.URL, "chrome-extension://") &&
				!strings.HasPrefix(ev.RedirectResponse.URL, "chrome-extension://") {
				res.redirected = append(res.redirected, short(ev.RedirectResponse.URL))
			}
			mu.Unlock()
		}
	})
	group.Go(func() {
		for ev, err := range failed {
			if err != nil {
				return
			}
			// An extension that blocks a request makes it fail before the
			// request leaves the network service. The Fetch domain never
			// sees such a request.
			if ev.ErrorText == "net::ERR_BLOCKED_BY_CLIENT" {
				mu.Lock()
				res.blocked = append(res.blocked, short(urls[ev.RequestID]))
				mu.Unlock()
			}
		}
	})
	group.Go(func() {
		for ev, err := range paused {
			if err != nil {
				return
			}
			name, err := answer(tab, ev)
			if err != nil {
				log.Printf("answering %s: %v", short(ev.Request.URL), err)
				continue
			}
			mu.Lock()
			res.answered = append(res.answered, name)
			mu.Unlock()
		}
	})

	// Only the requests to the ad hosts are paused. The page and its images
	// load as usual. The wildcard * means zero or more characters.
	var patterns []*fetch.RequestPattern
	for _, host := range adHosts {
		patterns = append(patterns, &fetch.RequestPattern{URLPattern: "https://" + host + "/*"})
	}
	if _, err := chromedp.Call(tab, fetch.Enable, fetch.EnableParams{Patterns: patterns}); err != nil {
		return nil, fmt.Errorf("enabling the Fetch domain: %w", err)
	}
	// The extension cannot see a response that comes from the cache.
	if _, err := chromedp.Call(tab, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true}); err != nil {
		return nil, fmt.Errorf("disabling the cache: %w", err)
	}
	if err := chromedp.Do(tab,
		chromedp.EmulateViewport(1280, 900),
		chromedp.Navigate(pageURL),
	); err != nil {
		return nil, fmt.Errorf("loading %s: %w", pageURL, err)
	}

	// The ad scripts run after the page loads. Wait until the measures stay
	// the same for half a second.
	type measured struct {
		Slots     int `json:"slots"`
		Creatives int `json:"creatives"`
		Height    int `json:"height"`
	}
	var last measured
	for stable, i := 0, 0; stable < 3; i++ {
		if i > 50 {
			return nil, errors.New("the page did not settle")
		}
		m, err := chromedp.Run(tab, chromedp.Evaluate[measured](fmt.Sprintf(measure, slotSelector)))
		if err != nil {
			return nil, fmt.Errorf("measuring the page: %w", err)
		}
		if m == last && i > 0 {
			stable++
		} else {
			stable = 0
		}
		last = m
		time.Sleep(200 * time.Millisecond)
	}

	buf, err := chromedp.Run(tab, chromedp.FullScreenshot(100))
	if err != nil {
		return nil, fmt.Errorf("taking the screenshot: %w", err)
	}
	if err := os.WriteFile(shot, buf, 0o644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", shot, err)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(buf)); err == nil {
		fmt.Fprintf(w, "wrote %s (%d x %d pixels, %d bytes)\n", shot, cfg.Width, cfg.Height, len(buf))
	}

	stop()
	group.Wait()
	mu.Lock()
	defer mu.Unlock()
	res.slots, res.creatives, res.height = last.Slots, last.Creatives, last.Height
	return &res, nil
}

// short returns the host and the path of a URL.
func short(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host + u.Path
}

// answer fulfills a paused request with a small response, so that the page
// behaves as if the ad network answered. It returns the name of the request.
func answer(ctx context.Context, ev fetch.EventRequestPaused) (string, error) {
	u, err := url.Parse(ev.Request.URL)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", ev.Request.URL, err)
	}
	contentType, body := "text/javascript", ""
	var raw []byte
	switch {
	case u.Path == "/pagead/js/adsbygoogle.js":
		body = adsByGoogleJS
	case u.Path == "/tag/js/gpt.js":
		body = gptJS
	case u.Path == "/gtag/js":
		body = "window.gtagLoaded = true;"
	case u.Host == "ad.doubleclick.net":
		contentType, raw = "image/png", bannerPNG()
	default:
		// the tracking pixel, a GIF of 1 pixel
		contentType = "image/gif"
		raw, _ = base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	}
	if raw == nil {
		raw = []byte(body)
	}
	_, err = chromedp.Call(ctx, fetch.FulfillRequest, fetch.FulfillRequestParams{
		RequestID:    ev.RequestID,
		ResponseCode: 200,
		ResponseHeaders: []*fetch.HeaderEntry{
			{Name: "Content-Type", Value: contentType},
			// The page loads the ad script with crossorigin="anonymous", as a
			// real page does, so the script needs this header.
			{Name: "Access-Control-Allow-Origin", Value: "*"},
		},
		Body: raw,
	})
	if err != nil {
		return "", fmt.Errorf("fulfilling the request: %w", err)
	}
	return short(ev.Request.URL), nil
}

// creative is the markup that the scripts below put in a slot. It is a plain
// box with a text, so the page shows an ad without a picture from the internet.
const creative = `<div class="ad-creative" style="box-sizing:border-box;width:%s;height:%s;margin:0 auto;display:flex;align-items:center;justify-content:center;background:linear-gradient(90deg,#7c2d12,#ea580c);color:#fff;font:700 20px sans-serif;border-radius:4px">Harbour Bank: open an account today</div>`

// adsByGoogleJS imitates the script of the ad network. It draws a box in each
// slot that has no ad yet. The page pushes a request for each slot.
var adsByGoogleJS = `(function () {
  function fill() {
    document.querySelectorAll("ins.adsbygoogle").forEach(function (ins) {
      if (ins.dataset.adStatus) { return; }
      ins.dataset.adStatus = "filled";
      var wide = ins.dataset.adFormat === "horizontal";
      ins.innerHTML = '` + fmt.Sprintf(creative, `' + (wide ? "728px" : "300px") + '`, `' + (wide ? "90px" : "250px") + '`) + `';
    });
  }
  window.adsbygoogle = {push: fill};
  fill();
})();`

// gptJS imitates the Google Publisher Tag. It runs the queue of the page, and
// it draws a box in the slot that the page displays.
var gptJS = `(function () {
  var g = window.googletag = window.googletag || {};
  var queue = g.cmd || [];
  var pub = {enableSingleRequest: function () { return pub; }};
  g.cmd = {push: function (f) { f.call(g); }};
  g.pubads = function () { return pub; };
  g.defineSlot = function (path, size, id) {
    var slot = {size: size, addService: function () { return slot; }};
    return slot;
  };
  g.enable = function () {};
  g.display = function (id) {
    document.getElementById(id).innerHTML = '` + fmt.Sprintf(creative, "300px", "250px") + `';
  };
  queue.forEach(function (f) { f.call(g); });
})();`

// bannerPNG draws a picture of 728 by 90 pixels, a gradient with a button.
func bannerPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 728, 90))
	for x := range 728 {
		c := color.RGBA{R: uint8(30 + x/6), G: uint8(64 + x/10), B: 170, A: 255}
		for y := range 90 {
			img.Set(x, y, c)
		}
	}
	for x := 560; x < 700; x++ {
		for y := 28; y < 62; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// findExtension returns the unpacked directory of the extension, and a
// function that removes the temporary files. It never uses the network.
func findExtension(flagPath string) (string, func(), error) {
	candidates := []string{flagPath}
	if flagPath == "" {
		candidates = []string{os.Getenv("UBLOCK_EXTENSION"), "ublock"}
		if cfg, err := os.UserConfigDir(); err == nil {
			candidates = append(candidates, filepath.Join(cfg, "chromedp-examples", "ublock"))
		}
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if flagPath != "" {
				return "", nil, fmt.Errorf("reading -ext: %w", err)
			}
			continue
		}
		dir, cleanup := path, func() {}
		if !info.IsDir() {
			if dir, err = os.MkdirTemp("", "chromedp-ublock-"); err != nil {
				return "", nil, fmt.Errorf("creating a directory for the ZIP file: %w", err)
			}
			cleanup = func() { os.RemoveAll(dir) }
			if err := unzip(path, dir); err != nil {
				cleanup()
				return "", nil, err
			}
		}
		if dir, err = manifestDir(dir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("using %s: %w", path, err)
		}
		if dir, err = filepath.Abs(dir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("making the path absolute: %w", err)
		}
		if err := checkManifest(dir); err != nil {
			cleanup()
			return "", nil, err
		}
		return dir, cleanup, nil
	}
	return "", nil, errors.New(noExtension)
}

// noExtension is the error when the program finds no extension.
const noExtension = `no extension found, and the program never downloads one.

uBlock Origin is not in this repository, because it uses the license GPLv3
and this repository uses the MIT license. Put it on disk:

  1. Download the file uBOLite_<version>.chromium.zip from
     https://github.com/uBlockOrigin/uBOL-home/releases/latest
  2. Unpack it into the directory ublock:
     unzip uBOLite_<version>.chromium.zip -d ublock
  3. Run the program again in the same directory, or give the path with
     -ext ublock, or with the variable UBLOCK_EXTENSION. -ext also takes the
     ZIP file.

Chrome 154 refuses the classic uBlock Origin, because it uses Manifest V2.
Its releases are at https://github.com/gorhill/uBlock/releases and they do not
load. uBlock Origin Lite uses Manifest V3 and does load.`

// manifestDir returns dir, or its only subdirectory when that holds the file
// manifest.json. A ZIP file of a release can have the files in a directory.
func manifestDir(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", dir, err)
	}
	if len(entries) == 1 && entries[0].IsDir() {
		sub := filepath.Join(dir, entries[0].Name())
		if _, err := os.Stat(filepath.Join(sub, "manifest.json")); err == nil {
			return sub, nil
		}
	}
	return "", errors.New("it has no manifest.json")
}

// checkManifest prints the extension, and stops early for Manifest V2.
func checkManifest(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("reading the manifest: %w", err)
	}
	var m struct {
		Name            string `json:"name"`
		Version         string `json:"version"`
		ManifestVersion int    `json:"manifest_version"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parsing the manifest of %s: %w", dir, err)
	}
	if m.ManifestVersion < 3 {
		return fmt.Errorf("the extension in %s uses Manifest V%d, and Chrome 154 refuses it. Use uBlock Origin Lite, which uses Manifest V3", dir, m.ManifestVersion)
	}
	fmt.Printf("extension: version %s, Manifest V%d, in %s\n", m.Version, m.ManifestVersion, dir)
	return nil
}

// maxFile is the largest size that the program unpacks from one file of the ZIP.
const maxFile = 64 << 20

// unzip unpacks the ZIP file into dir. It refuses a name that leaves dir.
func unzip(name, dir string) error {
	r, err := zip.OpenReader(name)
	if err != nil {
		return fmt.Errorf("opening %s: %w", name, err)
	}
	defer r.Close()
	for _, f := range r.File {
		// filepath.IsLocal is false for an absolute name and for a name that
		// holds ".." and leaves the directory.
		if !filepath.IsLocal(f.Name) {
			return fmt.Errorf("the ZIP file has the name %q, which leaves the directory", f.Name)
		}
		target := filepath.Join(dir, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", target, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(target), err)
		}
		if err := unzipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func unzipFile(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return fmt.Errorf("reading %s: %w", f.Name, err)
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", target, err)
	}
	defer dst.Close()
	n, err := io.Copy(dst, io.LimitReader(src, maxFile+1))
	if err != nil {
		return fmt.Errorf("unpacking %s: %w", f.Name, err)
	}
	if n > maxFile {
		return fmt.Errorf("%s is larger than %d bytes", f.Name, maxFile)
	}
	return dst.Close()
}
