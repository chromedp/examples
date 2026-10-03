# About

This example shows how to use a standalone Go binary with
[the `docker.io/chromedp/headless-shell` container][docker-hub] to take a
screenshot of each of several websites.

[docker-hub]: https://hub.docker.com/r/chromedp/headless-shell/tags

## Running

Run these commands from the root of the repository:

```sh
# build the container
$ podman build -f multi/Dockerfile --tag localhost/multi .

# make the output directory
$ mkdir out

# run the example
$ podman run --rm --volume ./out:/out localhost/multi 'https://www.google.com/' 'https://ifconfig.me'

# list the output
$ ls out
0.png  1.png
```

To run the program without a container, give it URLs and the flag `-out`. The
`data:` URLs need no internet:

```sh
$ go run ./multi -out out 'data:text/html,<h1>hello</h1>'
```
