// Command rawcall is a chromedp example demonstrating how to send protocol
// commands that chromedp has no action for. A protocol command is a value of
// the type cdp.Command, and cdp.Call or chromedp.Call sends it to a target.
// chromedp.CallBrowser sends a command to the browser. The program overrides
// the time zone, the locale and the language of a page, sets its geolocation,
// throttles the CPU, changes the network conditions, and grants a permission to
// an origin. After each command it reads the effect from the page with
// chromedp.Evaluate. It also writes a command value by hand, for a command that
// the generated package cdproto does not have. It starts a local server and needs no
// internet. Use -v to print the protocol messages and -visible to show the
// browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// locale is what the page reports about its time zone and its language.
type locale struct {
	TimeZone string `json:"timeZone"`
	Offset   int    `json:"offset"`
	Language string `json:"language"`
	Number   string `json:"number"`
	Date     string `json:"date"`
}

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		log.Fatal(err)
	}
	if err := emulate(ctx); err != nil {
		log.Fatal(err)
	}
	if err := position(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
	if err := load(ctx); err != nil {
		log.Fatal(err)
	}
	if err := browserCommands(ctx); err != nil {
		log.Fatal(err)
	}
}

// emulate changes the time zone and the locale of the page.
func emulate(ctx context.Context) error {
	// To find a command, open the page of the protocol at
	// https://chromedevtools.github.io/devtools-protocol/, and look in a
	// domain, here Emulation. The command Emulation.setTimezoneOverride is the
	// value emulation.SetTimezoneOverride in cdproto. The name of the domain
	// is the package, and the name of the command starts with a capital
	// letter. The value has the JSON method name, and its type holds the
	// parameters and the result.
	fmt.Printf("the command value %s sends the method %q\n", "emulation.SetTimezoneOverride", emulation.SetTimezoneOverride.Method)

	before, err := readLocale(ctx)
	if err != nil {
		return err
	}
	// The first line shows the time zone of this computer.
	fmt.Printf("before: %+v\n", before)

	// An action receives its target, and cdp.Call sends the command to it.
	// The type of the parameters is the first type of the command, and the
	// fields of the struct are the parameters of the protocol.
	setZone := chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, emulation.SetTimezoneOverride, emulation.SetTimezoneOverrideParams{TimezoneID: "America/New_York"})
		return err
	})
	if err := chromedp.Do(ctx, setZone); err != nil {
		return fmt.Errorf("overriding the time zone: %w", err)
	}

	// Outside an action, chromedp.Call sends the command to the target of
	// the context.
	if _, err := chromedp.Call(ctx, emulation.SetLocaleOverride, emulation.SetLocaleOverrideParams{Locale: "id_ID"}); err != nil {
		return fmt.Errorf("overriding the locale: %w", err)
	}
	after, err := readLocale(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("after:  %+v\n", after)

	// The locale override changes the formats of Intl, but it does not change
	// navigator.language. The language of the browser comes from the user
	// agent override, which needs the user agent text too, so read it from
	// the browser first.
	version, err := chromedp.CallBrowser(ctx, browser.GetVersion, cdp.Empty{})
	if err != nil {
		return fmt.Errorf("reading the user agent: %w", err)
	}
	if _, err := chromedp.Call(ctx, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{
		UserAgent:      version.UserAgent,
		AcceptLanguage: "id-ID",
	}); err != nil {
		return fmt.Errorf("overriding the language: %w", err)
	}
	language, err := chromedp.Run(ctx, chromedp.Evaluate[string](`navigator.language`))
	if err != nil {
		return fmt.Errorf("reading the language: %w", err)
	}
	fmt.Printf("navigator.language after the user agent override: %s\n", language)
	return nil
}

// readLocale asks the page for its time zone, its language and its formats.
func readLocale(ctx context.Context) (locale, error) {
	l, err := chromedp.Run(ctx, chromedp.Evaluate[locale](`({
  timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  offset: new Date(0).getTimezoneOffset(),
  language: navigator.language,
  number: (1234567.891).toLocaleString(),
  date: new Date(Date.UTC(2026, 9, 4, 12)).toLocaleString(),
})`))
	if err != nil {
		return locale{}, fmt.Errorf("reading the locale of the page: %w", err)
	}
	return l, nil
}

// position sets the geolocation, and asks the page for it.
func position(ctx context.Context, host string) error {
	// A page can use the geolocation only when the user grants the
	// permission. The command Browser.setPermission is a command of the
	// browser, so the program sends it with chromedp.CallBrowser. This
	// command takes the place of the old Browser.grantPermissions, which
	// cdproto does not have.
	if _, err := chromedp.CallBrowser(ctx, browser.SetPermission, browser.SetPermissionParams{
		Permission: &browser.PermissionDescriptor{Name: "geolocation"},
		Setting:    browser.PermissionSettingGranted,
		Origin:     host,
	}); err != nil {
		return fmt.Errorf("granting the permission: %w", err)
	}
	state, err := chromedp.Run(ctx, chromedp.Evaluate[string](`navigator.permissions.query({name: "geolocation"}).then(p => p.state)`, awaitPromise))
	if err != nil {
		return fmt.Errorf("reading the permission: %w", err)
	}
	fmt.Printf("permission geolocation: %s\n", state)

	// The numbers of the command are pointers, because 0 is a valid value
	// and a field that the program leaves out is not sent.
	if _, err := chromedp.Call(ctx, emulation.SetGeolocationOverride, emulation.SetGeolocationOverrideParams{
		Latitude:  new(-6.2088),
		Longitude: new(106.8456),
		Accuracy:  new(25.0),
	}); err != nil {
		return fmt.Errorf("overriding the geolocation: %w", err)
	}
	coords, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]float64](`new Promise((resolve, reject) => {
  navigator.geolocation.getCurrentPosition(
    p => resolve({latitude: p.coords.latitude, longitude: p.coords.longitude, accuracy: p.coords.accuracy}),
    e => reject(new Error(e.message)),
  );
})`, awaitPromise))
	if err != nil {
		return fmt.Errorf("reading the position: %w", err)
	}
	fmt.Printf("position: %v\n", coords)
	return nil
}

