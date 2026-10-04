// Command termcast is a chromedp example demonstrating how to stream the screen
// of the browser to the terminal with terminal graphics. It loads the page
// /animation/ of the local test site, which shows an animated SVG of a harbour
// at night, and lets the animation play for the time of -d. The package
// github.com/chromedp/termcast draws the frames of the screencast in the
// terminal while the page plays. The flag -fps sets the number of frames per
// second that the stream draws. A terminal that can show Kitty, iTerm2 or Sixel
// graphics is needed. If the terminal has none, the program stops with an error
// that says so. The variable TERM_GRAPHICS can force the type of graphics to
// kitty, iterm or sixel. The stream clears the terminal at each frame, so the
// program keeps its log lines and its results until the stream stops, and then
// it prints them. With -v or -visible the program does the same work and prints
// the same lines, but it does not start the stream, because the protocol
// messages and the window do not mix with the images. The flag -url gives the
// full URL of the page to play, and then the program does not start the local
// site. It starts a local server and needs no internet. Use -v to print the
// protocol messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/examples/internal/testsite"
	termcastlib "github.com/chromedp/termcast"
)

// defaultFPS is the frame rate of the stream when -fps is not set. The package
// default is 4 frames per second, which is enough for a page that changes
// rarely. The boat of the animation crosses the picture in 16 seconds, so at 4
// frames per second the scene jumps. At 8 frames per second the motion looks
// smooth, and the terminal can still keep up with the images.
const defaultFPS = 8

func main() {
	urlstr := flag.String("url", "", "full URL of the page to play. The local test site starts when it is empty")
	d := flag.Duration("d", 10*time.Second, "time to let the animation play")
	fps := flag.Float64("fps", defaultFPS, "frames per second that the stream draws on the terminal")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// choose the page. Without -url, the program starts the local site
	pageURL := *urlstr
	if pageURL == "" {
		site := testsite.New()
		defer site.Close()
		pageURL = site.URL + "/animation/"
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

	// Start the stream only when nothing else writes to the terminal. The
	// protocol messages of -v and the window of -visible must not mix with the
	// images. Every method of a nil *Stream is safe, so the rest of the
	// program is the same with and without the stream.
	var s *termcastlib.Stream
	if !*verbose && !*visible {
		var err error
		s, err = termcastlib.Start(ctx, termcastlib.WithFPS(*fps))
		if err != nil {
			log.Fatal(fmt.Errorf("streaming to the terminal: %w. Use -v or -visible to run without the stream", err))
		}
	}
	defer s.Stop()

	// The stream holds the lines of the log and the results while it runs, and
	// prints them after the last frame. Without a stream, the writer is the
	// standard error, and the results go to the standard output.
	log.SetOutput(s.LogWriter())
	var out io.Writer = os.Stdout
	if s != nil {
		out = s.LogWriter()
	}

	// create a timeout, so that the program cannot wait forever for the page
	ctx, cancel = context.WithTimeout(ctx, *d+30*time.Second)
	defer cancel()

	if err := run(ctx, out, pageURL, *d); err != nil {
		s.Fatal(err)
	}
}

// run loads the page and lets the animation play for the time d. It prints the
// time that the animation played to w.
func run(ctx context.Context, w io.Writer, pageURL string, d time.Duration) error {
	log.Printf("loading %s", pageURL)
	if err := chromedp.Do(ctx, chromedp.Navigate(pageURL)); err != nil {
		return fmt.Errorf("loading %s: %w", pageURL, err)
	}

	// The animation runs in the page. The screencast sends a frame each time
	// that the page changes, and the stream draws the latest frame at most
	// -fps times per second. The program only waits.
	log.Printf("playing the animation for %s", d)
	start := time.Now()
	if err := chromedp.Do(ctx, chromedp.Sleep(d)); err != nil {
		return fmt.Errorf("playing the animation: %w", err)
	}

	fmt.Fprintf(w, "played the animation for %s\n", time.Since(start).Round(100*time.Millisecond))
	return nil
}
