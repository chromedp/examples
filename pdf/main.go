// Command pdf is a chromedp example demonstrating how to capture a PDF of a
// page. It reads www.google.com.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

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
