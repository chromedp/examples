// Command forecast is a chromedp example demonstrating how to render the weather
// forecast of a city in the terminal. It starts a local server and needs no
// internet, but it needs a terminal that can show images. Use the flag -q to
// name the place, for example -q Tokyo. The program clicks the unit, the tab
// and the day, prints what the forecast says, and draws the forecast. Use -v to
// print the protocol messages and -visible to show the browser window and leave
// it open. The flag -url reads another site instead. The selectors are written
// for the local site, so a live site can differ.
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

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/kenshaw/rasterm"
)

// widgetSel selects the header with the search form and the data block of the
// forecast. A screenshot of both nodes shows the whole widget.
const widgetSel = `#taw, #wob_wc`

// hideMenuJS hides the menu bar of the local site and scrolls to the top.
const hideMenuJS = `document.querySelector('.site-header')?.style.setProperty('display', 'none'); window.scrollTo(0, 0)`

// summary is what the forecast says about the chosen day.
type summary struct {
	Place     string   `json:"place"`
	When      string   `json:"when"`
	Condition string   `json:"condition"`
	Temp      string   `json:"temp"`
	Unit      string   `json:"unit"`
	Details   []string `json:"details"`
}

// summaryJS reads the summary of one day from the page. It uses innerText, so
// it skips the parts that the style sheet hides, such as the other unit.
const summaryJS = `(function(day) {
  const root = document.querySelector('#wob_wc');
  const sum = root.querySelector('.wob-sum[data-day="' + day + '"]');
  const where = sum.querySelector('.wob-where').children;
  return {
    place: where[0].innerText.trim(),
    when: where[1].innerText.trim(),
    condition: where[2].innerText.trim(),
    temp: sum.querySelector('.wob-big').innerText.trim(),
    unit: root.dataset.unit.toUpperCase(),
    details: Array.from(sum.querySelectorAll('.wob-details li')).map(li => li.innerText.trim()),
  };
})(%d)`

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open (no effect with -remote)")
	timeout := flag.Duration("timeout", 30*time.Second, "time limit of the program")
	urlstr := flag.String("url", "", "base URL of the site to read, for example a live site (default: the local test site, and the selectors are written for it)")
	query := flag.String("q", "", "place to show the weather for (default jakarta)")
	lang := flag.String("hl", "", "language code (see hl.json)")
	unit := flag.String("unit", "", "temperature unit (C, F, or blank)")
	typ := flag.String("type", "", "kind of forecast (temp, rain, wind)")
	day := flag.Int("day", 0, "day of the forecast (0 to 7)")
	scale := flag.Float64("scale", 1.5, "scale of the screenshot")
	padding := flag.Int("padding", 20, "white space around the image, in pixels")
	remoteURL := flag.String("remote", "", "WebSocket URL of a running browser to use")
	out := flag.String("out", "", "file to write the screenshot to")
	flag.Parse()
	if err := run(context.Background(), *verbose, *visible, *timeout, *urlstr, *query, *lang, *unit, *typ, *day, *scale, *padding, *remoteURL, *out); err != nil {
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

func run(ctx context.Context, verbose, visible bool, timeout time.Duration, urlstr, query, lang, unit, typ string, day int, scale float64, padding int, remoteURL, out string) error {
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
	if query = strings.TrimSpace(query); query == "" {
		query = "jakarta"
	}

	// start the local test site, unless the flag -url names a site
	if urlstr == "" {
		site := testsite.New()
		defer site.Close()
		urlstr = site.URL
	}

	// build the address of the search. The site sends the browser on to the
	// page of the city and keeps the other parameters.
	v := make(url.Values)
	v.Set("q", query)
	if lang != "" {
		v.Set("hl", lang)
	}
	pageURL := strings.TrimRight(urlstr, "/") + "/weather?" + v.Encode()

	// if the flag -remote is set, use a remote browser
	if remoteURL != "" {
		ctx, _ = remote.NewAllocator(ctx, remoteURL)
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
	if visible && remoteURL == "" {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	// open the search page, and wait for the forecast. A place that the site
	// does not know gives a list of cities and no forecast.
	if err := chromedp.Do(ctx,
		chromedp.Navigate(pageURL),
		chromedp.Query(chromedp.ID("wob_wc"), chromedp.NodeVisible),
	); err != nil {
		return fmt.Errorf("loading the forecast of %q from %s: %w", query, pageURL, err)
	}

	// Each control of the page sets an attribute of the data block. After a
	// click, wait for the new value, so that the next step sees the new state.
	click := func(control, attr, value string) error {
		return chromedp.Do(ctx,
			chromedp.Click(control),
			chromedp.Query(fmt.Sprintf(`#wob_wc[%s=%q]`, attr, value), chromedp.NodeVisible),
		)
	}

	// click on unit
	if unit != "" {
		if err := click("#wob_unit_"+strings.ToLower(unit), "data-unit", strings.ToLower(unit)); err != nil {
			return fmt.Errorf("clicking on the unit %s: %w", unit, err)
		}
	}

	// click on type
	if typ != "temp" {
		if err := click("#wob_"+typ, "data-type", typ); err != nil {
			return fmt.Errorf("clicking on the type %s: %w", typ, err)
		}
	}

	// click on day
	if day != 0 {
		if err := click(fmt.Sprintf(`.wob-days button[data-wob-di="%d"]`, day), "data-day", fmt.Sprint(day)); err != nil {
			return fmt.Errorf("clicking on day %d: %w", day, err)
		}
	}

	// print what the forecast says
	sum, err := chromedp.Run(ctx, chromedp.Evaluate[summary](fmt.Sprintf(summaryJS, day)))
	if err != nil {
		return fmt.Errorf("reading the forecast: %w", err)
	}
	fmt.Printf("%s, %s, %s\n", sum.Place, sum.When, sum.Condition)
	fmt.Printf("temperature: %s °%s\n", sum.Temp, sum.Unit)
	for _, detail := range sum.Details {
		fmt.Println(detail)
	}

	// Capture the screenshot of the header and the data block. The inactive
	// tabs and days are hidden by the page, so the screenshot shows the chart
	// of the chosen type and day. The menu bar of the site sticks to the top of
	// the window and covers a part of the forecast, so hide it and
	// scroll to the top first.
	if err := chromedp.Do(ctx, chromedp.Evaluate[chromedp.Void](hideMenuJS)); err != nil {
		return fmt.Errorf("hiding the menu bar: %w", err)
	}
	buf, err := chromedp.Run(ctx, chromedp.ScreenshotScale(chromedp.CSSAll(widgetSel), scale))
	if err != nil {
		return fmt.Errorf("taking the screenshot: %w", err)
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
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		draw.Draw(dst, image.Rect(padding, padding, padding+w, padding+h), img, bounds.Min, draw.Src)
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

var langs map[string]string

func init() {
	if err := json.Unmarshal(hlJSON, &langs); err != nil {
		panic(err)
	}
}

//go:embed hl.json
var hlJSON []byte
