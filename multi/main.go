// Command multi is a chromedp example demonstrating how to use headless-shell
// and a container (Docker, Podman, other). See README.md.
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
	wait := flag.Duration("wait", 1*time.Second, "wait duration")
	out := flag.String("out", "", "out directory")
	flag.Parse()
	if err := run(context.Background(), *verbose, *wait, *out, flag.Args()...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, wait time.Duration, out string, urls ...string) error {
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
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()
	for i, urlstr := range urls {
		var buf []byte
		if err := chromedp.Run(ctx, snapshot(wait, urlstr, &buf)); err != nil {
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

func snapshot(wait time.Duration, urlstr string, buf *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.Sleep(wait),
		chromedp.CaptureScreenshot(buf),
	}
}
