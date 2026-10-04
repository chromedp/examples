// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport. It
// starts a local server and needs no internet. The program reads the long
// article /article/Harbour_Line and the reference /docs/time of the local test
// site, which are more than 20,000 px tall, and it writes these files to the
// directory of the flag -out, which is the current directory by default: the
// viewport, an element at the scale 1 and 2, a clip of a region, the full page
// as a PNG and as a JPEG, and the full page of the tall article again from
// tiles. For each file it prints the size in pixels and in bytes, and the
// memory that the decoded image needs. The flag -url gives the full URL of one
// page to read, for example a live site, and then the program does not start
// the local site. The selectors are written for the local site, so a live site
// can differ. Use -v to print the protocol messages, -visible to show the
// browser window and leave it open, and -visible-on-terminal to draw the page in
// the terminal with terminal graphics.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // the decoder, for image.DecodeConfig
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
)

// maxTexture is the largest side of a texture that the graphics code of Chrome
// can make, in pixels. A page that is taller than this needs more care, see
// fullPageTiles.
const maxTexture = 16384

// tileHeight is the height of one strip of fullPageTiles. It is far below
// maxTexture, so that the browser sends a small message for each strip.
const tileHeight = 8192

// out receives the results that the program prints. It is the standard output,
// or the held writer of the stream when the flag -visible-on-terminal is on,
// because the stream clears the terminal and would erase the results.
var out io.Writer = os.Stdout

// target is a page to read, and the name that its files get.
type target struct {
	Name string
	URL  string
}

func main() {
	urlstr := flag.String("url", "", "full URL of the page to read, for example a live site. The local test site starts when it is empty. The selectors are written for the local site and a live site can differ")
	outDir := flag.String("out", ".", "directory for the image files")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// choose the pages. Without -url, the program starts the local site
	var targets []target
	if *urlstr != "" {
		targets = []target{{Name: "page", URL: *urlstr}}
	} else {
		site := testsite.New()
		defer site.Close()
		targets = []target{
			{Name: "article", URL: site.URL + "/article/Harbour_Line"},
			{Name: "docs-time", URL: site.URL + "/docs/time"},
		}
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

	// start the stream before the first navigation, so that the frames show
	// the pages while they load
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	if s != nil {
		out = s.LogWriter()
	}

	// create a timeout, so that no wait loop can run forever. A local page
	// is fast, but a full screenshot of a very tall page can take seconds
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := run(ctx, targets, *outDir); err != nil {
		s.Fatal(err)
	}
}

// run takes the screenshots. The element, viewport and clip screenshots use
// the first page, and the full screenshots use every page.
func run(ctx context.Context, targets []target, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating the directory %s: %w", dir, err)
	}
	first := targets[0]
	if err := chromedp.Do(ctx, chromedp.Navigate(first.URL)); err != nil {
		return fmt.Errorf("loading %s: %w", first.URL, err)
	}

	// the viewport: CaptureScreenshot takes only what the window shows now,
	// so its size is the size of the window and not the size of the page
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return fmt.Errorf("capturing the viewport: %w", err)
	}
	if err := save(dir, "viewport.png", buf); err != nil {
		return err
	}

	// elements: Screenshot scrolls the first element that matches the
	// selector into view, waits until it is visible, and captures its box.
	// The selectors are for the local site. Every page of it has the logo,
	// and the article has the infobox
	for _, e := range []struct{ file, sel string }{
		{"element-logo.png", `img.Homepage-logo`},
		{"element-infobox.png", `table.infobox`},
	} {
		buf, err := chromedp.Run(ctx, chromedp.Screenshot(chromedp.CSS(e.sel)))
		if err != nil {
			return fmt.Errorf("capturing %s: %w", e.sel, err)
		}
		if err := save(dir, e.file, buf); err != nil {
			return err
		}
	}

	// the same element at the scale 2. The page scale factor makes the
	// browser draw the element again with twice the pixels, so the text is
	// sharp and the image is four times as large
	buf, err = chromedp.Run(ctx, chromedp.ScreenshotScale(chromedp.CSS(`table.infobox`), 2))
	if err != nil {
		return fmt.Errorf("capturing the infobox at the scale 2: %w", err)
	}
	if err := save(dir, "element-infobox-2x.png", buf); err != nil {
		return err
	}

	// a clip: a rectangle of the page, in CSS pixels from its top left
	// corner. It can lie outside the viewport, because of CaptureBeyondViewport
	buf, err = clipScreenshot(ctx, page.Viewport{X: 20, Y: 1000, Width: 400, Height: 300, Scale: 2})
	if err != nil {
		return err
	}
	if err := save(dir, "clip.png", buf); err != nil {
		return err
	}

	// the full pages
	for _, t := range targets {
		if err := fullPage(ctx, t, dir); err != nil {
			return err
		}
	}

	// the decoded images are gone, but the numbers show what the program
	// needed from the system
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(out, "memory of the program: %.0f MB from the system, %.0f MB allocated in total\n",
		mb(m.Sys), mb(m.TotalAlloc))
	return nil
}

