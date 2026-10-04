// Command dragdrop is a chromedp example demonstrating how to drag and drop
// with the actions DragAndDrop and DragAndDropXY. The program loads a local
// page with three parts. A slider has a handle that follows the mouse events. A
// sortable list changes its order when a person drags one item onto another
// item, and it also listens for mouse events. An area with HTML5 drag and drop
// has draggable cards and drop zones, and the page reads the text of a
// DataTransfer when a card drops. The same two actions work for both kinds of
// page. After each drag, the program asks the page what it recorded and prints
// the positions, the new order and the dropped text. It starts a local server
// and needs no internet. Use -v to print the protocol messages and -visible to
// show the browser window and leave it open. Use -visible-on-terminal to draw
// the page in the terminal with terminal graphics.
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

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

// state is what the page records. The page keeps it in the variable state.
type state struct {
	Slider struct {
		Value int     `json:"value"`
		Left  float64 `json:"left"`
	} `json:"slider"`
	Order []string `json:"order"`
	Drops []drop   `json:"drops"`
}

// drop is one drop of a card on a zone.
type drop struct {
	Zone  string   `json:"zone"`
	Text  string   `json:"text"`
	Types []string `json:"types"`
}

// point is a place in the viewport, in CSS pixels.
type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

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

	// draw the page in the terminal when the user asks for it. The stream
	// starts the browser, so it starts before the first navigation
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())
	out := io.Writer(os.Stdout)
	if s != nil {
		out = s.LogWriter()
	}

	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		s.Fatal(err)
	}
	if err := slider(ctx, out); err != nil {
		s.Fatal(err)
	}
	if err := list(ctx, out); err != nil {
		s.Fatal(err)
	}
	if err := cards(ctx, out); err != nil {
		s.Fatal(err)
	}
}

// slider drags the handle of the slider three times.
func slider(ctx context.Context, out io.Writer) error {
	// DragAndDrop takes two selectors. It presses the mouse at the center of
	// the first element, moves to the center of the second one in steps, and
	// releases the mouse. Here the second element is a mark on the track.
	if err := chromedp.Do(ctx, chromedp.DragAndDrop(chromedp.ID("handle"), chromedp.ID("mark"))); err != nil {
		return fmt.Errorf("dragging the handle to the mark: %w", err)
	}
	st, err := readState(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "slider, dragged to the mark: value %d, handle at %.1f px\n", st.Slider.Value, st.Slider.Left)

	// DragAndDropXY takes two points, so it can drag to a place that has no
	// element. The page tells where the handle is now. The last argument sets
	// the number of mouse moves. The default is 10.
	from, err := handleCenter(ctx)
	if err != nil {
		return err
	}
	to := point{from.X - 150, from.Y}
	if err := chromedp.Do(ctx, chromedp.DragAndDropXY(from.X, from.Y, to.X, to.Y, 20)); err != nil {
		return fmt.Errorf("dragging the handle to the left: %w", err)
	}
	if st, err = readState(ctx); err != nil {
		return err
	}
	fmt.Fprintf(out, "slider, dragged from x %.1f to x %.1f in 20 steps: value %d, handle at %.1f px\n", from.X, to.X, st.Slider.Value, st.Slider.Left)

	// The page keeps the handle on the track. A drag far beyond the end of the
	// track gives the largest value. The point must stay in the viewport.
	from, err = handleCenter(ctx)
	if err != nil {
		return err
	}
	to = point{from.X + 500, from.Y}
	if err := chromedp.Do(ctx, chromedp.DragAndDropXY(from.X, from.Y, to.X, to.Y)); err != nil {
		return fmt.Errorf("dragging the handle past the end: %w", err)
	}
	if st, err = readState(ctx); err != nil {
		return err
	}
	fmt.Fprintf(out, "slider, dragged from x %.1f to x %.1f: value %d, handle at %.1f px\n", from.X, to.X, st.Slider.Value, st.Slider.Left)
	return nil
}

