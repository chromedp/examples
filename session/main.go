// Command session is a chromedp example demonstrating how to save the state of
// a session and restore it in another browser. It logs in on a local page that
// sets a cookie that JavaScript cannot read (HttpOnly), and that writes
// localStorage and sessionStorage. The program reads the cookies with the
// command Network.getCookies and the storage with JavaScript, and it saves both
// in a JSON file. It then starts a second browser, which has a new profile and
// knows nothing about the first one. The second browser shows the login page.
// The program sets the cookies with Network.setCookies and the storage with a
// script that Page.addScriptToEvaluateOnNewDocument runs before the scripts of
// the page. The second browser then shows the page of a logged in user, and it
// never ran the login. The saved file holds the cookie of a session. Keep such
// a file private, as you keep a password. It starts a local server and needs
// no internet. Use -v to print the protocol messages and -visible to show the
// browser windows and leave them open.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// state is what the program saves: the cookies and the two kinds of storage of
// one origin.
type state struct {
	Origin         string            `json:"origin"`
	Cookies        []*network.Cookie `json:"cookies"`
	LocalStorage   map[string]string `json:"localStorage"`
	SessionStorage map[string]string `json:"sessionStorage"`
}

func main() {
	file := flag.String("state", "", "file for the saved state (default: a new temporary file)")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// create the contexts of the two browsers. Each call of NewContext with
	// the background context starts a browser of its own, with its own
	// profile.
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	first, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	second, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			for _, ctx := range []context.Context{first, second} {
				wsURL, dir := chromedp.KeptOpen(ctx)
				fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
			}
		}()
	}

	path, err := stateFile(*file)
	if err != nil {
		log.Fatal(err)
	}
	if err := run(first, second, srv.URL, path); err != nil {
		log.Fatal(err)
	}
}

// stateFile returns the name of the file for the state. An empty name means a
// new temporary file.
func stateFile(name string) (string, error) {
	if name != "" {
		return name, nil
	}
	f, err := os.CreateTemp("", "session-*.json")
	if err != nil {
		return "", fmt.Errorf("creating a temporary file: %w", err)
	}
	defer f.Close()
	return f.Name(), nil
}

// run logs in with the first browser, saves the state, and restores it in the
// second browser.
func run(first, second context.Context, host, path string) error {
	if err := login(first, host); err != nil {
		return err
	}
	s, err := capture(first, host)
	if err != nil {
		return err
	}
	if err := save(path, s); err != nil {
		return err
	}

	// The second browser has no cookie yet, so the server asks for a login.
	if err := chromedp.Do(second, chromedp.Navigate(host+"/home")); err != nil {
		return fmt.Errorf("loading the home page in the second browser: %w", err)
	}
	text, err := chromedp.Run(second, chromedp.Text(chromedp.CSS("body")))
	if err != nil {
		return fmt.Errorf("reading the home page in the second browser: %w", err)
	}
	fmt.Printf("second browser before the restore: %s\n", text)

	// Read the file again, as a later run of the program does.
	s, err = load(path)
	if err != nil {
		return err
	}
	if err := restore(second, s); err != nil {
		return err
	}
	if err := chromedp.Do(second, chromedp.Navigate(host+"/home")); err != nil {
		return fmt.Errorf("loading the home page after the restore: %w", err)
	}
	text, err = chromedp.Run(second, chromedp.Text(chromedp.CSS("body")))
	if err != nil {
		return fmt.Errorf("reading the home page after the restore: %w", err)
	}
	fmt.Printf("second browser after the restore: %s\n", text)
	return nil
}

// login opens the login page. The server sets the cookie, and the script of
// the page writes the storage.
func login(ctx context.Context, host string) error {
	if err := chromedp.Do(ctx, chromedp.Navigate(host+"/login")); err != nil {
		return fmt.Errorf("loading the login page: %w", err)
	}
	text, err := chromedp.Run(ctx, chromedp.Text(chromedp.CSS("body")))
	if err != nil {
		return fmt.Errorf("reading the login page: %w", err)
	}
	fmt.Printf("first browser: %s\n", text)
	return nil
}

