// Command geoip is a chromedp example demonstrating how to look up the location
// of an IP address and show its map in the terminal. It looks up the address
// offline in the embedded GeoLite2 City database of MaxMind, and it takes a
// screenshot of the map of the place from the local test site. It needs no
// internet, but it needs a terminal that can show images. Give one or more IP
// addresses as arguments, or none to look up the example address 8.8.8.8. Use
// -l to choose the language of the place names, -zoom and -scale to change the
// map, -v to print the protocol messages, -visible to show the browser window
// and leave it open, and -visible-on-terminal to draw the page in the terminal
// with terminal graphics. The flag -url reads the map from another site instead
// of the local test site. The tiles and the selectors are written for the local
// site, so a live site can differ.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
	"github.com/kenshaw/rasterm"
	"github.com/oschwald/geoip2-golang"
)

// defaultIP is the address that the program looks up when it has no argument.
const defaultIP = "8.8.8.8"

// geoLite2CityMmdb is the GeoLite2 City database of MaxMind.
//
//go:embed GeoLite2-City.mmdb
var geoLite2CityMmdb []byte

func main() {
	var tc termcast.Flags
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	timeout := flag.Duration("timeout", 30*time.Second, "time limit of the program")
	urlstr := flag.String("url", "", "base URL of the map site to read (default: the local test site, and the tiles and selectors are written for it)")
	lang := flag.String("l", "en", "language code of the place names, for example de, es, fr, ja, pt-BR, ru or zh-CN")
	zoom := flag.Float64("zoom", 12.5, "zoom level of the map")
	scale := flag.Float64("scale", 1.5, "scale of the map image")
	tc.Register(flag.CommandLine)
	flag.Parse()
	if err := run(context.Background(), &tc, *verbose, *visible, *timeout, *urlstr, *lang, *zoom, *scale, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, tc *termcast.Flags, verbose, visible bool, timeout time.Duration, urlstr, lang string, zoom, scale float64, args []string) error {
	// open the embedded database
	db, err := geoip2.FromBytes(geoLite2CityMmdb)
	if err != nil {
		return fmt.Errorf("opening the database: %w", err)
	}
	defer db.Close()

	// start the local test site, unless the flag -url names a site
	if urlstr == "" {
		site := testsite.New()
		defer site.Close()
		urlstr = site.URL
	}
	base := strings.TrimRight(urlstr, "/")

	// without arguments, look up the example address
	if len(args) == 0 {
		args = []string{defaultIP}
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

	// draw the page in the terminal if the flag -visible-on-terminal is set
	s, err := tc.Start(ctx, verbose)
	if err != nil {
		return err
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	stdout := io.Writer(os.Stdout)
	if s != nil {
		stdout = s.LogWriter()
	}

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	for i, ipstr := range args {
		if i != 0 {
			fmt.Fprintln(stdout)
		}
		ip := net.ParseIP(ipstr)
		if ip == nil {
			fmt.Fprintf(stdout, "%s: unable to lookup: not an IP address\n", ipstr)
			continue
		}
		record, err := db.City(ip)
		if err != nil {
			fmt.Fprintf(stdout, "%s: unable to lookup: %v\n", ipstr, err)
			continue
		}
		loc := record.Location
		if loc.Latitude == 0 && loc.Longitude == 0 {
			fmt.Fprintf(stdout, "%s: unable to lookup: the database has no location for it\n", ipstr)
			continue
		}
		fmt.Fprintf(stdout, "%s: %s\n", ipstr, describe(record, lang))

		// show the map of the place
		img, err := getMap(ctx, base, lang, loc.Latitude, loc.Longitude, zoom, scale)
		if err != nil {
			fmt.Fprintf(stdout, "unable to get map: %v\n", err)
			continue
		}
		// show the map after the stream has stopped
		s.Stop()
		if err := rasterm.Encode(os.Stdout, img); err != nil {
			return fmt.Errorf("showing the map: %w", err)
		}
	}
	return nil
}

// describe returns the place of a record as one line, with the names in the
// language lang. A name that the database does not have in that language falls
// back to English.
func describe(record *geoip2.City, lang string) string {
	var parts []string
	if name := localName(record.City.Names, lang); name != "" {
		parts = append(parts, name)
	}
	for _, s := range record.Subdivisions {
		if name := localName(s.Names, lang); name != "" {
			parts = append(parts, name)
		}
	}
	country := localName(record.Country.Names, lang)
	if country == "" {
		country = localName(record.RegisteredCountry.Names, lang)
	}
	if country != "" {
		parts = append(parts, country)
	}
	s := strings.Join(parts, ", ")
	if code := record.Country.IsoCode; code != "" {
		s += " (" + code
		if tz := record.Location.TimeZone; tz != "" {
			s += " " + tz
		}
		s += ")"
	}
	s += fmt.Sprintf(" @ %f,%f", record.Location.Latitude, record.Location.Longitude)
	if r := record.Location.AccuracyRadius; r != 0 {
		s += fmt.Sprintf(", accurate to %d km", r)
	}
	if emoji := emojiFlag(record.Country.IsoCode); emoji != "" {
		s += " " + emoji
	}
	return s
}

// localName returns the name for the language lang, or the English name if
// there is none.
func localName(names map[string]string, lang string) string {
	if name, ok := names[lang]; ok {
		return name
	}
	return names["en"]
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
	buf, err := chromedp.Run(ctx, chromedp.ScreenshotScale(`#app-container`, scale))
	if err != nil {
		return nil, fmt.Errorf("taking the screenshot of the map: %w", err)
	}
	return png.Decode(bytes.NewReader(buf))
}

// emojiFlag returns the flag of a country from its two letter code.
func emojiFlag(code string) string {
	if len(code) != 2 {
		return ""
	}
	return string(0x1f1e6+rune(code[0])-'A') + string(0x1f1e6+rune(code[1])-'A')
}