// handleCenter returns the center of the handle of the slider. The selectors
// of DragAndDrop find the centers by themselves, but DragAndDropXY needs
// numbers.
func handleCenter(ctx context.Context) (point, error) {
	p, err := chromedp.Run(ctx, chromedp.Evaluate[point](`(() => {
		const r = document.getElementById("handle").getBoundingClientRect();
		return {x: r.x + r.width / 2, y: r.y + r.height / 2};
	})()`))
	if err != nil {
		return point{}, fmt.Errorf("reading the position of the handle: %w", err)
	}
	return p, nil
}

// list drags items of the sortable list onto other items.
func list(ctx context.Context, out io.Writer) error {
	st, err := readState(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "list, at the start: %s\n", strings.Join(st.Order, ", "))

	// The two selectors can have different types. The page moves the item
	// that the mouse leaves to the place of the item below the mouse.
	moves := [][2]string{{"alpha", "gamma"}, {"delta", "beta"}}
	for _, m := range moves {
		from := chromedp.CSS(fmt.Sprintf(`#list li[data-id="%s"]`, m[0]))
		to := chromedp.CSS(fmt.Sprintf(`#list li[data-id="%s"]`, m[1]))
		if err := chromedp.Do(ctx, chromedp.DragAndDrop(from, to)); err != nil {
			return fmt.Errorf("dragging %s onto %s: %w", m[0], m[1], err)
		}
		if st, err = readState(ctx); err != nil {
			return err
		}
		fmt.Fprintf(out, "list, %s dragged onto %s: %s\n", m[0], m[1], strings.Join(st.Order, ", "))
	}
	return nil
}

// cards drags the cards of the HTML5 area onto its zones.
func cards(ctx context.Context, out io.Writer) error {
	// The same call works. The page has draggable elements, so the browser
	// starts a native drag after the first mouse moves. The action catches the
	// drag and sends the drag events, so the page gets the events dragover and
	// drop with a DataTransfer that holds the text of the card.
	drags := [][2]string{{"card1", "done"}, {"card2", "later"}}
	for _, d := range drags {
		if err := chromedp.Do(ctx, chromedp.DragAndDrop(chromedp.ID(d[0]), chromedp.ID(d[1]))); err != nil {
			return fmt.Errorf("dragging %s onto %s: %w", d[0], d[1], err)
		}
	}
	st, err := readState(ctx)
	if err != nil {
		return err
	}
	for _, d := range st.Drops {
		fmt.Fprintf(out, "html5, dropped text %q on the zone %s (data types: %s)\n", d.Text, d.Zone, strings.Join(d.Types, ", "))
	}

	// The page moved the cards into the zones, so the program can count the
	// cards in each zone.
	for _, zone := range []string{"inbox", "done", "later"} {
		nodes, err := chromedp.Run(ctx, chromedp.Nodes(chromedp.CSSAll("#"+zone+" .card"), chromedp.AtLeast(0)))
		if err != nil {
			return fmt.Errorf("counting the cards of %s: %w", zone, err)
		}
		fmt.Fprintf(out, "html5, cards in %s: %d\n", zone, len(nodes))
	}
	return nil
}

// readState asks the page what it recorded.
func readState(ctx context.Context) (state, error) {
	st, err := chromedp.Run(ctx, chromedp.Evaluate[state](`state`))
	if err != nil {
		return state{}, fmt.Errorf("reading the state of the page: %w", err)
	}
	return st, nil
}

// newMux returns the handlers of the test server. The page / holds the three
// parts. The page keeps what it saw in the variable state.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// The browser asks for /favicon.ico by itself, and the server has
		// no icon.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, page)
	})
	return mux
}

