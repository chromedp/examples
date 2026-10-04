// Command emulate is a chromedp example demonstrating how to emulate a specific
// device such as an iPhone. It starts a local server and needs no internet. The
// program emulates an iPhone 17 with a preset of the package chromedp/device,
// and loads the pages /ua and /viewport-test of the local test site. It prints
// the user agent, the viewport, the device pixel ratio and the touch support that
// the page reports, and the layout that the page chooses. Then it resets the
// emulation, sets a desktop viewport and does the same for the desktop. For
// both it writes a screenshot of the window and a screenshot of the full page to
// the directory of the flag -out, which is the current directory by default, and
// it prints the size of each file. The flag -url gives the full URL of one page
// to read, for example a live site, and then the program does not start the
// local site and uses this page for both steps. The selectors are written for
// the local site, so a live site can differ. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/png" // the decoder, for image.DecodeConfig
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
)

// facts are the ids of the elements of the page /ua, with the label that the
// program prints for each. The script of the page writes into them what the
// browser reports about itself. Emulate changes the user agent, but not the
// platform, so the platform of the phone is still the one of this machine.
var facts = []struct{ label, sel string }{
	{"user agent", `#ua`},
	{"platform", `#platform`},
	{"viewport", `#viewport`},
	{"screen", `#screen`},
	{"device pixel ratio", `#dpr`},
	{"touch points", `#touch`},
	{"orientation", `#orientation`},
}

func main() {
	urlstr := flag.String("url", "", "full URL of the page to read, for example a live site. The local test site starts when it is empty. The selectors are written for the local site and a live site can differ")
	out := flag.String("out", ".", "directory for the screenshots")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// choose the pages. Without -url, the program starts the local site. The
	// page /ua reports the browser, and the page /viewport-test changes its
	// layout at 480 px and at 1099 px
	reportURL, layoutURL := *urlstr, *urlstr
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		reportURL, layoutURL = site.URL+"/ua", site.URL+"/viewport-test"
	}

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

	// create a timeout, so that no wait loop can run forever
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("creating the directory %s: %v", *out, err)
	}

	// emulate an iPhone. The preset holds the user agent, the size of the
	// viewport in CSS pixels, the device pixel ratio, and the flags for a
	// mobile browser and for touch. Emulate sends all of them to the browser
	phone := device.IPhone17.Device()
	fmt.Printf("emulating %s: %d x %d px, pixel ratio %g, mobile %t, touch %t\n",
		phone.Name, phone.Width, phone.Height, phone.Scale, phone.Mobile, phone.Touch)
	if err := chromedp.Do(ctx, chromedp.Emulate(device.IPhone17)); err != nil {
		log.Fatal(err)
	}
	if err := inspect(ctx, "phone", reportURL, layoutURL, *out); err != nil {
		log.Fatal(err)
	}

	// reset the emulation. Emulate(device.Reset) sets back the user agent,
	// the size of the window, the pixel ratio and the flags for mobile and
	// touch. A desktop does not need a preset, so set a large viewport. The
	// window is the same, and the page sees a new size
	fmt.Println("emulating a desktop: 1280 x 800 px")
	if err := chromedp.Do(ctx,
		chromedp.Emulate(device.Reset),
		chromedp.EmulateViewport(1280, 800),
	); err != nil {
		log.Fatal(err)
	}
	if err := inspect(ctx, "desktop", reportURL, layoutURL, *out); err != nil {
		log.Fatal(err)
	}
}

// inspect prints what the page /ua reports, and what layout the page
// /viewport-test shows. It writes two screenshots of the layout page, one of the
// window and one of the full page. The name is the start of the names of the
// files.
func inspect(ctx context.Context, name, reportURL, layoutURL, dir string) error {
	// the script of the page fills the elements when the page loads, and sets
	// the attribute data-ready of the body when it has finished. Wait for the
	// attribute, because the elements hold the word "unknown" until then
	if err := chromedp.Do(ctx,
		chromedp.Navigate(reportURL),
		chromedp.WaitReady(`body[data-ready="true"]`),
	); err != nil {
		return fmt.Errorf("loading %s: %w", reportURL, err)
	}
	for _, f := range facts {
		text, err := chromedp.Run(ctx, chromedp.Text(f.sel))
		if err != nil {
			return fmt.Errorf("reading %s: %w", f.sel, err)
		}
		fmt.Printf("  %-19s %s\n", f.label+":", text)
	}

	// the layout of the page. The label shows the active layout. Its spans
	// have the property display none, except for one, and the property
	// innerText gives only the visible text
	if err := chromedp.Do(ctx, chromedp.Navigate(layoutURL)); err != nil {
		return fmt.Errorf("loading %s: %w", layoutURL, err)
	}
	label, err := chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.vp-label').innerText`))
	if err != nil {
		return fmt.Errorf("reading the layout label: %w", err)
	}
	fmt.Printf("  %-19s %s\n", "layout:", label)

	// CaptureScreenshot takes what the window shows. Its image has the size of
	// the viewport multiplied by the device pixel ratio, so a phone with the
	// ratio 3 gives an image that is 3 times as wide as the viewport
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return fmt.Errorf("capturing the window: %w", err)
	}
	if err := save(dir, name+"-window.png", buf); err != nil {
		return err
	}

	// FullScreenshot takes the whole page, at the same pixel ratio
	buf, err = chromedp.Run(ctx, chromedp.FullScreenshot(100))
	if err != nil {
		return fmt.Errorf("capturing the full page: %w", err)
	}
	return save(dir, name+"-full.png", buf)
}

// save writes an image file and prints its size in pixels and in bytes.
func save(dir, name string, buf []byte) error {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("reading the size of %s: %w", path, err)
	}
	fmt.Printf("  wrote %s: %d x %d px, %d bytes\n", path, cfg.Width, cfg.Height, len(buf))
	return nil
}
