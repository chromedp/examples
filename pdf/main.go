// Command pdf is a chromedp example demonstrating how to capture a PDF of a
// page. It reads www.google.com and writes sample.pdf to the current directory.
// Use -v to print the protocol messages and -visible to show the browser window
// and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

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

	// capture the PDF
	buf, err := printToPDF(ctx, `https://www.google.com/`)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("sample.pdf", buf, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote sample.pdf")
}

// printToPDF navigates to urlstr and returns the page as a PDF.
func printToPDF(ctx context.Context, urlstr string) ([]byte, error) {
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return nil, err
	}
	return chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, page.PrintToPDF, page.PrintToPDFParams{PrintBackground: new(false)})
		return res.Data, err
	})
}