// page is the HTML of the test page. Each part has a fixed place, so that all
// the elements are in the viewport together, which DragAndDrop needs. The
// viewport of the headless browser is 780 by 437 pixels, so the page is small.
const page = `<html><head><title>dragdrop</title>
<style>
body { margin: 0; font-family: sans-serif; user-select: none; }
#slider { position: absolute; left: 20px; top: 15px; width: 400px; height: 30px; }
#track { position: absolute; left: 0; top: 13px; width: 400px; height: 4px; background: #bbb; }
#mark { position: absolute; left: 290px; top: 5px; width: 5px; height: 20px; background: #c33; }
#handle { position: absolute; left: 0; top: 0; width: 30px; height: 30px; background: #46c; }
#list { position: absolute; left: 20px; top: 60px; width: 200px; margin: 0; padding: 0; list-style: none; }
#list li { height: 36px; line-height: 36px; margin-bottom: 4px; padding-left: 10px; background: #dde; }
#list li.dragging { pointer-events: none; opacity: 0.6; }
.zone { position: absolute; top: 250px; width: 220px; height: 170px; background: #cdb; padding: 5px; box-sizing: border-box; }
.card { height: 36px; line-height: 36px; margin-bottom: 4px; padding-left: 10px; background: #46c; color: white; }
</style></head><body>
<div id="slider"><div id="track"></div><div id="mark"></div><div id="handle"></div></div>
<ul id="list">
<li data-id="alpha">alpha</li><li data-id="beta">beta</li>
<li data-id="gamma">gamma</li><li data-id="delta">delta</li>
</ul>
<div class="zone" id="inbox" style="left: 20px">inbox
<div class="card" id="card1" draggable="true" data-text="buy milk">card 1</div>
<div class="card" id="card2" draggable="true" data-text="call Ann">card 2</div></div>
<div class="zone" id="done" style="left: 280px">done</div>
<div class="zone" id="later" style="left: 540px">later</div>
<script>
// The page records what it saw, and the program reads it.
const state = {slider: {value: 0, left: 0}, order: [], drops: []};
const list = document.getElementById("list");
const order = () => Array.from(list.children, li => li.dataset.id);
state.order = order();

// The slider moves with the mouse events. The handle keeps the distance that
// the mouse had from its edge, and it stays on the track.
const handle = document.getElementById("handle");
let grab = null;
handle.addEventListener("mousedown", e => {
  grab = e.clientX - handle.offsetLeft;
  e.preventDefault();
});
document.addEventListener("mousemove", e => {
  if (grab === null) return;
  const max = 400 - handle.offsetWidth;
  const left = Math.min(max, Math.max(0, e.clientX - grab));
  handle.style.left = left + "px";
  state.slider = {value: Math.round(left / max * 100), left: left};
});

// The list moves an item with the mouse events too. On mouseup, the item takes
// the place of the item that is below the mouse. The dragged item has
// pointer-events none while it moves, so the event target is the item below.
let dragged = null;
list.addEventListener("mousedown", e => {
  dragged = e.target.closest("li");
  dragged.classList.add("dragging");
  e.preventDefault();
});
document.addEventListener("mouseup", e => {
  if (grab !== null) grab = null;
  if (dragged === null) return;
  const over = e.target.closest ? e.target.closest("#list li") : null;
  dragged.classList.remove("dragging");
  if (over && over !== dragged) {
    const down = Array.from(list.children).indexOf(dragged) < Array.from(list.children).indexOf(over);
    list.insertBefore(dragged, down ? over.nextSibling : over);
  }
  dragged = null;
  state.order = order();
});

// The HTML5 drag and drop. A card puts its text in the DataTransfer, a zone
// accepts the drop when dragover cancels the event, and the drop reads the text.
for (const card of document.querySelectorAll(".card")) {
  card.addEventListener("dragstart", e => {
    e.dataTransfer.setData("text/plain", card.dataset.text);
    e.dataTransfer.effectAllowed = "move";
  });
}
for (const zone of document.querySelectorAll(".zone")) {
  zone.addEventListener("dragover", e => e.preventDefault());
  zone.addEventListener("drop", e => {
    e.preventDefault();
    const text = e.dataTransfer.getData("text/plain");
    state.drops.push({zone: zone.id, text: text, types: Array.from(e.dataTransfer.types)});
    const card = Array.from(document.querySelectorAll(".card")).find(c => c.dataset.text === text);
    zone.appendChild(card);
  });
}
</script></body></html>`