// capture reads the cookies and the storage of the page that the browser shows.
func capture(ctx context.Context, host string) (*state, error) {
	// Network.getCookies returns the cookies that apply to the URLs. It also
	// returns the cookies that have the flag HttpOnly, which JavaScript cannot
	// read. The command Storage.getCookies gives all cookies of a browser
	// context instead.
	res, err := chromedp.Call(ctx, network.GetCookies, network.GetCookiesParams{URLs: []string{host}})
	if err != nil {
		return nil, fmt.Errorf("reading the cookies: %w", err)
	}

	// The storage is not part of the protocol commands that this program
	// uses, so a script reads it. Each Evaluate gives a map of the entries.
	local, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]string](`Object.fromEntries(Object.entries(localStorage))`))
	if err != nil {
		return nil, fmt.Errorf("reading localStorage: %w", err)
	}
	session, err := chromedp.Run(ctx, chromedp.Evaluate[map[string]string](`Object.fromEntries(Object.entries(sessionStorage))`))
	if err != nil {
		return nil, fmt.Errorf("reading sessionStorage: %w", err)
	}
	return &state{Origin: host, Cookies: res.Cookies, LocalStorage: local, SessionStorage: session}, nil
}

// save writes the state as indented JSON. The mode 0o600 gives the file to the
// owner only.
func save(path string, s *state) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("saved the state in %s (%d bytes), cookies: %d, localStorage entries: %d, sessionStorage entries: %d\n",
		path, len(data), len(s.Cookies), len(s.LocalStorage), len(s.SessionStorage))
	for _, c := range s.Cookies {
		fmt.Printf("  cookie %s, HttpOnly: %t\n", c.Name, c.HTTPOnly)
	}
	return nil
}

// load reads the state from the file.
func load(path string) (*state, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return &s, nil
}

// restore sets the cookies and the storage in a browser that has none.
func restore(ctx context.Context, s *state) error {
	// A cookie parameter has fewer fields than a cookie. The URL gives the
	// domain and the secure scheme of the cookie, so the program leaves the
	// domain out. A cookie that has no expiry time is a session cookie, and
	// the parameter keeps it as one when Expires is 0.
	params := make([]*network.CookieParam, 0, len(s.Cookies))
	for _, c := range s.Cookies {
		p := &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			URL:      s.Origin,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: c.SameSite,
		}
		if !c.Session {
			p.Expires = cdp.TimeSinceEpoch(c.Expires)
		}
		params = append(params, p)
	}
	if _, err := chromedp.Call(ctx, network.SetCookies, network.SetCookiesParams{Cookies: params}); err != nil {
		return fmt.Errorf("setting the cookies: %w", err)
	}

	// The storage belongs to an origin, and a page cannot write the storage
	// of another origin. So the script runs at the start of every new
	// document, before the scripts of the page, and it writes the entries
	// only for the saved origin. JSON is valid JavaScript, so the script
	// holds the state as a literal. sessionStorage lives as long as a tab, so
	// the script writes it only when the key is not there yet.
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encoding the script: %w", err)
	}
	source := `(() => {
  const state = ` + string(data) + `;
  if (location.origin !== state.origin) return;
  for (const [key, value] of Object.entries(state.localStorage)) localStorage.setItem(key, value);
  for (const [key, value] of Object.entries(state.sessionStorage)) {
    if (sessionStorage.getItem(key) === null) sessionStorage.setItem(key, value);
  }
})();`
	if _, err := chromedp.Call(ctx, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: source}); err != nil {
		return fmt.Errorf("adding the restore script: %w", err)
	}
	return nil
}

// newMux returns the handlers of the test server. The page /login sets the
// cookie and writes the storage. The page /home shows the user of the cookie
// and the storage, or asks for a login.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "token-of-ada",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   3600,
		})
		fmt.Fprint(w, `<html><body>logged in as ada
<script>
localStorage.setItem("theme", "dark");
sessionStorage.setItem("cart", "3 items");
</script></body></html>`)
	})
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil || c.Value != "token-of-ada" {
			fmt.Fprint(w, `<html><body>please log in</body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body>hello ada, theme <span id="theme"></span>, cart <span id="cart"></span>
<script>
document.getElementById("theme").textContent = localStorage.getItem("theme");
document.getElementById("cart").textContent = sessionStorage.getItem("cart");
</script></body></html>`)
	})
	return mux
}
