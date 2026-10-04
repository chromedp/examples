// Command download_file is a chromedp example demonstrating how to do headless
// file downloads. It starts a local server and needs no internet. The program
// opens the repository page /repo/chromedp/examples of the local test site,
// clicks the Code button and then Download ZIP, and waits for the download
// events of the browser. It prints the progress, writes the ZIP file to the
// directory of the flag -out, which is the current directory by default, and
// prints the name and the size of the file. It then opens the file with
// archive/zip and lists some of its entries. The flag -url gives the full URL
// of the repository page, for example a live site, and then the program does
// not start the local site. The selectors are written for the local site, so a
// live site can differ. For this technique to work, the file type must trigger
// the "Download / Save As" browser dialog. See the download_image example for
// how to save a file that the browser window loads without a download. Use -v
// to print the protocol messages and -visible to show the browser window and
// leave it open. Use -visible-on-terminal to draw the page in the terminal with
// terminal graphics.
package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	"github.com/chromedp/termcast"
)

func main() {
	urlstr := flag.String("url", "", "full URL of the repository page, for example a live site. The local test site starts when it is empty. The selectors are written for the local site and a live site can differ")
	out := flag.String("out", ".", "directory for the downloaded file")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()

	// choose the page. Without -url, the program starts the local site
	if *urlstr == "" {
		site := testsite.New()
		defer site.Close()
		*urlstr = site.URL + "/repo/chromedp/examples"
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

	// draw the page in the terminal when the user asks for it. The stream
	// starts the browser, so it starts before the first navigation
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	stdout := io.Writer(os.Stdout)
	if s != nil {
		stdout = s.LogWriter()
	}

	// create a timeout, so that no wait loop can run forever
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	path, err := download(ctx, stdout, *urlstr, *out)
	if err != nil {
		s.Fatal(err)
	}
	if err := list(stdout, path); err != nil {
		s.Fatal(err)
	}
}

// download clicks through the page to the ZIP file, waits until the browser
// has saved it in the directory dir, and returns the path of the file.
func download(ctx context.Context, out io.Writer, urlstr, dir string) (string, error) {
	// the download path must be absolute, because the browser does not know
	// the working directory of this program
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("making %s absolute: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating the directory %s: %w", dir, err)
	}

	// subscribe to the download events before the click, so that the program
	// cannot miss one. A download sends browser.DownloadWillBegin first, with
	// the name that the server suggests, and then browser.DownloadProgress
	// many times. To handle many downloads, a program can keep a map of the
	// GUID values
	begin := chromedp.Events(ctx, browser.DownloadWillBegin)
	progress := chromedp.Events(ctx, browser.DownloadProgress)

	if err := chromedp.Do(ctx,
		// navigate to the page
		chromedp.Navigate(urlstr),
		// find the "Code" button, and click it when it is ready
		chromedp.Click(`//button//span[text()="Code"]`, chromedp.NodeReady),
		// configure headless browser downloads. Use
		// SetDownloadBehaviorBehaviorAllowAndName and not
		// SetDownloadBehaviorBehaviorAllow, so that Chrome names the file with
		// its GUID. Without EventsEnabled, the browser sends no download
		// events. The setting works with Chrome 92.0.4498.0 or later, because
		// of issue 1204880, see
		// https://bugs.chromium.org/p/chromium/issues/detail?id=1204880
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			_, err := cdp.Call(ctx, t, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{
				Behavior:      browser.SetDownloadBehaviorBehaviorAllowAndName,
				DownloadPath:  dir,
				EventsEnabled: new(true),
			})
			return err
		}),
		// click the "Download ZIP" link when it is visible. The menu of the
		// Code button shows it
		chromedp.Click(`//span[text()="Download ZIP"]`, chromedp.NodeVisible),
	); err != nil {
		return "", fmt.Errorf("clicking through %s: %w", urlstr, err)
	}

	// wait for the start of the download. The file name that the server
	// suggests comes from its header Content-Disposition
	var name string
	for ev, err := range begin {
		if err != nil {
			return "", fmt.Errorf("waiting for the download to begin: %w", err)
		}
		name = ev.SuggestedFilename
		fmt.Fprintf(out, "download begins: %s from %s\n", name, ev.URL)
		break
	}

	// wait until the download is complete. The browser can send the same
	// received size more than once, so the program waits for the state
	// instead of for 100 percent
	var guid string
	var size float64
	for ev, err := range progress {
		if err != nil {
			return "", fmt.Errorf("waiting for the download to finish: %w", err)
		}
		completed := "(unknown)"
		if ev.TotalBytes != 0 {
			completed = fmt.Sprintf("%0.2f%%", ev.ReceivedBytes/ev.TotalBytes*100.0)
		}
		fmt.Fprintf(out, "state: %s, received %.0f of %.0f bytes, completed: %s\n", ev.State, ev.ReceivedBytes, ev.TotalBytes, completed)
		if ev.State == browser.DownloadProgressStateCompleted {
			guid, size = ev.GUID, ev.ReceivedBytes
			break
		}
		if ev.State == browser.DownloadProgressStateCanceled {
			return "", fmt.Errorf("the browser canceled the download of %s", name)
		}
	}

	// the browser saved the file under its GUID, because of the behavior that
	// the program set. Give it the name that the server suggested
	path := filepath.Join(dir, name)
	if err := os.Rename(filepath.Join(dir, guid), path); err != nil {
		return "", fmt.Errorf("renaming the downloaded file: %w", err)
	}
	fmt.Fprintf(out, "wrote %s, %.0f bytes\n", path, size)
	return path, nil
}

// list opens the ZIP file and prints the number of its entries and the first
// few of them. A file that archive/zip can open is a valid archive.
func list(out io.Writer, path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("opening %s as a ZIP file: %w", path, err)
	}
	defer r.Close()

	fmt.Fprintf(out, "%s holds %d entries:\n", filepath.Base(path), len(r.File))
	for _, f := range r.File[:min(8, len(r.File))] {
		fmt.Fprintf(out, "  %-44s %8d bytes, %8d compressed\n", f.Name, f.UncompressedSize64, f.CompressedSize64)
	}
	return nil
}
