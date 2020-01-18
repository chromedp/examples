// Command keys is a chromedp example demonstrating how to send key events to
// an element.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

var flagPort = flag.Int("port", 8544, "port")

func main() {
	flag.Parse()

	// run server
	go testServer(fmt.Sprintf(":%d", *flagPort))

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run task list
	var val1, val2, val3, val4 string
	err := chromedp.Run(ctx, sendkeys(fmt.Sprintf("http://localhost:%d", *flagPort), &val1, &val2, &val3, &val4))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("#input1 value: %s", val1)
	log.Printf("#textarea1 value: %s", val2)
	log.Printf("#input2 value: %s", val3)
	log.Printf("#select1 value: %s", val4)
}

// sendkeys sends keys to the server and extracts 4 values from the html page.
func sendkeys(host string, val1, val2, val3, val4 *string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(host),
		chromedp.WaitVisible(`#input1`, chromedp.ByID),
		chromedp.WaitVisible(`#textarea1`, chromedp.ByID),
		chromedp.SendKeys(`#textarea1`, kb.End+"\b\b\n\naoeu\n\ntest1\n\nblah2\n\n\t\t\t\b\bother box!\t\ntest4", chromedp.ByID),
		chromedp.Value(`#input1`, val1, chromedp.ByID),
		chromedp.Value(`#textarea1`, val2, chromedp.ByID),
		chromedp.SetValue(`#input2`, "test3", chromedp.ByID),
		chromedp.Value(`#input2`, val3, chromedp.ByID),
		chromedp.SendKeys(`#select1`, kb.ArrowDown+kb.ArrowDown, chromedp.ByID),
		chromedp.Value(`#select1`, val4, chromedp.ByID),
	}
}

// testServer is a simple HTTP server that displays the passed headers in the html.
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
