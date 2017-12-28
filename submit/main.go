// Command submit is a chromedp example demonstrating how to fill out and
// submit a form.
package main

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := chromedp.New(ctxt, chromedp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var res string
	err = c.Run(ctxt, submit(`https://github.com/search`, `//input[@name="q"]`, `chromedp`, &res))
	if err != nil {
		log.Fatal(err)
	}

	// shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("got: `%s`", res)
}

func submit(urlstr, sel, q string, res *string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(sel),
		chromedp.SendKeys(sel, q),
		chromedp.Submit(sel),
		chromedp.WaitNotPresent(`//*[@id="code_search"]/h2/svg`),
		chromedp.Text(`//*[@id="js-pjax-container"]/div[2]/div/div[2]/ul/li/p`, res),
	}
}
