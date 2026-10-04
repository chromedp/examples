// Command screencast is a chromedp example demonstrating how to record the
// screen of a page as a series of JPEG images. It loads a local page that
// animates, starts a screencast with the command Page.startScreencast, and
// saves each frame as a numbered file in the directory named by -out. It
// acknowledges every frame with Page.screencastFrameAck, because the browser
// sends no more frames until it receives the acknowledgment. After the time
// named by -d, it stops the screencast, and prints the number of frames and the
// frame rate. A headless browser sends a frame only when the page changes, so
// a page that does not change gives few frames. The program does not need
// ffmpeg. To make a video from the frames, run this command in the directory of
// the frames:
//
//	ffmpeg -framerate 10 -i frame-%04d.jpg out.mp4
//
// The program asks for every sixth frame. A page that draws 60 frames per
// second then gives about 10 frames per second, and the video plays in real
// time. If the program prints another rate, set -framerate to that rate. It
// starts a local server and needs no internet. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	out := flag.String("out", "", "directory for the frames (default: a new temporary directory)")
	d := flag.Duration("d", 3*time.Second, "time to record")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

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

	dir, err := frameDir(*out)
	if err != nil {
		log.Fatal(err)
	}
	if err := run(ctx, srv.URL, dir, *d); err != nil {
		log.Fatal(err)
	}
}

// frameDir makes the directory for the frames. An empty name means a new
// temporary directory.
func frameDir(name string) (string, error) {
	if name == "" {
		dir, err := os.MkdirTemp("", "screencast-")
		if err != nil {
			return "", fmt.Errorf("creating a temporary directory: %w", err)
		}
		return dir, nil
	}
	if err := os.MkdirAll(name, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", name, err)
	}
	return name, nil
}

// run records the page for the time d, and saves the frames in dir.
func run(ctx context.Context, pageURL, dir string, d time.Duration) error {
	// Load the page first. This call starts the browser, and the browser stops
	// when the context of the first call ends. The recording below uses a
	// context with a time limit, so it must not be the first.
	if err := chromedp.Do(ctx, chromedp.Navigate(pageURL)); err != nil {
		return fmt.Errorf("loading %s: %w", pageURL, err)
	}
	fmt.Printf("saving the frames in %s\n", dir)

	// Subscribe before the start of the screencast, so that the first frame
	// is not lost. The iterator ends with an error when the time limit ends.
	recCtx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	frames := chromedp.Events(recCtx, page.ScreencastFrame)

	// The quality is a pointer, because 0 is a valid quality. EveryNthFrame 6
	// asks for one frame of each six that the browser draws.
	if _, err := chromedp.Call(ctx, page.StartScreencast, page.StartScreencastParams{
		Format:        page.StartScreencastFormatJpeg,
		Quality:       new(int64(80)),
		MaxWidth:      800,
		MaxHeight:     600,
		EveryNthFrame: 6,
	}); err != nil {
		return fmt.Errorf("starting the screencast: %w", err)
	}

	var count int
	var first, last cdp.TimeSinceEpoch
	for ev, err := range frames {
		if errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if err != nil {
			return fmt.Errorf("receiving a frame: %w", err)
		}

		// Acknowledge the frame at once, so that the browser can send the next
		// one while the program writes this one.
		if _, err := chromedp.Call(ctx, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: ev.SessionID}); err != nil {
			return fmt.Errorf("acknowledging frame %d: %w", count+1, err)
		}

		// The data of the frame is the JPEG file, already decoded from base64.
		count++
		name := filepath.Join(dir, fmt.Sprintf("frame-%04d.jpg", count))
		if err := os.WriteFile(name, ev.Data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		if count == 1 {
			first = ev.Metadata.Timestamp
		}
		last = ev.Metadata.Timestamp
	}

	if _, err := chromedp.Call(ctx, page.StopScreencast, cdp.Empty{}); err != nil {
		return fmt.Errorf("stopping the screencast: %w", err)
	}

	// The browser stamps each frame. The frame rate is the number of
	// intervals between the frames, divided by the time between the first
	// and the last frame.
	fmt.Printf("%d frames in %s", count, d)
	if span := last.Float64() - first.Float64(); count > 1 && span > 0 {
		fmt.Printf(", %.1f frames per second", float64(count-1)/span)
	}
	fmt.Println()
	return nil
}

// newMux returns the handlers of the test server. The page animates with a CSS
// animation and with a script that changes a number each 100 ms.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>screencast</title>
<style>
body { font: 48px sans-serif; }
#box { width: 80px; height: 80px; background: tomato; animation: slide 2s linear infinite alternate; }
@keyframes slide { from { margin-left: 0; } to { margin-left: 600px; } }
</style></head><body>
<div id="count">0</div>
<div id="box"></div>
<script>
let n = 0;
setInterval(() => { document.getElementById("count").textContent = ++n; }, 100);
</script>
</body></html>`)
	})
	return mux
}
