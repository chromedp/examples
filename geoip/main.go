// Command forecast is a chromedp example demonstrating how to extract and
// render data from a page.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"net"
	"os"
	"strings"
	"time"

	_ "embed"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
	"github.com/oschwald/geoip2-golang"
)

func main() {
	verbose := flag.Bool("v", false, "verbose")
	timeout := flag.Duration("timeout", 1*time.Minute, "timeout")
	lang := flag.String("l", "en", "language code")
	zoom := flag.Float64("zoom", 12.5, "zoom level")
	scale := flag.Float64("scale", 1.5, "scale")
	flag.Parse()
	if err := run(context.Background(), *verbose, *timeout, *lang, *zoom, *scale, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose bool, timeout time.Duration, lang string, zoom, scale float64, args []string) error {
	// create chrome instance
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel := chromedp.NewContext(ctx, opts...)
	defer cancel()

	// create timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	for i, ipstr := range args {
		if i != 0 {
			fmt.Fprintln(os.Stdout)
		}
		ip := net.ParseIP(ipstr)
		record, err := db.City(ip)
		if err != nil {
			fmt.Fprintf(os.Stdout, "%s: unable to lookup: %v\n", ipstr, err)
			continue
		}
		var sd []string
		for _, s := range record.Subdivisions {
			sd = append(sd, s.Names[lang])
		}
		var extra string
		if len(sd) != 0 {
			extra = ", " + strings.Join(sd, ", ")
		}
		var emoji string
		if len(record.Country.IsoCode) == 2 {
			emoji = " " + emojiFlag(record.Country.IsoCode)
		}
		fmt.Fprintf(
			os.Stdout,
			"%s: %s%s, %s (%s %s) @ %f,%f%s\n",
			ipstr,
			record.City.Names[lang],
			extra,
			record.Country.Names[lang],
			record.Country.IsoCode,
			record.Location.TimeZone,
			record.Location.Latitude,
			record.Location.Longitude,
			emoji,
		)
		img, err := getMap(ctx, timeout, record.Location.Latitude, record.Location.Longitude, zoom, scale)
		if err != nil {
			fmt.Fprintf(os.Stdout, "unable to get map: %v\n", err)
			continue
		}
		if err = rasterm.Encode(os.Stdout, img); err != nil {
			fmt.Fprintf(os.Stdout, "unable to show map: %v", err)
			continue
		}
	}
	return nil
}

func getMap(ctx context.Context, timeout time.Duration, lat, lng, zoom, scale float64) (image.Image, error) {
	var address string
	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf(mapURL, lat, lng, zoom)),
		chromedp.Text(`div[data-tooltip="Copy address"] > div:nth-child(2) > span > span`, &address, chromedp.ByQuery, chromedp.NodeVisible),
		chromedp.WaitReady(`div:has(> [aria-label="Collapse side panel"])`,
			chromedp.AtLeast(7),
			chromedp.After(func(ctx context.Context, _ runtime.ExecutionContextID, nodes ...*cdp.Node) error {
				var id cdp.NodeID
				for _, n := range nodes {
					if _, err := dom.GetBoxModel().WithNodeID(n.NodeID).Do(ctx); err == nil {
						id = n.NodeID
						break
					}
				}
				if id == cdp.EmptyNodeID {
					return errors.New("unable to find node")
				}
				return chromedp.Click([]cdp.NodeID{id}, chromedp.ByNodeID).Do(ctx)
			}),
		),
		chromedp.WaitReady(`div:has(+.onegoogle) > div > div`, chromedp.ByQuery,
			chromedp.AtLeast(1),
			chromedp.After(func(ctx context.Context, _ runtime.ExecutionContextID, nodes ...*cdp.Node) error {
				script, visible := fmt.Sprintf(inViewportJS, nodes[0].FullXPath()), false
				for {
					if err := chromedp.EvaluateAsDevTools(script, &visible).Do(ctx); err != nil {
						return err
					}
					if !visible {
						<-time.After(180 * time.Millisecond)
						return nil
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(10 * time.Millisecond):
					}
				}
			}),
		),
		chromedp.ScreenshotScale(`#app-container`, scale, &buf, chromedp.ByQuery),
	); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stdout, "  Address: %s\n", address)
	return png.Decode(bytes.NewReader(buf))
}

func emojiFlag(code string) string {
	return string(0x1f1e6+rune(code[0])-'A') + string(0x1f1e6+rune(code[1])-'A')
}

const mapURL = `http://www.google.com/maps/place/%[1]f,%[2]f/@%[1]f,%[2]f,%[3]fz?hl=en`

// inViewportJS is a JavaScript snippet that will get the specified node
// position relative to the viewport and returns true if the specified node
// is within the window's viewport.
const inViewportJS = `(function(a) {
  var r = a[0].getBoundingClientRect();
  return r.top >= 0 && r.left >= 0 && r.bottom <= window.innerHeight && r.right <= window.innerWidth;
})($x(%q))`

var db *geoip2.Reader

func init() {
	var err error
	if db, err = geoip2.FromBytes(geoLite2CityMmdb); err != nil {
		panic(err)
	}
}

//go:embed GeoLite2-City.mmdb
var geoLite2CityMmdb []byte
