# About example standalone

This is a version of the [`simple`][1] example but uses the `chromedp.WithTargets`
option with `chromedp/client.WatchPageTargets` to connect to an existing Chrome
instance (ie, no process will be launched).

## Manually Starting Chrome

To use this example, please manually start a Chrome instance with the `--remote-debugging-port=9222`
command-line option make the Chrome Debugging Protocol available to clients:

```sh
# start google-chrome
$ google-chrome --remote-debugging-port=9222

# start headless_shell
$ headless_shell --headless --remote-debugging-port=9222

# start google-chrome-unstable in headless mode
$ google-chrome-unstable --headless --remote-debugging-port=9222
```

### Docker Image

A Docker image, [knqz/chrome-headless][2], provides a small ready-to-use
`headless_shell` that can be used with this example:

```sh
# retrieve docker image
$ docker pull knqz/chrome-headless

# start chrome-headless
$ docker run -d -p 9222:9222 --rm --name chrome-headless knqz/chrome-headless
```

[1]: ../simple
[2]: https://hub.docker.com/r/knqz/chrome-headless/

## Building and Running

The `standalone` example can be run like any other Go code:

```sh
# run example
$ cd $GOPATH/src/github.com/chromedp/examples/standalone
$ go build && ./standalone
```
