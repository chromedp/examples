# Using

The program connects to a browser that is already running. The browser must
have a remote debugging port.

Start the browser with `headless-shell`:

```sh
$ podman run --rm --detach --publish 9222:9222 docker.io/chromedp/headless-shell:latest
```

Or start Google Chrome:

```sh
$ google-chrome-stable --remote-debugging-port=9222
```

Then run the program from the root of the repository:

```sh
$ go run ./remote
```

The flag `-url` sets the URL of the browser. The default is
`ws://127.0.0.1:9222`.

```sh
$ go run ./remote -url ws://127.0.0.1:9222
```

The flag `-nav` sets the page to read. The default is
`https://www.duckduckgo.com/`, so the program needs the internet unless you set
this flag.
