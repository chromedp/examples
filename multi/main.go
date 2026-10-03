// Command multi is a chromedp example demonstrating how to use headless-shell
// and a container (Docker, Podman, other). The program takes URLs as arguments,
// takes a screenshot of each page and prints its size. The flag -out names a
// directory for the PNG files. See README.md. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/yookoala/realpath"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	wait := flag.Duration("wait", 1*time.Second, "wait duration")
	out := flag.String("out", "", "out directory")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *wait, *out, flag.Args()...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose, visible bool, wait time.Duration, out string, urls ...string) error {
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
		var err error
		if out, err = realpath.Realpath(out); err != nil {
			return err
		}
	}
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if visible {
		opts = append(opts, chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()
	if visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}
	for i, urlstr := range urls {
		buf, err := snapshot(ctx, wait, urlstr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: unable to snapshot %d (%s): %v\n", i, urlstr, err)
			continue
		}
		img, err := png.Decode(bytes.NewReader(buf))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: unable to decode snapshot %d (%s): %v\n", i, urlstr, err)
			continue
		}
		b := img.Bounds()
		fmt.Fprintf(os.Stdout, "image %d (%s) width: %d height: %d\n", i, urlstr, b.Dx(), b.Dy())
		if out != "" {
			outpath := path.Join(out, strconv.Itoa(i)+".png")
			if err := os.WriteFile(outpath, buf, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error: unable to write snapshot %d (%s): %v\n", i, urlstr, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "wrote image %d (%s) -> %s\n", i, urlstr, outpath)
		}
	}
	return nil
}

// snapshot navigates to urlstr, waits, and returns a screenshot of the page.
func snapshot(ctx context.Context, wait time.Duration, urlstr string) ([]byte, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.Sleep(wait),
	); err != nil {
		return nil, err
	}
	return chromedp.Run(ctx, chromedp.CaptureScreenshot())
}
