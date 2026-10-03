# About chromedp examples

This repository holds 23 example programs for [`chromedp`][1], a Go package that
drives Chrome through the Chrome DevTools Protocol. The package documentation
has small examples that are self-contained. These programs are larger. Some of
them need the internet or an external component.

The programs use the new typed API of `chromedp`. The file `docs/API.md` in the
`chromedp` repository shows the old code and the new code side by side. The
file `docs/MIGRATION.md` in the same folder lists every name that changed. The
API is a proposal and the maintainer has not approved it. See
[the decision for the port](docs/decisions/2026-10-03-the-programs-use-the-new-typed-api.md).

## Build and run

The module needs Go 1.27. The typed API is not in a released version of
`chromedp`, and `go.mod` still names the released versions. A build needs a
`go.work` file in the root of this repository. Git ignores that file. For
example:

```
go 1.27.1

use (
	.
	../chromedp
)

replace (
	github.com/chromedp/cdproto => ../cdproto-typed
	github.com/chromedp/chromedp v0.16.0 => ../chromedp
)
```

The folder `../chromedp` must hold the branch `typed-api` of `chromedp`. The
folder `../cdproto-typed` must hold the typed `cdproto`.

`chromedp` starts the browser. It finds Chrome or Chromium on the `PATH`. Run a
program from the root of this repository:

```sh
# run the program <prog>
$ go run ./<prog>

# build the program <prog>
$ go build -o /tmp/<prog> ./<prog> && /tmp/<prog>
```

Every program accepts the flag `-v`. It prints the protocol messages between
the program and the browser. Without `-v`, a program prints only its own output.
For example, `go run ./cookie -v` shows the protocol messages. Run
`go run ./<prog> -h` to see all flags of a program.

Every program except `remote` accepts the flag `-visible`. It shows the browser
window instead of a headless browser, and it leaves the browser open when the
program ends. The program prints the websocket address and the profile directory
of the browser to the standard error. The browser stays running until you close
its window, and the profile directory stays on disk until you delete it. To show
the window with no flag, set the variable `CHROMEDP_VISIBLEWINDOW=1`. On Linux,
a visible window needs `DISPLAY` or `WAYLAND_DISPLAY`. The `remote` program has
no `-visible` flag, because it attaches to a browser that you started yourself.
The flag `-visible` has no effect on the remote browser of `forecast -remote`.

Some programs need arguments:

- `forecast` needs the flag `-q`, for example `go run ./forecast -q Jakarta`.
- `geoip` takes IP addresses, for example `go run ./geoip 8.8.8.8`.
- `multi` takes URLs. See [multi/README.md](multi/README.md).
- `remote` needs a running browser. See [remote/README.md](remote/README.md).
- `upload` uploads its own source file, so you can run it from any folder.

## The programs

<!-- the following section is updated by running `go run gen.go` -->
<!-- START EXAMPLES -->
| Example                           | Description                                                                                     |
|-----------------------------------|-------------------------------------------------------------------------------------------------|
| [click](/click)                   | use a selector to click on an element                                                           |
| [cookie](/cookie)                 | set an HTTP cookie on requests                                                                  |
| [download_file](/download_file)   | do headless file downloads                                                                      |
| [download_image](/download_image) | do headless image downloads                                                                     |
| [emulate](/emulate)               | emulate a specific device such as an iPhone                                                     |
| [eval](/eval)                     | evaluate JavaScript and retrieve the result                                                     |
| [fast](/fast)                     | measure the speed of the internet connection and show the result in the terminal                |
| [forecast](/forecast)             | render the weather forecast of Google in the terminal                                           |
| [geoip](/geoip)                   | look up the location of an IP address and show its map in the terminal                          |
| [headers](/headers)               | add extra HTTP headers to browser requests                                                      |
| [keys](/keys)                     | send key events to an element                                                                   |
| [latlon](/latlon)                 | retrieve the latitude and the longitude from Google Maps with the navigation events of the page |
| [logic](/logic)                   | combine actions and Go code in a function that reads a list from a page                         |
| [multi](/multi)                   | use headless-shell and a container (Docker, Podman, other)                                      |
| [pdf](/pdf)                       | capture a PDF of a page                                                                         |
| [proxy](/proxy)                   | authenticate to a proxy server that requires authentication                                     |
| [remote](/remote)                 | connect to an existing Chrome DevTools instance using a remote WebSocket URL                    |
| [screenshot](/screenshot)         | take a screenshot of a specific element and of the entire browser viewport                      |
| [submit](/submit)                 | fill out and submit a form                                                                      |
| [subtree](/subtree)               | populate and travel a subtree of the DOM                                                        |
| [text](/text)                     | extract text from a specific element                                                            |
| [upload](/upload)                 | upload a file on a form                                                                         |
| [visible](/visible)               | wait until an element is visible                                                                |
<!-- END EXAMPLES -->

