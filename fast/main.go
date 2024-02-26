// Command fast is a chromedp example demonstrating how to extract and
// render data from a page. Inspired by [adhocore/fast].
//
// [adhocore/fast]: https://github.com/adhocore/fast
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
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout")
	scale := flag.Float64("scale", 1.5, "scale")
	padding := flag.Int("padding", 0, "padding")
	out := flag.String("out", "", "out")
	flag.Parse()
	if err := run(context.Background(), *verbose, *timeout, *scale, *padding, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, timeout time.Duration, scale float64, padding int, out string) error {
	// create chrome instance
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// capture screenshot
	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(`https://fast.com`),
		chromedp.WaitVisible(`#speed-value.succeeded`),
		chromedp.Click(`#show-more-details-link`),
		chromedp.WaitVisible(`#upload-value.succeeded`),
		chromedp.ScreenshotScale(`.speed-controls-container`, scale, &buf),
	); err != nil {
		return err
	}

	end := time.Now()

	// decode png
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return err
	}

	// pad image
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

	// write to disk
	if out != "" {
		if err := os.WriteFile(out, buf, 0o644); err != nil {
			return err
		}
	}

	// output
	if err := rasterm.Encode(os.Stdout, img); err != nil {
		return err
	}

	// metrics
	_, err = fmt.Fprintf(os.Stdout, "time: %v\n", end.Sub(start))
	return err
}
