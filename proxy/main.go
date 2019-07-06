package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"
)

func main() {
	ctx := context.Background()
	options := []chromedp.ExecAllocatorOption{
		chromedp.ProxyServer("socks5://127.0.0.1:9050"),
	}
	options = append(options, chromedp.DefaultExecAllocatorOptions[:]...)

	c, cc := chromedp.NewExecAllocator(ctx, options...)
	defer cc()
	// create context
	ctx, cancel := chromedp.NewContext(c)
	defer cancel()
	var res string
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://2ip.ru"),
		chromedp.Text(`#d_clip_button`, &res, chromedp.NodeVisible, chromedp.ByID))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
