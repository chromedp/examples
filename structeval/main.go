// Command structeval is a chromedp example demonstrating how to evaluate
// JavaScript into typed Go values. The type parameter of chromedp.Evaluate
// decides how the result is read. The program loads a local page that holds a
// JavaScript object with nested data, and it reads parts of the object as a
// struct, a slice of structs, a map, a number, a decimal number and a boolean.
// It shows the Go errors for a JavaScript exception, for a result of undefined
// or null, and for a result of the wrong type. It also shows how to wait for a
// promise with an option for Evaluate. It starts a local server and needs no
// internet. Use -v to print the protocol messages, -visible to show the browser
// window and leave it open, and -visible-on-terminal to draw the page in the
// terminal with terminal graphics.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

// User is the shape of the field user of the object of the page. The tags
// name the properties of the JavaScript object.
type User struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Langs   []string `json:"langs"`
	Address struct {
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"address"`
}

// Item is the shape of one entry of the list items.
type Item struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// out receives the results that the program prints. It is the standard output,
// or the held writer of the stream when the flag -visible-on-terminal is on,
// because the stream clears the terminal and would erase the results.
var out io.Writer = os.Stdout

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
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

	// start the stream before the first navigation, so that the frames show
	// the page while it loads
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	if s != nil {
		out = s.LogWriter()
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		s.Fatal(err)
	}
	if err := values(ctx); err != nil {
		s.Fatal(err)
	}
	if err := failures(ctx); err != nil {
		s.Fatal(err)
	}
	if err := promises(ctx); err != nil {
		s.Fatal(err)
	}
}

// values reads parts of the object of the page into Go types.
func values(ctx context.Context) error {
	// A struct. Evaluate asks the browser for the value as JSON, and then
	// decodes it into the type. Properties that the struct does not name are
	// ignored.
	user, err := chromedp.Run(ctx, chromedp.Evaluate[User](`page.user`))
	if err != nil {
		return fmt.Errorf("reading the user: %w", err)
	}
	fmt.Fprintf(out, "struct: %s, %d, %v, lives in %s\n", user.Name, user.Age, user.Langs, user.Address.City)

	// A slice of structs.
	items, err := chromedp.Run(ctx, chromedp.Evaluate[[]Item](`page.items`))
	if err != nil {
		return fmt.Errorf("reading the items: %w", err)
	}
	fmt.Fprintf(out, "slice of structs: %d items, the last is %+v\n", len(items), items[len(items)-1])

	// A map. The keys of the JavaScript object become the keys of the map.
	stock, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]int](`page.stock`))
	if err != nil {
		return fmt.Errorf("reading the stock: %w", err)
	}
	fmt.Fprintf(out, "map: %v\n", stock)

	// A number, a decimal number and a boolean. An expression can compute the
	// value in the browser.
	count, err := chromedp.Run(ctx, chromedp.Evaluate[int](`page.items.length`))
	if err != nil {
		return fmt.Errorf("reading the count: %w", err)
	}
	total, err := chromedp.Run(ctx, chromedp.Evaluate[float64](`page.items.reduce((sum, item) => sum + item.price, 0)`))
	if err != nil {
		return fmt.Errorf("reading the total: %w", err)
	}
	active, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`page.active`))
	if err != nil {
		return fmt.Errorf("reading the flag: %w", err)
	}
	fmt.Fprintf(out, "number: %d, decimal number: %.2f, boolean: %t\n", count, total, active)

	// The type []byte gives the JSON text, and the type
	// *runtime.RemoteObject gives the object of the protocol, which holds the
	// type of the value and a handle to it.
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[[]byte](`page.stock`))
	if err != nil {
		return fmt.Errorf("reading the raw value: %w", err)
	}
	obj, err := chromedp.Run(ctx, chromedp.Evaluate[*runtime.RemoteObject](`page.stock`))
	if err != nil {
		return fmt.Errorf("reading the remote object: %w", err)
	}
	fmt.Fprintf(out, "[]byte: %s, remote object: type %s, class %s\n", raw, obj.Type, obj.ClassName)
	return nil
}

// failures shows the errors of Evaluate.
func failures(ctx context.Context) error {
	// A JavaScript exception is a Go error of the type *chromedp.ExceptionError.
	// It holds the details of the protocol, with the line and the exception
	// object.
	_, err := chromedp.Run(ctx, chromedp.Evaluate[int](`page.missing.property`))
	var exc *chromedp.ExceptionError
	if !errors.As(err, &exc) {
		return fmt.Errorf("want an exception error, got %v", err)
	}
	fmt.Fprintf(out, "exception: %s, description: %s\n", exc.Text, firstLine(exc.Exception.Description))

	// The message of the error holds the same text.
	fmt.Fprintf(out, "error text: %v\n", firstLine(err.Error()))

	// A result of undefined has no value in a type that cannot be nil, such
	// as int, and the error is a value that errors.Is can test.
	_, err = chromedp.Run(ctx, chromedp.Evaluate[int](`undefined`))
	fmt.Fprintf(out, "undefined into int: %v, ErrJSUndefined: %t\n", err, errors.Is(err, chromedp.ErrJSUndefined))

	// A null gives the zero value of the type, and no error. So use a pointer
	// type when null is a valid result and the program must tell it from 0.
	n, err := chromedp.Run(ctx, chromedp.Evaluate[int](`null`))
	fmt.Fprintf(out, "null into int: %d, error: %v\n", n, err)

	// A pointer, a map and a slice can be nil, so they take the null.
	user, err := chromedp.Run(ctx, chromedp.Evaluate[*User](`null`))
	if err != nil {
		return fmt.Errorf("reading null into a pointer: %w", err)
	}
	fmt.Fprintf(out, "null into *User: nil is %t\n", user == nil)

	// A value of the wrong type is an error of the JSON decoder.
	_, err = chromedp.Run(ctx, chromedp.Evaluate[int](`page.user.name`))
	fmt.Fprintf(out, "a string into int: %v\n", err)
	return nil
}

// promises waits for a promise. Evaluate has no option for it, but an option
// is a func that changes the parameters of the protocol command, so the
// program can write its own.
func promises(ctx context.Context) error {
	// Without the option, the result is the promise object, and its JSON has
	// no properties.
	pending, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]any](`fetch("/slow").then(r => r.text())`))
	if err != nil {
		return fmt.Errorf("reading the promise: %w", err)
	}
	fmt.Fprintf(out, "a promise without the option: %v\n", pending)

	// With AwaitPromise, the browser waits until the promise settles and
	// gives its value. The field is a pointer, because false is a value.
	awaitPromise := func(p *runtime.EvaluateParams) { p.AwaitPromise = new(true) }
	text, err := chromedp.Run(ctx, chromedp.Evaluate[string](`fetch("/slow").then(r => r.text())`, awaitPromise))
	if err != nil {
		return fmt.Errorf("reading the awaited promise: %w", err)
	}
	fmt.Fprintf(out, "a promise with AwaitPromise: %q\n", text)

	// A promise that rejects is an exception.
	_, err = chromedp.Run(ctx, chromedp.Evaluate[string](`Promise.reject(new Error("no luck"))`, awaitPromise))
	var exc *chromedp.ExceptionError
	if !errors.As(err, &exc) {
		return fmt.Errorf("want an exception error, got %v", err)
	}
	fmt.Fprintf(out, "a rejected promise: %s\n", firstLine(exc.Exception.Description))
	return nil
}

// firstLine returns the first line of the text.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// newMux returns the handlers of the test server. The page holds an object
// with nested data. The endpoint /slow answers after 200 ms.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>structeval</title></head><body>
<script>
const page = {
  user: {
    name: "Ada",
    age: 36,
    langs: ["go", "javascript"],
    address: {city: "London", country: "UK"},
    note: "the Go struct has no field for this",
  },
  items: [
    {id: 1, name: "pen", price: 1.5},
    {id: 2, name: "book", price: 12.25},
    {id: 3, name: "lamp", price: 30},
  ],
  stock: {pen: 10, book: 0, lamp: 4},
  active: true,
};
</script></body></html>`)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, "slow answer")
	})
	return mux
}
