// Package termstream holds the flags of the example programs for the stream of
// termcast, and the calls that the programs share around the stream.
//
// The package termcast has no flags. Each program owns its flags, and this
// package defines the two flags that the programs have in common, so that they
// have the same names and texts in every program.
package termstream

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"log"

	"github.com/chromedp/termcast"
	"github.com/kenshaw/rasterm"
)

// ErrVerbose is the error of [Flags.Start] when the flag -visible-on-terminal
// and the flag -v are both on.
var ErrVerbose = errors.New("the stream on the terminal does not work with -v, because the protocol messages draw over the frames")

// Flags holds the values of the flags -visible-on-terminal and
// -terminal-fps. [Register] makes it.
type Flags struct {
	// On is the value of -visible-on-terminal.
	On bool
	// FPS is the value of -terminal-fps.
	FPS float64
}

// Register adds the flags -visible-on-terminal and -terminal-fps to fs, and
// returns their values. The values are set when the program parses fs.
func Register(fs *flag.FlagSet) *Flags {
	f := new(Flags)
	fs.BoolVar(&f.On, "visible-on-terminal", false, "draw the page in the terminal with terminal graphics. the stream does not work with -v")
	fs.Float64Var(&f.FPS, "terminal-fps", termcast.DefaultFPS, "frames per second of the stream on the terminal")
	return f
}

// Start starts the stream when the flag -visible-on-terminal is on, with the
// frame rate of -terminal-fps and the options. It returns a nil stream and no
// error when the flag is off. A nil stream is a stream that is off. It returns
// [ErrVerbose] when verbose is also on.
func (f *Flags) Start(ctx context.Context, verbose bool, opts ...termcast.Option) (*termcast.Stream, error) {
	switch {
	case !f.On:
		return nil, nil
	case verbose:
		return nil, ErrVerbose
	}
	return termcast.Start(ctx, append([]termcast.Option{termcast.WithFPS(f.FPS)}, opts...)...)
}

// Fatal stops the stream and then calls [log.Fatal] with v. log.Fatal skips
// the deferred calls of the program, so the stream stops first. Then the final
// frame and the held log lines reach the terminal. s can be nil.
func Fatal(s *termcast.Stream, v ...any) {
	s.Stop()
	log.Fatal(v...)
}

// Draw draws img in the terminal of w. It uses the terminal that the stream
// found, and detects the terminal when the stream is off. Stop the stream
// first, so that it does not draw over the image.
func Draw(ctx context.Context, w io.Writer, s *termcast.Stream, img image.Image) error {
	t := s.Terminal()
	if s == nil {
		var err error
		if t, _, err = rasterm.Detect(ctx); err != nil {
			return fmt.Errorf("detecting the terminal: %w", err)
		}
	}
	enc := t.Encoder()
	if enc == nil {
		return rasterm.ErrTermGraphicsNotAvailable
	}
	if err := enc.Encode(w, img); err != nil {
		return fmt.Errorf("drawing the image: %w", err)
	}
	return nil
}
