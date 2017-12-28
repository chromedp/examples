// Command keys is a chromedp example demonstrating how to send key events to
// an element.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
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
	var val1, val2, val3, val4 string
	err = c.Run(ctxt, sendkeys(&val1, &val2, &val3, &val4))
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

	log.Printf("#input1 value: %s", val1)
	log.Printf("#textarea1 value: %s", val2)
	log.Printf("#input2 value: %s", val3)
	log.Printf("#select1 value: %s", val4)
}

func sendkeys(val1, val2, val3, val4 *string) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate("file:" + os.Getenv("GOPATH") + "/src/github.com/chromedp/chromedp/testdata/visible.html"),
		chromedp.WaitVisible(`#input1`, chromedp.ByID),
		chromedp.WaitVisible(`#textarea1`, chromedp.ByID),
		chromedp.SendKeys(`#textarea1`, kb.End+"\b\b\n\naoeu\n\ntest1\n\nblah2\n\n\t\t\t\b\bother box!\t\ntest4", chromedp.ByID),
		chromedp.Value(`#input1`, val1, chromedp.ByID),
		chromedp.Value(`#textarea1`, val2, chromedp.ByID),
		chromedp.SetValue(`#input2`, "test3", chromedp.ByID),
		chromedp.Value(`#input2`, val3, chromedp.ByID),
		chromedp.SendKeys(`#select1`, kb.ArrowDown+kb.ArrowDown, chromedp.ByID),
		chromedp.Value(`#select1`, val4, chromedp.ByID),
		chromedp.Sleep(30 * time.Second),
	}
}
