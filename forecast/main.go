// Command forecast is a chromedp example demonstrating how to extract and
// render data from a page.
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
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

const dataSel = `div[data-ve-view]`

func main() {
	verbose := flag.Bool("v", false, "verbose")
	timeout := flag.Duration("timeout", 1*time.Minute, "timeout")
	query := flag.String("q", "", "query")
	lang := flag.String("hl", "en", "language")
	unit := flag.String("unit", "", "temperature unit (C, F, or blank)")
	scale := flag.Float64("scale", 1.5, "scale")
	padding := flag.Int("padding", 20, "padding")
	out := flag.String("out", "", "out file")
	flag.Parse()
	if err := run(context.Background(), *verbose, *timeout, *query, *lang, *unit, *scale, *padding, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, timeout time.Duration, query, lang, unit string, scale float64, padding int, out string) error {
	if unit = strings.ToUpper(unit); unit != "F" && unit != "C" && unit != "" {
		return fmt.Errorf("invalid unit %q", unit)
	}

	query = "weather " + query

	// build search params
	v := make(url.Values)
	v.Set("q", strings.TrimSpace(query))
	if lang != "" {
		v.Set("hl", lang)
	}

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

	// get nodes
	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.google.com/search?"+v.Encode()),
		chromedp.WaitVisible(dataSel, chromedp.ByQuery),
		chromedp.Nodes(dataSel, &nodes, chromedp.ByQuery, chromedp.NodeVisible),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return dom.RequestChildNodes(nodes[0].NodeID).WithDepth(-1).Do(ctx)
		}),
		chromedp.Sleep(50*time.Millisecond),
	); err != nil {
		return err
	}

	if unit != "" {
		// click on unit button if present
		if node := findNode(`°`+unit, nodes); node != nil {
			_ = chromedp.Run(ctx, chromedp.MouseClickNode(node))
		}
	}

	// capture screenshot
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.ScreenshotScale(dataSel, scale, &buf, chromedp.ByQuery)); err != nil {
		return err
	}
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
	return rasterm.Encode(os.Stdout, img)
}

func findNode(val string, nodes []*cdp.Node) *cdp.Node {
	for _, node := range nodes {
		if node.Parent == nil || node.Parent.Parent == nil {
			continue
		}
		if node.Parent.Parent.NodeName == "A" && node.Parent.NodeName == "SPAN" && node.NodeName == "#text" && node.NodeValue == val {
			return node.Parent.Parent
		}
		if node.ChildNodeCount > 0 {
			if n := findNode(val, node.Children); n != nil {
				return n
			}
		}
	}
	return nil
}
