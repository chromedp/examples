# About

This example demonstrates using a standalone Go binary with the [the
`docker.io/chromedp/headless-shell` container][docker-hub] to scrape multiple
websites.

[docker-hub]: https://hub.docker.com/r/chromedp/headless-shell/tags

## Running

```sh
# build container
$ cd /path/to/chromedp/examples
$ podman build -f multi/Dockerfile --tag localhost/multi .

# make output directory
$ mkdir out

# run example
$ podman run --rm --volume ./out:/out localhost/multi 'https://www.google.com/' 'https://ifconfig.me'

# list the output
$ ls out
0.png  1.png
```
