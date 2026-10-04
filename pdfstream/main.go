// Command pdfstream is a chromedp example demonstrating how to print a page to
// a PDF file with a stream. By default, the command Page.printToPDF returns
// the whole PDF in its result, and cdproto decodes it into one byte slice. With
// the transfer mode ReturnAsStream, the browser keeps the PDF and returns a
// handle. The program reads the stream in chunks with the command IO.read,
// copies it to a file with io.Copy and closes the stream with IO.close. The
// stream is better for a large PDF, because the program never holds the whole
// document in memory, and because the browser does not have to send it in one
// message. For a small PDF, the default mode is simpler. The program writes the
// file named by -out, prints its size and checks that it starts with %PDF. It
// starts a local server and needs no internet. Use -v to print the protocol
// messages and -visible to show the browser window and leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	cdpio "github.com/chromedp/cdproto/io"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
	out := flag.String("out", "out.pdf", "name of the PDF file to write")
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

	if err := run(ctx, srv.URL, *out); err != nil {
		log.Fatal(err)
	}
}

// run loads the page, prints it to the file, and checks the file.
func run(ctx context.Context, pageURL, out string) error {
	if err := chromedp.Do(ctx, chromedp.Navigate(pageURL)); err != nil {
		return fmt.Errorf("loading %s: %w", pageURL, err)
	}
	n, err := chromedp.Run(ctx, func(ctx context.Context, t *chromedp.Target) (int64, error) {
		return printToFile(ctx, t, out)
	})
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d bytes\n", out, n)

	// Every PDF file starts with the text %PDF.
	f, err := os.Open(out)
	if err != nil {
		return fmt.Errorf("opening %s: %w", out, err)
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("reading the start of %s: %w", out, err)
	}
	if string(magic) != "%PDF" {
		return fmt.Errorf("%s starts with %q, not %%PDF", out, magic)
	}
	fmt.Printf("%s starts with %s\n", out, magic)
	return nil
}

// printToFile prints the page and copies the stream of the PDF to the file. It
// returns the number of bytes that it wrote.
func printToFile(ctx context.Context, t *chromedp.Target, out string) (int64, error) {
	// The optional settings of the command are pointers when zero is a valid
	// value, so that the program can tell zero from not set. new(true) and
	// new(0.5) make a pointer to a value.
	res, err := cdp.Call(ctx, t, page.PrintToPDF, page.PrintToPDFParams{
		PrintBackground: new(true),
		MarginTop:       new(0.5),
		MarginBottom:    new(0.5),
		TransferMode:    page.PrintToPDFTransferModeReturnAsStream,
	})
	if err != nil {
		return 0, fmt.Errorf("printing the page: %w", err)
	}
	// The browser keeps the stream in memory until the program closes it, so
	// close it even when the copy fails.
	defer func() {
		if _, err := cdp.Call(ctx, t, cdpio.Close, cdpio.CloseParams{Handle: res.Stream}); err != nil {
			log.Printf("closing the stream: %v", err)
		}
	}()

	f, err := os.Create(out)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", out, err)
	}
	r := &streamReader{ctx: ctx, target: t, handle: res.Stream}
	n, err := io.Copy(f, r)
	if err != nil {
		f.Close()
		return n, fmt.Errorf("copying the stream to %s: %w", out, err)
	}
	if err := f.Close(); err != nil {
		return n, fmt.Errorf("closing %s: %w", out, err)
	}
	fmt.Printf("read the stream with %d calls of IO.read\n", r.calls)
	return n, nil
}

// streamReader is an io.Reader for a stream of the browser. Each call of Read
// sends one command IO.read.
type streamReader struct {
	ctx    context.Context
	target *chromedp.Target
	handle cdpio.StreamHandle

	// pending holds the bytes that the browser returned and that Read has not
	// given to the caller yet.
	pending []byte
	eof     bool
	calls   int
}

// Read implements io.Reader. It asks the browser for at most len(p) bytes. The
// result of IO.read has the field EOF, which is true when the stream ended, and
// Read then returns io.EOF after it has given all the bytes.
func (r *streamReader) Read(p []byte) (int, error) {
	if len(r.pending) == 0 {
		if r.eof {
			return 0, io.EOF
		}
		res, err := cdp.Call(r.ctx, r.target, cdpio.Read, cdpio.ReadParams{Handle: r.handle, Size: int64(len(p))})
		if err != nil {
			return 0, fmt.Errorf("reading the stream: %w", err)
		}
		r.calls++
		r.pending, r.eof = res.Data, res.EOF
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	if len(r.pending) == 0 && r.eof {
		return n, io.EOF
	}
	return n, nil
}

// newMux returns the handlers of the test server. The page is a long table, so
// the PDF has several pages.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		b.WriteString(`<html><head><title>PDF stream</title>
<style>
th { background: #336; color: white; }
td, th { border: 1px solid #999; padding: 4px 12px; }
table { border-collapse: collapse; }
</style></head><body><h1>PDF stream</h1><table><tr><th>Row</th><th>Square</th></tr>`)
		for i := 1; i <= 1500; i++ {
			fmt.Fprintf(&b, "<tr><td>%d</td><td>%d</td></tr>", i, i*i)
		}
		b.WriteString("</table></body></html>")
		fmt.Fprint(w, b.String())
	})
	return mux
}
