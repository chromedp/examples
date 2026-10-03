// Command keys is a chromedp example demonstrating how to send key events to an
// element. It starts a local server and needs no internet. Use -v to print the
// protocol messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	port := flag.Int("port", 8544, "port of the local web server")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	go testServer(fmt.Sprintf(":%d", *port))

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

	// run the actions
	val1, val2, val3, val4, err := sendkeys(ctx, fmt.Sprintf("http://localhost:%d", *port))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("#input1 value: %s", val1)
	log.Printf("#textarea1 value: %s", val2)
	log.Printf("#input2 value: %s", val3)
	log.Printf("#select1 value: %s", val4)
}

// sendkeys sends keys to the page of the server and returns four values from
// the page.
func sendkeys(ctx context.Context, host string) (val1, val2, val3, val4 string, err error) {
	if err = chromedp.Do(ctx,
		chromedp.Navigate(host),
		chromedp.WaitVisible(chromedp.ID("input1")),
		chromedp.WaitVisible(chromedp.ID("textarea1")),
		chromedp.SendKeys(chromedp.ID("textarea1"), kb.End+"\b\b\n\naoeu\n\ntest1\n\nblah2\n\n\t\t\t\b\bother box!\t\ntest4"),
	); err != nil {
		return "", "", "", "", err
	}
	if val1, err = chromedp.Run(ctx, chromedp.Value(chromedp.ID("input1"))); err != nil {
		return "", "", "", "", err
	}
	if val2, err = chromedp.Run(ctx, chromedp.Value(chromedp.ID("textarea1"))); err != nil {
		return "", "", "", "", err
	}
	if err = chromedp.Do(ctx, chromedp.SetValue(chromedp.ID("input2"), "test3")); err != nil {
		return "", "", "", "", err
	}
	if val3, err = chromedp.Run(ctx, chromedp.Value(chromedp.ID("input2"))); err != nil {
		return "", "", "", "", err
	}
	if err = chromedp.Do(ctx, chromedp.SendKeys(chromedp.ID("select1"), kb.ArrowDown+kb.ArrowDown)); err != nil {
		return "", "", "", "", err
	}
	if val4, err = chromedp.Run(ctx, chromedp.Value(chromedp.ID("select1"))); err != nil {
		return "", "", "", "", err
	}
	return val1, val2, val3, val4, nil
}

// testServer serves a static HTML page.
func testServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(res, indexHTML)
	})
	return http.ListenAndServe(addr, mux)
}

const indexHTML = `<!doctype html>
<html>
<head>
  <title>example</title>
</head>
<body>
  <div id="box1" style="display:none">
    <div id="box2">
      <p>box2</p>
    </div>
  </div>
  <div id="box3">
    <h2>box3</h2>
    <p id="box4">
      box4 text
      <input id="input1" value="some value"><br><br>
      <textarea id="textarea1" style="width:500px;height:400px">textarea</textarea><br><br>
      <input id="input2" type="submit" value="Next">
      <select id="select1">
        <option value="one">1</option>
        <option value="two">2</option>
        <option value="three">3</option>
        <option value="four">4</option>
      </select>
    </p>
  </div>
</body>
</html>`
