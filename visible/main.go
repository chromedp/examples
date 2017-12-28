// Command visible is a chromedp example demonstrating how to wait until an
// element is visible.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
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
	err = c.Run(ctxt, visible())
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
}

func visible() chromedp.Tasks {
	var res *runtime.RemoteObject
	return chromedp.Tasks{
		chromedp.Navigate("file:" + os.Getenv("GOPATH") + "/src/github.com/chromedp/chromedp/testdata/visible.html"),
		chromedp.Evaluate(makeVisibleScript, &res),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			log.Printf(">>> res: %+v", res)
			return nil
		}),
		chromedp.WaitVisible(`#box1`),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX1 IS VISIBLE")
			return nil
		}),
		chromedp.WaitVisible(`#box2`),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> BOX2 IS VISIBLE")
			return nil
		}),
		chromedp.ActionFunc(func(context.Context, cdp.Executor) error {
			log.Printf(">>>>>>>>>>>>>>>>>>>> WAITING TO EXIT")
			time.Sleep(150 * time.Second)
			return errors.New("exiting")
		}),
	}
}

const (
	makeVisibleScript = `setTimeout(function() {
	document.querySelector('#box1').style.display = '';
}, 30000);`
)
