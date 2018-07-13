# About example standalone

This is a version of the [`simple`](../simple) example but demonstrates how
connect to a standalone (ie, existing) Chrome instance with `chromedp`.

## Manually Starting Chrome

To use this example, manually start a Chrome instance with the
`--remote-debugging-port=9222` command-line option, in order to make the
browser instance available via the Chrome DevTools Protocol on port `9222` to
clients:

```sh
# start google-chrome
$ google-chrome --remote-debugging-port=9222

# start headless_shell
$ headless_shell --headless --remote-debugging-port=9222

# start google-chrome-unstable in headless mode
$ google-chrome-unstable --headless --remote-debugging-port=9222
```

### Docker Image

A Docker image, [chromedp/headless-shell][docker-hub], provides a small
ready-to-use `headless_shell` that can be used with this example:

```sh
# retrieve docker image
$ docker pull chromedp/headless-shell

# start headless-shell
$ docker run -d -p 9222:9222 --rm --name headless-shell chromedp/headless-shell
```

## Building and Running

The `standalone` example can be run like any other Go code:

```sh
# run example
$ cd $GOPATH/src/github.com/chromedp/examples/standalone
$ go build && ./standalone
```

[docker-hub]: https://hub.docker.com/r/chromedp/headless-shell/
