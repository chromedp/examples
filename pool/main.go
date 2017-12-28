// Command pool is a chromedp example demonstrating how to use chromedp pool.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create pool
	pool, err := chromedp.NewPool( /*chromedp.PoolLog(log.Printf, log.Printf, log.Printf)*/ )
	if err != nil {
		log.Fatal(err)
	}

	// loop over the URLs
	var wg sync.WaitGroup
	for i, urlstr := range []string{
		"https://brank.as/",
		"https://brank.as/careers",
		"https://brank.as/about",
	} {
		wg.Add(1)
		go takeScreenshot(ctxt, &wg, pool, i, urlstr)
	}

	// wait for to finish
	wg.Wait()

	// shutdown pool
	err = pool.Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}

func takeScreenshot(ctxt context.Context, wg *sync.WaitGroup, pool *chromedp.Pool, id int, urlstr string) {
	defer wg.Done()

	// allocate
	c, err := pool.Allocate(ctxt)
	if err != nil {
		log.Printf("url (%d) `%s` error: %v", id, urlstr, err)
		return
	}
	defer c.Release()

	// run tasks
	var buf []byte
	err = c.Run(ctxt, screenshot(urlstr, &buf))
	if err != nil {
		log.Printf("url (%d) `%s` error: %v", id, urlstr, err)
		return
	}

	// write to disk
	err = ioutil.WriteFile(fmt.Sprintf("%d.png", id), buf, 0644)
	if err != nil {
		log.Printf("url (%d) `%s` error: %v", id, urlstr, err)
		return
	}
}

func screenshot(urlstr string, picbuf *[]byte) chromedp.Action {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.Sleep(2 * time.Second),
		chromedp.WaitVisible(`#navbar-nav-main`),
		chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
			buf, err := page.CaptureScreenshot().Do(ctxt, h)
			if err != nil {
				return err
			}
			*picbuf = buf
			return nil
		}),
	}
}
