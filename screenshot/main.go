// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element.
package main

import (
	"context"
	"io/ioutil"
	"log"
	"time"

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
	var buf []byte
	err = c.Run(ctxt, screenshot(`https://brank.as/`, `#contact-form`, &buf))
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

	err = ioutil.WriteFile("contact-form.png", buf, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func screenshot(urlstr, sel string, res *[]byte) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.Sleep(2 * time.Second),
		chromedp.WaitVisible(sel, chromedp.ByID),
		chromedp.WaitNotVisible(`div.v-middle > div.la-ball-clip-rotate`, chromedp.ByQuery),
		chromedp.Screenshot(sel, res, chromedp.NodeVisible, chromedp.ByID),
	}
}
