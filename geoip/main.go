// Command geoip is a chromedp example demonstrating how to look up the location
// of an IP address and show its map in the terminal. It starts a local server
// and needs no internet, but it needs a terminal that can show images. Give one
// or more IP addresses as arguments, or none to look up the address of this
// computer. The program reads the answer of the lookup service, and then it
// takes a screenshot of the map of the place. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open. The flag
// -url reads another site instead. The selectors are written for the local
// site, so a live site can differ.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/kenshaw/rasterm"
)

// record is the answer of the lookup service for one address.
type record struct {
	IP           string  `json:"ip"`
	Range        string  `json:"range"`
	Country      string  `json:"country"`
	CountryCode  string  `json:"country_code"`
	Region       string  `json:"region"`
	City         string  `json:"city"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Timezone     string  `json:"timezone"`
	Organization string  `json:"organization"`
	AccuracyKM   int     `json:"accuracy_radius_km"`
}

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	timeout := flag.Duration("timeout", 30*time.Second, "time limit of the program")
	urlstr := flag.String("url", "", "base URL of the site to read, for example a live site (default: the local test site, and the selectors are written for it)")
	lang := flag.String("l", "en", "language code of the place names (the local site uses English only)")
	zoom := flag.Float64("zoom", 12.5, "zoom level of the map")
	scale := flag.Float64("scale", 1.5, "scale of the map image")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *timeout, *urlstr, *lang, *zoom, *scale, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, verbose, visible bool, timeout time.Duration, urlstr, lang string, zoom, scale float64, args []string) error {
	// start the local test site, unless the flag -url names a site
	if urlstr == "" {
		site := testsite.New()
		defer site.Close()
		urlstr = site.URL
	}
	base := strings.TrimRight(urlstr, "/")

	// Without arguments, look up the address of this computer. The service
	// does that when the query has no address.
	if len(args) == 0 {
		args = []string{""}
	}

	// create context
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
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

	for i, ipstr := range args {
		if i != 0 {
			fmt.Fprintln(os.Stdout)
		}

		// look up the address with the standard library, because the answer is
		// JSON and needs no browser
		rec, err := lookup(ctx, base, ipstr)
		if err != nil {
			fmt.Fprintf(os.Stdout, "%s: unable to lookup: %v\n", ipstr, err)
			continue
		}
		fmt.Fprintf(
			os.Stdout,
			"%s: %s, %s, %s (%s %s) @ %f,%f %s\n  Network: %s, %s, accurate to %d km\n",
			rec.IP,
			rec.City,
			rec.Region,
			rec.Country,
			rec.CountryCode,
			rec.Timezone,
			rec.Latitude,
			rec.Longitude,
			emojiFlag(rec.CountryCode),
			rec.Organization,
			rec.Range,
			rec.AccuracyKM,
		)

		// show the map of the place
		img, err := getMap(ctx, base, lang, rec.Latitude, rec.Longitude, zoom, scale)
		if err != nil {
			fmt.Fprintf(os.Stdout, "unable to get map: %v\n", err)
			continue
		}
		if err := rasterm.Encode(os.Stdout, img); err != nil {
			return fmt.Errorf("showing the map: %w", err)
		}
	}
	return nil
}

// lookup asks the lookup service for the place of an address. The service
// answers 404 for an address that it does not know.
func lookup(ctx context.Context, base, ipstr string) (*record, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/geoip?ip="+url.QueryEscape(ipstr), nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the service answered %s", res.Status)
	}
	var rec record
	if err := json.NewDecoder(res.Body).Decode(&rec); err != nil {
		return nil, fmt.Errorf("decoding the answer: %w", err)
	}
	return &rec, nil
}

// getMap opens the map of a place, waits until the map has its tiles, and
// returns a screenshot of the map.
func getMap(ctx context.Context, base, lang string, lat, lon, zoom, scale float64) (image.Image, error) {
	mapURL := fmt.Sprintf("%s/map?lat=%f&lon=%f&zoom=%.2f&hl=%s", base, lat, lon, zoom, url.QueryEscape(lang))
	// The map counts the tiles that it still loads in the attribute
	// data-loading. It is 0 when the map is ready, so wait for that.
	if err := chromedp.Do(ctx,
		chromedp.Navigate(mapURL),
		chromedp.Query(`#map[data-loading="0"]`, chromedp.NodeVisible),
	); err != nil {
		return nil, fmt.Errorf("loading the map: %w", err)
	}
	// The element #coords says where the map is.
	where, err := chromedp.Run(ctx, chromedp.Text(`#coords`))
	if err != nil {
		return nil, fmt.Errorf("reading the position of the map: %w", err)
	}
	buf, err := chromedp.Run(ctx, chromedp.ScreenshotScale(`#app-container`, scale))
	if err != nil {
		return nil, fmt.Errorf("taking the screenshot of the map: %w", err)
	}
	fmt.Fprintf(os.Stdout, "  Map: %s\n", where)
	return png.Decode(bytes.NewReader(buf))
}

// emojiFlag returns the flag of a country from its two letter code.
func emojiFlag(code string) string {
	if len(code) != 2 {
		return ""
	}
	return string(0x1f1e6+rune(code[0])-'A') + string(0x1f1e6+rune(code[1])-'A')
}
