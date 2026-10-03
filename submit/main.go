// Command submit is a chromedp example demonstrating how to fill out and
// submit a form.
package main

import (
	"context"
	"log"
	"strings"

	"github.com/chromedp/chromedp"
)

func main() {
	// create context
	ctx, cancel := chromedp.NewContext(context.Background(), chromedp.WithDebugf(log.Printf))
	defer cancel()

	// run task list
	res, err := submit(ctx, `https://github.com/search`, `//input[@name="q"]`, `chromedp`)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("got: `%s`", strings.TrimSpace(res))
}

func submit(ctx context.Context, urlstr, sel, q string) (string, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(sel),
		chromedp.SendKeys(sel, q),
		chromedp.Submit(sel),
		chromedp.WaitVisible(`//*[contains(., 'repository results')]`),
	); err != nil {
		return "", err
	}
	return chromedp.Run(ctx, chromedp.Text(`(//*//ul[contains(@class, "repo-list")]/li[1]//p)[1]`))
}
