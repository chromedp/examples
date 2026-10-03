// Command forecast is a chromedp example demonstrating how to render the weather
// forecast of Google in the terminal. It reads www.google.com and needs a
// terminal that can show images. Use the flag -q to name the place, for example
// -q Jakarta. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open.
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
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

const (
	hdrSel  = `#taw`
	dataSel = `#wob_wc`
	svgSel  = `#wob_d svg path`
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open (no effect with -remote)")
	timeout := flag.Duration("timeout", 1*time.Minute, "time limit of the program")
	query := flag.String("q", "", "place to show the weather for")
	lang := flag.String("hl", "", "language code (see hl.json)")
	unit := flag.String("unit", "", "temperature unit (C, F, or blank)")
	typ := flag.String("type", "", "kind of forecast (temp, rain, wind)")
	day := flag.Int("day", 0, "day of the forecast (0 to 7)")
	scale := flag.Float64("scale", 1.5, "scale of the screenshot")
	padding := flag.Int("padding", 20, "white space around the image, in pixels")
	remote := flag.String("remote", "", "WebSocket URL of a running browser to use")
	out := flag.String("out", "", "file to write the screenshot to")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *timeout, *query, *lang, *unit, *typ, *day, *scale, *padding, *remote, *out); err != nil {
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

func run(ctx context.Context, verbose, visible bool, timeout time.Duration, query, lang, unit, typ string, day int, scale float64, padding int, remote, out string) error {
	// make sure that the flag values are valid
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

	// build the search parameters
	v := make(url.Values)
	v.Set("q", strings.TrimSpace(query))
	if lang != "" {
		v.Set("hl", lang)
	}

	// if the flag -remote is set, use a remote browser
	if remote != "" {
		ctx, _ = chromedp.NewRemoteAllocator(ctx, remote)
	}

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
	if visible && remote == "" {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// open the search page, and find the nodes of the forecast
	if err := chromedp.Do(ctx, chromedp.Navigate("https://www.google.com/search?"+v.Encode())); err != nil {
		return err
	}
	first := func(ctx context.Context, t *chromedp.Target, n []*chromedp.Node) (*chromedp.Node, error) {
		return n[0], nil
	}
	hdrNode, err := chromedp.Run(ctx, chromedp.QueryAfter(chromedp.CSS(hdrSel), first, chromedp.NodeVisible))
	if err != nil {
		return err
	}
	dataNode, err := chromedp.Run(ctx, chromedp.QueryAfter(chromedp.CSS(dataSel), first, chromedp.NodeVisible))
	if err != nil {
		return err
	}
	nodes := []*chromedp.Node{hdrNode, dataNode}
	dataNodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSS(dataSel), chromedp.NodeVisible))
	if err != nil {
		return err
	}
	if err := chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, dom.RequestChildNodes, dom.RequestChildNodesParams{NodeID: dataNodes[0].NodeID, Depth: -1})
		return err
	})); err != nil {
		return err
	}

	// click on unit
	if unit != "" {
		if node := findNode(`°`+unit, dataNodes); node != nil {
			_ = chromedp.Do(ctx, chromedp.MouseClickNode(node))
		}
	}

	// click on type
	if typ != "temp" {
		_ = chromedp.Do(ctx, chromedp.Click(chromedp.ID("wob_"+typ)))
	}
	// hide the other types of the chart
	_, _ = chromedp.Run(ctx,
		chromedp.QueryAfter(`#wob_d > div:first-child > *:not(#wob_`+typ+`)`,
			func(ctx context.Context, t *chromedp.Target, nodes []*chromedp.Node) (chromedp.Void, error) {
				for _, n := range nodes {
					_, _ = cdp.Call(ctx, t, dom.SetAttributeValue, dom.SetAttributeValueParams{
						NodeID: n.NodeID,
						Name:   "style",
						Value:  "display:none;",
					})
				}
				return chromedp.Void{}, nil
			},
		),
	)

	// click on day
	if day != 0 {
		_ = chromedp.Do(ctx, chromedp.Click(fmt.Sprintf(`//*[@data-wob-di=%d]`, day)))
	}

	// capture the screenshot
	buf, err := chromedp.Run(ctx, chromedp.ScreenshotNodes(nodes, scale))
	if err != nil {
		return err
	}
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
	return rasterm.Encode(os.Stdout, img)
}

// findNode returns the link (an A node) that holds a span with the text val. It
// searches nodes and their children, and returns nil if it finds none.
func findNode(val string, nodes []*chromedp.Node) *chromedp.Node {
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
