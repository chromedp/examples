// Command standalone is a chromedp example demonstrating how to use chromedp
// with a standalone (ie, a "pre-existing" / manually started) chrome instance.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/client"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome
	c, err := chromedp.New(ctxt, chromedp.WithTargets(client.New().WatchPageTargets(ctxt)), chromedp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// run task list
	var site, res string
	err = c.Run(ctxt, googleSearch("site:brank.as", "Easy Money Management", &site, &res))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("saved screenshot of #testimonials from search result listing `%s` (%s)", res, site)
}

func googleSearch(q, text string, site, res *string) chromedp.Tasks {
	var buf []byte
	sel := fmt.Sprintf(`//a[text()[contains(., '%s')]]`, text)
	return chromedp.Tasks{
		chromedp.Navigate(`https://www.google.com`),
		chromedp.Sleep(2 * time.Second),
		chromedp.WaitVisible(`#hplogo`, chromedp.ByID),
		chromedp.SendKeys(`#lst-ib`, q+"\n", chromedp.ByID),
		chromedp.WaitVisible(`#res`, chromedp.ByID),
		chromedp.Text(sel, res),
		chromedp.Click(sel),
		chromedp.Sleep(2 * time.Second),
		chromedp.WaitVisible(`#footer`, chromedp.ByQuery),
		chromedp.WaitNotVisible(`div.v-middle > div.la-ball-clip-rotate`, chromedp.ByQuery),
		chromedp.Location(site),
		chromedp.Screenshot(`#testimonials`, &buf, chromedp.ByID),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			return ioutil.WriteFile("testimonials.png", buf, 0644)
		}),
	}
}
