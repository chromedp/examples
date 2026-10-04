// Command dialogs is a chromedp example demonstrating how to handle the
// JavaScript dialogs of a page. It loads a local page with a button for each
// kind of dialog: alert, confirm, prompt and beforeunload. A goroutine listens
// for the event Page.javascriptDialogOpening and answers each dialog with the
// command Page.handleJavaScriptDialog. It accepts an alert, dismisses a
// confirm, accepts a prompt with a text, and accepts the dialog that asks
// whether the page can unload. The program prints the message and the answer of
// each dialog, and the text that the page shows after it. A dialog blocks the
// page, and so it blocks the click that opens it. The goroutine must answer
// while the click waits. It starts a local server and needs no internet. Use -v
// to print the protocol messages and -visible to show the browser window and
// leave it open.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

func main() {
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

	if err := run(ctx, srv.URL); err != nil {
		log.Fatal(err)
	}
}

// answer is what the handler reports after it closed a dialog.
type answer struct {
	line string
	err  error
}

// run opens each kind of dialog, and prints what the handler did.
func run(ctx context.Context, host string) error {
	// Load the page first. This call starts the browser. The events below use
	// the same context, and they end when the program ends.
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/")); err != nil {
		return fmt.Errorf("loading the page: %w", err)
	}

	// Subscribe before the click that opens the first dialog, so that the
	// goroutine misses no dialog. The goroutine answers the dialogs one by
	// one, and it reports each answer on the channel.
	dialogs := chromedp.Events(ctx, page.JavascriptDialogOpening)
	answers := make(chan answer)
	go func() {
		for ev, err := range dialogs {
			if err != nil {
				// The context ended, so the program is done.
				return
			}
			var res answer
			res.line, res.err = respond(ctx, &ev)
			answers <- res
		}
	}()

	// Each click blocks until the handler answers the dialog. Then the page
	// runs on, and it writes the result of the dialog in the element #result.
	for _, id := range []string{"alert", "confirm", "prompt"} {
		if err := chromedp.Do(ctx, chromedp.Click(chromedp.ID(id))); err != nil {
			return fmt.Errorf("clicking %s: %w", id, err)
		}
		if err := report(ctx, answers); err != nil {
			return err
		}
		text, err := chromedp.Run(ctx, chromedp.Text(chromedp.ID("result")))
		if err != nil {
			return fmt.Errorf("reading the result of %s: %w", id, err)
		}
		fmt.Printf("  the page says %q\n", text)
	}

	// A page can ask the user to confirm that it can unload. Chrome opens the
	// dialog only when the user acted on the page, and the clicks above count
	// as such. The navigation below triggers the dialog. The message of this type
	// is empty, because the browser shows its own text.
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/next")); err != nil {
		return fmt.Errorf("leaving the page: %w", err)
	}
	if err := report(ctx, answers); err != nil {
		return err
	}
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("body")))
	if err != nil {
		return fmt.Errorf("reading the next page: %w", err)
	}
	fmt.Printf("  the browser is now on the page that says %q\n", text)
	return nil
}

// respond answers one dialog, and describes the dialog and the answer.
func respond(ctx context.Context, ev *page.EventJavascriptDialogOpening) (string, error) {
	var params page.HandleJavaScriptDialogParams
	var what string
	switch ev.Type {
	case page.DialogTypeConfirm:
		// To dismiss a dialog is to press Cancel. The page sees false.
		params.Accept = false
		what = "dismissed"
	case page.DialogTypePrompt:
		// Accept with a text to answer the question of a prompt. The text is
		// ignored for the other types.
		params.Accept = true
		params.PromptText = "Ada"
		what = fmt.Sprintf("accepted with the text %q", params.PromptText)
	default:
		params.Accept = true
		what = "accepted"
	}
	if _, err := chromedp.Call(ctx, page.HandleJavaScriptDialog, params); err != nil {
		return "", fmt.Errorf("answering the %s dialog: %w", ev.Type, err)
	}
	return fmt.Sprintf("%s dialog, message %q: %s", ev.Type, ev.Message, what), nil
}

// report waits for the answer of the handler, and prints it.
func report(ctx context.Context, answers <-chan answer) error {
	select {
	case a := <-answers:
		if a.err != nil {
			return a.err
		}
		fmt.Println(a.line)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// newMux returns the handlers of the test server. The page has a button for
// each dialog, and it asks to confirm the unload after the first click.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>dialogs</title></head><body>
<button id="alert">alert</button>
<button id="confirm">confirm</button>
<button id="prompt">prompt</button>
<p id="result"></p>
<script>
const result = document.getElementById("result");
document.getElementById("alert").onclick = () => {
  alert("Hello from the page");
  result.textContent = "the alert closed";
};
document.getElementById("confirm").onclick = () => {
  result.textContent = "confirm gave " + confirm("Delete the file?");
};
document.getElementById("prompt").onclick = () => {
  result.textContent = "prompt gave " + prompt("What is your name?", "nobody");
};
window.onbeforeunload = event => {
  event.preventDefault();
  event.returnValue = "";
};
</script></body></html>`)
	})
	mux.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>the next page</body></html>`)
	})
	return mux
}
