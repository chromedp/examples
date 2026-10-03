// Command visible is a chromedp example demonstrating how to wait until an
// element is visible. It starts a local server and needs no internet. Use -v to
// print the protocol messages and -visible to show the browser window and leave
// it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
)

func main() {
	port := flag.Int("port", 8544, "port")
	verbose := flag.Bool("v", false, "verbose")
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
		opts = append(opts, chromedp.WithVisibleWindow(), chromedp.WithKeepOpen())
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
	err := waitBoxes(ctx, fmt.Sprintf("http://localhost:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
}

// waitBoxes loads the page and waits for the elements box1 and box2. A script on
// the page shows box1 after 3 seconds.
func waitBoxes(ctx context.Context, host string) error {
	return chromedp.Do(ctx,
		chromedp.Navigate(host),
		chromedp.Evaluate[chromedp.Void](makeVisibleScript),
		chromedp.Func(func(context.Context, *chromedp.Target) error {
			log.Printf("waiting 3s for box to become visible")
			return nil
		}),
		chromedp.WaitVisible(chromedp.ID("box1")),
		chromedp.Func(func(context.Context, *chromedp.Target) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX1 IS VISIBLE")
			return nil
		}),
		chromedp.WaitVisible(chromedp.ID("box2")),
		chromedp.Func(func(context.Context, *chromedp.Target) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX2 IS VISIBLE")
			return nil
		}),
	)
}

const (
	makeVisibleScript = `setTimeout(function() {
	document.querySelector('#box1').style.display = '';
}, 3000);`
)

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
    <h2>box3</h3>
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
