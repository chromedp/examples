# Using

Start a Chrome instance:

Using `headless-shell`:

```sh
$ podman run --rm --detach --publish 9222:9222 docker.io/chromedp/headless-shell:latest
```

Alternately, using Google Chrome:

```sh
$ google-chrome-stable --remote-debugging-protocol=9222
```

Then, execute the script:

```sh
$ go run main.go
```

The remote URL can be specified using the `-url` flag:

```sh
$ go run main.go -url ws://127.0.0.1:9222
```
