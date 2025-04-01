// Command forecast is a chromedp example demonstrating how to extract and
// render data from a page.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

const (
	hdrSel  = `#taw`
	dataSel = `#wob_wc`
	svgSel  = `#wob_d svg path`
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	timeout := flag.Duration("timeout", 1*time.Minute, "timeout")
	query := flag.String("q", "", "weather query")
	lang := flag.String("hl", "", "language (see hl.json)")
	unit := flag.String("unit", "", "temperature unit (C, F, or blank)")
	typ := flag.String("type", "", "selection type (temp, rain, wind)")
	day := flag.Int("day", 0, "day (0-7)")
	scale := flag.Float64("scale", 1.5, "scale")
	padding := flag.Int("padding", 20, "padding")
	remote := flag.String("remote", "", "remote")
	out := flag.String("out", "", "out file")
	flag.Parse()
	if err := run(context.Background(), *verbose, *timeout, *query, *lang, *unit, *typ, *day, *scale, *padding, *remote, *out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if strings.HasPrefix(err.Error(), "invalid lang ") {
			fmt.Fprint(os.Stderr, "\nvalid languages:\n")
			for _, key := range slices.Sorted(maps.Keys(langs)) {
				fmt.Fprintf(os.Stderr, " %s:\t%s\n", key, langs[key])
			}
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, timeout time.Duration, query, lang, unit, typ string, day int, scale float64, padding int, remote, out string) error {
	// check
	lang = strings.ToLower(lang)
	if _, ok := langs[lang]; !ok && lang != "" {
		return fmt.Errorf("invalid lang %q", lang)
	}
	if unit = strings.ToUpper(unit); unit != "F" && unit != "C" && unit != "" {
		return fmt.Errorf("invalid unit %q", unit)
	}
	switch typ = strings.ToLower(typ); typ {
	case "":
		typ = "temp"
	case "temp", "rain", "wind":
	default:
		return fmt.Errorf("invalid type %q", typ)
	}
	if day < 0 || day > 7 {
		return fmt.Errorf("invalid day %d", day)
	}
	if scale <= 0 {
		return fmt.Errorf("invalid scale %f", scale)
	}
	if padding < 0 {
		return fmt.Errorf("invalid padding %d", padding)
	}

	query = "weather forecast " + query

	// build search params
	v := make(url.Values)
	v.Set("q", strings.TrimSpace(query))
	if lang != "" {
		v.Set("hl", lang)
	}

	// use remote allocator context if specified
	if remote != "" {
		ctx, _ = chromedp.NewRemoteAllocator(ctx, remote)
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

	// get
	var nodes, dataNodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.google.com/search?"+v.Encode()),
		chromedp.QueryAfter(hdrSel, func(ctx context.Context, id runtime.ExecutionContextID, n ...*cdp.Node) error {
			nodes = append(nodes, n[0])
			return nil
		}, chromedp.ByQuery, chromedp.NodeVisible),
		chromedp.QueryAfter(dataSel, func(ctx context.Context, id runtime.ExecutionContextID, n ...*cdp.Node) error {
			nodes = append(nodes, n[0])
			return nil
		}, chromedp.ByQuery, chromedp.NodeVisible),
		chromedp.Nodes(dataSel, &dataNodes, chromedp.ByQuery, chromedp.NodeVisible),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return dom.RequestChildNodes(dataNodes[0].NodeID).WithDepth(-1).Do(ctx)
		}),
	); err != nil {
		return err
	}

	// click on unit
	if unit != "" {
		if node := findNode(`°`+unit, dataNodes); node != nil {
			_ = chromedp.Run(ctx, chromedp.MouseClickNode(node))
		}
	}

	// click on type
	if typ != "temp" {
		_ = chromedp.Run(ctx, chromedp.Click("wob_"+typ, chromedp.ByID))
	}
	// hide other types
	_ = chromedp.Run(ctx,
		chromedp.QueryAfter(`#wob_d > div:first-child > *:not(#wob_`+typ+`)`,
			func(ctx context.Context, id runtime.ExecutionContextID, nodes ...*cdp.Node) error {
				for _, n := range nodes {
					_ = dom.SetAttributeValue(n.NodeID, "style", "display:none;").Do(ctx)
				}
				return nil
			},
		),
	)

	// click on day
	if day != 0 {
		_ = chromedp.Run(ctx, chromedp.Click(fmt.Sprintf(`//*[@data-wob-di=%d]`, day)))
	}

	// capture screenshot
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.ScreenshotNodes(nodes, scale, &buf)); err != nil {
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

var langs map[string]string

func init() {
	if err := json.Unmarshal(hlJSON, &langs); err != nil {
		panic(err)
	}
}

//go:embed hl.json
var hlJSON []byte
