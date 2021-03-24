// Command pdf is a chromedp example demonstrating how to capture a pdf of a
// page.
package main

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// capture pdf
	var buf []byte
	if err := chromedp.Run(ctx, printToPDF(`https://www.google.com/`, &buf)); err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile("sample.pdf", buf, 0644); err != nil {
		log.Fatal(err)
	}
}

// print a specific pdf page.
func printToPDF(urlstr string, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(false).Do(ctx)
			if err != nil {
				return err
			}
			*res = buf
			return nil
		}),
	}
}
