// Command upload is a chromedp example demonstrating how to upload a file on a
// form.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/chromedp/chromedp"
)

func main() {
	port := flag.Int("port", 8544, "port")
	flag.Parse()

	// get wd
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	filepath := wd + "/main.go"

	// get some info about the file
	fi, err := os.Stat(filepath)
	if err != nil {
		log.Fatal(err)
	}

	// start upload server
	result := make(chan int, 1)
	go uploadServer(fmt.Sprintf(":%d", *port), result)

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// run the steps
	_, err = upload(ctx, fmt.Sprintf("http://localhost:%d", *port), filepath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("original size: %d, upload size: %d", fi.Size(), <-result)
}

// upload uploads the file on the form and returns the size that the page shows.
func upload(ctx context.Context, urlstr string, filepath string) (string, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.SendKeys(`input[name="upload"]`, filepath, chromedp.NodeVisible),
		chromedp.Click(`input[name="submit"]`),
	); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(chromedp.ID("result"), chromedp.NodeVisible))
}

func uploadServer(addr string, result chan int) error {
	// create http server and result channel
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(res, uploadHTML)
	})
	mux.HandleFunc("/upload", func(res http.ResponseWriter, req *http.Request) {
		f, _, err := req.FormFile("upload")
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()

		buf, err := io.ReadAll(f)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Fprintf(res, resultHTML, len(buf))

		result <- len(buf)
	})
	return http.ListenAndServe(addr, mux)
}

const (
	uploadHTML = `<!doctype html>
<html>
<body>
  <form method="POST" action="/upload" enctype="multipart/form-data">
    <input name="upload" type="file"/>
    <input name="submit" type="submit"/>
  </form>
</body>
</html>`

	resultHTML = `<!doctype html>
<html>
<body>
  <div id="result">%d</div>
</body>
</html>`
)