The programs `fast`, `geoip` and `remote` draw an image in the terminal. Run
them in a terminal that can show images.

## Verification

The table shows what happened when each program ran, with Chrome 154 on Linux.
"Offline" means that the program needs no internet and no service. The column
"Old" is the program on the branch `main` with `chromedp` v0.16.0. The column
"New" is the program on this branch.

| Example         | Needs                            | Old                                  | New                                  |
|-----------------|----------------------------------|--------------------------------------|--------------------------------------|
| click           | internet (pkg.go.dev)            | fails, timeout after 15 seconds      | same                                 |
| cookie          | offline                          | works                                | works, same output                   |
| download_file   | internet (github.com)            | fails, waits for a download event    | fails, times out after 60 seconds    |
| download_image  | internet (githubusercontent.com) | works, 38371 bytes                   | works, 38371 bytes                   |
| emulate         | internet (whatsmyua.info)        | works, same file sizes               | works, same file sizes               |
| eval            | internet (google.com)            | works                                | works                                |
| fast            | internet (fast.com), terminal    | fails at the end, no terminal image  | same                                 |
| forecast        | internet (google.com)            | fails, timeout                       | same                                 |
| geoip           | internet (google.com maps)       | lookup works, map times out          | same                                 |
| headers         | offline                          | works                                | works, same output                   |
| keys            | offline                          | works                                | works, same output                   |
| latlon          | internet (google.com maps)       | works                                | works, same output                   |
| logic           | internet (github.com)            | works                                | works, same output                   |
| multi           | offline with a `data:` URL       | works                                | works, same files                    |
| pdf             | internet (google.com)            | works                                | works                                |
| proxy           | offline                          | works                                | works, same requests                 |
| remote          | a Chrome with a debugging port   | works up to the terminal image       | same                                 |
| screenshot      | internet (pkg.go.dev, brank.as)  | works                                | works, same files                    |
| submit          | internet (wikipedia.org)         | not run                              | works                                |
| subtree         | offline                          | works                                | works, same output                   |
| text            | internet (pkg.go.dev)            | works                                | works, same output                   |
| upload          | offline                          | works                                | works, same output                   |
| visible         | offline                          | works                                | works, same output                   |

Notes:

1. The programs `fast`, `geoip` and `remote` draw the image with `rasterm`. In
   the test the output was not a terminal. The old code and the new code ended
   with the error `term graphics not available`.
2. The programs `geoip` and `forecast` embed their data files, `GeoLite2-City.mmdb`
   and `hl.json`, so they run from any folder.
3. The `remote` test used `chrome --headless --remote-debugging-port=9222` and a
   local web server. Without the flag `-nav`, `remote` reads the internet.

## Live sites can change

Most programs read live websites such as `pkg.go.dev`, `github.com` and
`google.com`. When a site changes its HTML, the selectors of a program stop
matching, and the program fails. The results in the table can be different on
another day.

## Questions, bugs and changes

Ask questions in the [GitHub Discussions of `chromedp`][2]. Report a bug as an
[issue of `chromedp`][3]. Do not open an issue for a question. Pull requests
are welcome, and `CONTRIBUTING.md` says how to prepare one. A new program must
keep to the rules in `AGENTS.md`.

| Document | Holds |
| --- | --- |
| [AGENTS.md](AGENTS.md) | the rules, the layout and the commands for a coding agent and for a person |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to prepare a change |
| [docs/decisions/README.md](docs/decisions/README.md) | the index of the decisions |

[1]: https://github.com/chromedp/chromedp
[2]: https://github.com/chromedp/chromedp/discussions
[3]: https://github.com/chromedp/chromedp/issues