// awaitPromise makes Evaluate wait for a promise to settle.
func awaitPromise(p *runtime.EvaluateParams) { p.AwaitPromise = new(true) }

// load slows the CPU and the network, and measures the effect.
func load(ctx context.Context) error {
	const work = `(() => {
  const start = performance.now();
  let sum = 0;
  for (let i = 0; i < 2e7; i++) sum += i;
  return performance.now() - start;
})()`
	normal, err := chromedp.Run(ctx, chromedp.Evaluate[float64](work))
	if err != nil {
		return fmt.Errorf("measuring the CPU: %w", err)
	}

	// A rate of 4 makes the CPU four times slower.
	if _, err := chromedp.Call(ctx, emulation.SetCPUThrottlingRate, emulation.SetCPUThrottlingRateParams{Rate: 4}); err != nil {
		return fmt.Errorf("throttling the CPU: %w", err)
	}
	slow, err := chromedp.Run(ctx, chromedp.Evaluate[float64](work))
	if err != nil {
		return fmt.Errorf("measuring the throttled CPU: %w", err)
	}
	fmt.Printf("CPU throttling rate 4: the loop took %.1f times as long\n", slow/normal)
	if _, err := chromedp.Call(ctx, emulation.SetCPUThrottlingRate, emulation.SetCPUThrottlingRateParams{Rate: 1}); err != nil {
		return fmt.Errorf("resetting the CPU: %w", err)
	}

	// The command Network.emulateNetworkConditions is not in cdproto. Its
	// successor sets rules for URL patterns. An empty pattern matches every
	// request. A throughput of -1 means no limit.
	const ping = `fetch("/ping").then(r => r.text()).then(() => "ok", e => e.message)`
	fetchTime := `(async () => {
  const start = performance.now();
  const result = await ` + ping + `;
  return {result, ms: performance.now() - start};
})()`
	type timing struct {
		Result string  `json:"result"`
		MS     float64 `json:"ms"`
	}
	conditions := func(c network.Conditions) error {
		_, err := chromedp.Call(ctx, network.EmulateNetworkConditionsByRule, network.EmulateNetworkConditionsByRuleParams{
			MatchedNetworkConditions: []*network.Conditions{&c},
		})
		return err
	}
	if err := conditions(network.Conditions{Latency: 400, DownloadThroughput: -1, UploadThroughput: -1}); err != nil {
		return fmt.Errorf("adding latency: %w", err)
	}
	slowFetch, err := chromedp.Run(ctx, chromedp.Evaluate[timing](fetchTime, awaitPromise))
	if err != nil {
		return fmt.Errorf("timing the slow request: %w", err)
	}
	fmt.Printf("network latency 400 ms: the request gave %q after at least 400 ms: %t\n", slowFetch.Result, slowFetch.MS >= 400)

	if err := conditions(network.Conditions{Offline: true, DownloadThroughput: -1, UploadThroughput: -1}); err != nil {
		return fmt.Errorf("going offline: %w", err)
	}
	offline, err := chromedp.Run(ctx, chromedp.Evaluate[timing](fetchTime, awaitPromise))
	if err != nil {
		return fmt.Errorf("timing the offline request: %w", err)
	}
	fmt.Printf("network offline: the request gave %q\n", offline.Result)

	// A later call replaces the rules of the earlier call, so restore the
	// normal conditions.
	if err := conditions(network.Conditions{DownloadThroughput: -1, UploadThroughput: -1}); err != nil {
		return fmt.Errorf("restoring the network: %w", err)
	}
	return nil
}

// browserCommands sends commands to the browser, and one command that the
// program defines itself.
func browserCommands(ctx context.Context) error {
	version, err := chromedp.CallBrowser(ctx, browser.GetVersion, cdp.Empty{})
	if err != nil {
		return fmt.Errorf("reading the version of the browser: %w", err)
	}
	fmt.Printf("Browser.getVersion with the command of cdproto: protocol %s\n", version.ProtocolVersion)

	// A command that is new in Chrome, or that cdproto leaves out, is still
	// a value. The method is the name in the protocol, the first type is the
	// struct of the parameters, and the second type is the struct of the
	// result. The JSON tags are the names of the protocol. This struct
	// reads one field of the result and ignores the others.
	type result struct {
		Product string `json:"product"`
	}
	getVersion := cdp.Command[cdp.Empty, result]{Method: "Browser.getVersion"}
	custom, err := chromedp.CallBrowser(ctx, getVersion, cdp.Empty{})
	if err != nil {
		return fmt.Errorf("calling the command that the program defines: %w", err)
	}
	fmt.Printf("Browser.getVersion with a command that the program defines: product %s\n", custom.Product)
	return nil
}

// newMux returns the handlers of the test server. The page has no content. The
// program asks the page questions with JavaScript. The endpoint /ping answers
// at once.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>rawcall</title></head><body>rawcall</body></html>`)
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "pong")
	})
	return mux
}
