// Command fast is a chromedp example demonstrating how to measure the speed of
// the internet connection and show the result in the terminal. It reads
// fast.com. Inspired by [adhocore/fast]. Use -v to print the protocol messages
// and -visible to show the browser window and leave it open.  [adhocore/fast]:
// https://github.com/adhocore/fast
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout")
	scale := flag.Float64("scale", 1.5, "scale")
	padding := flag.Int("padding", 0, "padding")
	out := flag.String("out", "", "out")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *timeout, *scale, *padding, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose, visible bool, timeout time.Duration, scale float64, padding int, out string) error {
	// create context
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

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// run the speed test, and capture the result
	if err := chromedp.Do(ctx,
		chromedp.Navigate(`https://fast.com`),
		chromedp.WaitVisible(`#speed-value.succeeded`),
		chromedp.Click(`#show-more-details-link`),
		chromedp.WaitVisible(`#upload-value.succeeded`),
	); err != nil {
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

	// show the image in the terminal
	if err := rasterm.Encode(os.Stdout, img); err != nil {
		return err
	}

	// print the time of the test
	_, err = fmt.Fprintf(os.Stdout, "time: %v\n", end.Sub(start))
	return err
}
