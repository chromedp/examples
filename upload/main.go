// Command upload is a chromedp example demonstrating how to upload a file on a
// form. It starts a local server and needs no internet. The program uploads its
// own source file, so it works from any directory. Use -v to print the protocol
// messages, -visible to show the browser window and leave it open, and
// -visible-on-terminal to draw the page in the terminal with terminal graphics.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

func main() {
	port := flag.Int("port", 8544, "port of the local web server")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// find the source file of this program. The path comes from the build, so
	// the program does not depend on the working directory
	_, filepath, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("could not find the source file")
	}

	// get some info about the file
	fi, err := os.Stat(filepath)
	if err != nil {
		log.Fatal(err)
	}

	// start the upload server
	result := make(chan int, 1)
	go uploadServer(fmt.Sprintf(":%d", *port), result)

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

	// start the stream before the first navigation, so that the frames show
	// the page while it loads
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())

	// run the actions
	_, err = upload(ctx, fmt.Sprintf("http://localhost:%d", *port), filepath)
	if err != nil {
		s.Fatal(err)
	}

	log.Printf("original size: %d, upload size: %d", fi.Size(), <-result)
}

// upload sends the file with the form of the page and returns the size that
// the page shows.
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

// uploadServer serves the upload form. It sends the size of the uploaded file to
// result.
func uploadServer(addr string, result chan int) error {
	// create the HTTP server
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
