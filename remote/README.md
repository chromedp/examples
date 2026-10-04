# Using

The program connects to a browser that is already running. The browser must
have a remote debugging port. By default the program starts a local test site
and reads its home page, so it needs no internet.

The simplest way is the flag `-start`. The program then starts a headless
Chrome on a free port, connects to it and stops it at the end. It looks for
`google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`,
`chrome` and `headless-shell` on the `PATH`.

```sh
$ go run ./remote -start
```

To use a browser that you start yourself, start Google Chrome with a debugging
port:

```sh
$ google-chrome-stable --remote-debugging-port=9222
```

Or start `headless-shell`:

```sh
$ podman run --rm --detach --publish 9222:9222 docker.io/chromedp/headless-shell:latest
```

Then run the program from the root of the repository:

```sh
$ go run ./remote
```

The flag `-url` sets the URL of the browser. The default is
`ws://127.0.0.1:9222`. Do not use it with `-start`.

```sh
$ go run ./remote -url ws://127.0.0.1:9222
```

The flag `-nav` sets the page to read. The default is the home page of the local
test site. The local site listens on `127.0.0.1` of this computer, so a browser
in a container cannot reach it. Give such a browser a page that it can reach
with this flag. The program then does not start the local site.
