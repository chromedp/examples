// Command download_image is a chromedp example demonstrating how to do headless
// image downloads. It starts a local server and needs no internet. The program
// opens the page /gallery of the local test site, reads the addresses of its 24
// pictures with Evaluate, and loads the large picture gallery-01.jpg in the
// browser. It gets the bytes of the picture from the network response, checks the
// type and the size with the package image, and writes the file named by the
// flag -out, which is download.jpg by default. The flag -url gives the full URL
// of the gallery page, for example a live site, and then the program does not
// start the local site. The selectors are written for the local site, so a live
// site can differ. For this technique to work, the file type must load inside
// the browser window without a download. See the download_file example for how
// to save a file that triggers the "Download / Save As" browser dialog. Use -v
// to print the protocol messages and -visible to show the browser window and
// leave it open.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/gif"  // the decoders, for image.Decode
	_ "image/jpeg" // the decoders, for image.Decode
	_ "image/png"  // the decoders, for image.Decode
	"log"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
)

// wanted is the name of the picture that the program downloads. On the local
// site it is a JPEG file of 1.2 MB.
const wanted = "gallery-01.jpg"

func main() {
	urlstr := flag.String("url", "", "full URL of the gallery page, for example a live site. The local test site starts when it is empty. The selectors are written for the local site and a live site can differ")
	out := flag.String("out", "download.jpg", "name of the image file to write")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// choose the page. Without -url, the program starts the local site
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/gallery"
	}

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	// create a timeout, so that no wait loop can run forever
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// read the addresses of the pictures from the gallery page
	urls, err := imageURLs(ctx, *urlstr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("the page lists %d pictures, for example:\n", len(urls))
	for _, u := range urls[:min(3, len(urls))] {
		fmt.Printf("  %s\n", u)
	}

	// find the large picture
	var imageURL string
	for _, u := range urls {
		if parsed, err := url.Parse(u); err == nil && path.Base(parsed.Path) == wanted {
			imageURL = u
		}
	}
	if imageURL == "" {
		log.Fatalf("the page has no picture named %s", wanted)
	}

	// download it through the network response
	buf, err := download(ctx, imageURL)
	if err != nil {
		log.Fatal(err)
	}

	// check what the bytes are. image.DecodeConfig reads the type and the size
	// from the header, and image.Decode reads all pixels, so it fails for a
	// file that is cut or damaged
	cfg, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		log.Fatalf("the downloaded file is not an image: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		log.Fatalf("decoding the downloaded image: %v", err)
	}
	fmt.Printf("downloaded %s: %s, %d x %d px, %d bytes, decoded size %v\n",
		wanted, format, cfg.Width, cfg.Height, len(buf), img.Bounds().Size())

	// write the file to disk. The program holds the bytes, so it chooses the
	// name and the location
	if err := os.WriteFile(*out, buf, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s\n", *out)
}

// imageURLs loads the gallery page and returns the full addresses of the files
// that its pictures link to. The script reads the property href of each link,
// and that property is the absolute address. The pictures themselves have a
// srcset and smaller copies, but the link points to the whole file.
func imageURLs(ctx context.Context, urlstr string) ([]string, error) {
	if err := chromedp.Do(ctx,
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(`a.gallery-link`),
	); err != nil {
		return nil, fmt.Errorf("loading %s: %w", urlstr, err)
	}
	urls, err := chromedp.Run(ctx, chromedp.Evaluate[[]string](
		`Array.from(document.querySelectorAll('a.gallery-link'), a => a.href)`))
	if err != nil {
		return nil, fmt.Errorf("reading the picture addresses: %w", err)
	}
	return urls, nil
}

// download loads the address in the browser window and returns the bytes of the
// response. The window keeps the body of a response until the page navigates
// away, so the program can ask for it after the load has finished.
func download(ctx context.Context, urlstr string) ([]byte, error) {
	// subscribe to the network events, so that the program can watch them
	// after the navigation. The request ID filters out the events of other
	// requests, and it finds the downloaded file later
	requests := chromedp.Events(ctx, network.RequestWillBeSent)
	finished := chromedp.Events(ctx, network.LoadingFinished)

	// navigate to the download URL
	if err := chromedp.Do(ctx, chromedp.Navigate(urlstr)); err != nil {
		return nil, fmt.Errorf("loading %s: %w", urlstr, err)
	}

	// the request ID of the download, to match the later events
	var requestID network.RequestID
	for ev, err := range requests {
		if err != nil {
			return nil, fmt.Errorf("waiting for the request: %w", err)
		}
		if ev.Request.URL == urlstr {
			requestID = ev.RequestID
			fmt.Printf("request %s: %s\n", ev.RequestID, ev.Request.URL)
			break
		}
	}

	// wait until the request with this ID is finished
	for ev, err := range finished {
		if err != nil {
			return nil, fmt.Errorf("waiting for the response: %w", err)
		}
		if ev.RequestID == requestID {
			fmt.Printf("finished %s: %.0f bytes on the network\n", ev.RequestID, ev.EncodedDataLength)
			break
		}
	}

	// get the downloaded bytes of the request
	buf, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
		res, err := cdp.Call(ctx, t, network.GetResponseBody, network.GetResponseBodyParams{RequestID: requestID})
		if err != nil {
			return nil, err
		}
		// the result holds the decoded body
		return res.Body, nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading the response body: %w", err)
	}
	return buf, nil
}