// fullPage writes the full screenshot of a page as a PNG and as a JPEG. When
// the page is taller than maxTexture, it writes a third file that comes from
// tiles.
func fullPage(ctx context.Context, t target, dir string) error {
	if err := chromedp.Do(ctx, chromedp.Navigate(t.URL)); err != nil {
		return fmt.Errorf("loading %s: %w", t.URL, err)
	}

	// Chrome loads an image with loading="lazy" only when it comes near the
	// viewport. A page with such images grows while it loads them, so the
	// measure of the height and the screenshot would not match. Ask Chrome
	// to load all images now, and wait until they are decoded.
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`
		Promise.all(Array.from(document.images, img => {
			img.loading = 'eager';
			return img.decode().catch(() => {});
		})).then(() => true)`, chromedp.EvalAwaitPromise)); err != nil {
		return fmt.Errorf("loading the images of %s: %w", t.URL, err)
	}

	width, height, err := contentSize(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s is %d x %d px\n", t.Name, width, height)

	// FullScreenshot(100) makes a PNG, and a quality below 100 makes a JPEG.
	// It sets CaptureBeyondViewport, so Chrome captures the whole page and
	// not only the window. A JPEG is smaller, but it loses detail
	for _, q := range []struct {
		quality int
		ext     string
	}{{100, "png"}, {80, "jpg"}} {
		buf, err := chromedp.Run(ctx, chromedp.FullScreenshot(q.quality))
		if err != nil {
			return fmt.Errorf("capturing the full page of %s: %w", t.URL, err)
		}
		if err := save(dir, fmt.Sprintf("full-%s.%s", t.Name, q.ext), buf); err != nil {
			return err
		}
	}

	// A texture is at most 16,384 px high, so a page that is taller than that
	// cannot be one texture. Chrome 154 handles it, and it captures the page
	// in parts by itself. A program that must work with an older Chrome, or
	// that wants to keep the messages small, can capture the page in strips.
	if height > maxTexture {
		return fullPageTiles(ctx, t, dir, width, height)
	}
	return nil
}

// fullPageTiles captures a tall page in strips with a clip and joins them in
// one image. Each strip is a separate command, so no message is large, and a
// failure shows which part of the page failed.
//
// The joined image needs width * height * 4 bytes in memory, and the program
// prints this number. For a page that is much taller, write each strip to its
// own file instead.
func fullPageTiles(ctx context.Context, t target, dir string, width, height int) error {
	whole := image.NewNRGBA(image.Rect(0, 0, width, height))
	strips := 0
	for y := 0; y < height; y += tileHeight {
		h := min(tileHeight, height-y)
		buf, err := clipScreenshot(ctx, page.Viewport{X: 0, Y: float64(y), Width: float64(width), Height: float64(h), Scale: 1})
		if err != nil {
			return fmt.Errorf("capturing the strip at %d px of %s: %w", y, t.URL, err)
		}
		strip, err := png.Decode(bytes.NewReader(buf))
		if err != nil {
			return fmt.Errorf("decoding the strip at %d px of %s: %w", y, t.URL, err)
		}
		draw.Draw(whole, image.Rect(0, y, width, y+h), strip, strip.Bounds().Min, draw.Src)
		strips++
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, whole); err != nil {
		return fmt.Errorf("encoding the joined image of %s: %w", t.URL, err)
	}
	fmt.Fprintf(out, "%s: joined %d strips of at most %d px\n", t.Name, strips, tileHeight)
	return save(dir, fmt.Sprintf("full-%s-tiles.png", t.Name), buf.Bytes())
}

// contentSize returns the size of the whole page in CSS pixels, which is the
// size that a full screenshot has at the scale 1.
func contentSize(ctx context.Context) (width, height int, err error) {
	res, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) (page.GetLayoutMetricsResult, error) {
		return cdp.Call(ctx, t, page.GetLayoutMetrics, cdp.Empty{})
	})
	if err != nil {
		return 0, 0, fmt.Errorf("reading the layout metrics: %w", err)
	}
	return int(math.Ceil(res.CSSContentSize.Width)), int(math.Ceil(res.CSSContentSize.Height)), nil
}

// clipScreenshot captures a rectangle of the page as a PNG. The rectangle is in
// CSS pixels, and its Scale multiplies the size of the image.
func clipScreenshot(ctx context.Context, clip page.Viewport) ([]byte, error) {
	buf, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
			Format:                page.CaptureScreenshotFormatPng,
			Clip:                  &clip,
			FromSurface:           new(true),
			CaptureBeyondViewport: new(true),
		})
		return res.Data, err
	})
	if err != nil {
		return nil, fmt.Errorf("capturing the clip %v: %w", clip, err)
	}
	return buf, nil
}

// save writes an image file and prints its size in pixels and in bytes, and the
// memory that the decoded image needs. It reads the size from the file header.
func save(dir, name string, buf []byte) error {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("reading the size of %s: %w", path, err)
	}
	fmt.Fprintf(out, "wrote %-28s %-4s %5d x %5d px %9d bytes, %4.0f MB when decoded\n",
		path, format, cfg.Width, cfg.Height, len(buf), mb(uint64(cfg.Width)*uint64(cfg.Height)*4))
	return nil
}

// mb converts bytes to megabytes.
func mb(n uint64) float64 {
	return float64(n) / (1 << 20)
}
