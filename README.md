# About chromedp examples

This repository holds 23 example programs for [`chromedp`][1], a Go package that
drives Chrome through the Chrome DevTools Protocol. The package documentation
has small examples that are self-contained. These programs are larger. Some of
them need the internet or an external component.

The programs use the typed API of `chromedp` v0.17.1 and `cdproto` v0.157.3.
The file `docs/API.md` in the `chromedp` repository shows the old code and the
new code side by side. The file `docs/MIGRATION.md` in the same folder lists
every name that changed. See
[the decision for the port](docs/decisions/2026-10-03-the-programs-use-the-new-typed-api.md).

## Build and run

The module needs Go 1.27. It requires `chromedp` v0.17.1 and `cdproto`
v0.157.3, and Go downloads them when you build a program.

`chromedp` starts the browser. It finds Chrome or Chromium on the `PATH`. If
Chrome has another name, link it to the name `google-chrome` in a folder on the
`PATH`. Run a program from the root of this repository:

```sh
# run the program <prog>, for example go run ./click
$ go run ./<prog>

# build the program <prog>
$ go build -o /tmp/<prog> ./<prog> && /tmp/<prog>
```

Every program accepts the flag `-v`. It prints the protocol messages between
the program and the browser. Without `-v`, a program prints only its own output.
For example, `go run ./cookie -v` shows the protocol messages. Run
`go run ./<prog> -h` to see all flags of a program. Put the flags of a program
after its folder, as in `go run ./multi -out out <url>`.

Every program except `remote` accepts the flag `-visible`. It shows the browser
window instead of a headless browser, and it leaves the browser open when the
program ends. The program prints the websocket address and the profile directory
of the browser to the standard error. The browser stays running until you close
its window, and the profile directory stays on disk until you delete it. To show
the window with no flag, set the variable `CHROMEDP_VISIBLEWINDOW=1`. Any value
other than an empty string, `0` and `false` turns it on. On Linux, a visible
window needs `DISPLAY` or `WAYLAND_DISPLAY`. The `remote` program has
no `-visible` flag, because it attaches to a browser that you started yourself.
The flag `-visible` has no effect on the remote browser of `forecast -remote`.

Some programs need arguments:

- `forecast` needs the flag `-q`, for example `go run ./forecast -q Jakarta`.
- `geoip` takes IP addresses, for example `go run ./geoip 8.8.8.8`.
- `multi` takes URLs. See [multi/README.md](multi/README.md).
- `remote` needs a running browser. See [remote/README.md](remote/README.md).
- `upload` uploads its own source file, so you can run it from any folder.

The programs `download_file`, `download_image`, `emulate`, `pdf` and
`screenshot` write files into the current directory. The programs `fast` and
`forecast` write a file only when you give the flag `-out`, and `multi` does so
only with its flag `-out`.

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

The programs `fast`, `forecast`, `geoip` and `remote` draw an image in the
terminal. Run them in a terminal that can show images.

## Verification

The table says which programs run with the current API, and what each program
needs. "Offline" means that the program needs no internet and no service. A
program that needs a live site reads it from the internet. A program that needs
a terminal draws an image with `rasterm`. Every program runs in a headless
Chrome unless you set the flag `-visible`, so no program in the table needs a
display.

The column "Checked" says how the result was found. "Run on 2026-10-04" means that
the offline programs ran with `go run`, with Chrome 154 on Linux, and finished
with exit code 0. "Earlier test" means a run of the same code with Chrome 154 on
Linux before that date. These programs were not run again on 2026-10-04,
because they need a live site that can change. No program was run with the flag
`-visible`.

| Example         | Needs                            | Result with the current API                       | Checked           |
|-----------------|----------------------------------|---------------------------------------------------|-------------------|
| click           | internet (pkg.go.dev)            | fails, times out after 15 seconds                 | earlier test      |
| cookie          | offline                          | works                                             | run on 2026-10-04 |
| download_file   | internet (github.com)            | fails, times out after 60 seconds                 | earlier test      |
| download_image  | internet (githubusercontent.com) | works, writes 38371 bytes                         | earlier test      |
| emulate         | internet (whatsmyua.info)        | works                                             | earlier test      |
| eval            | internet (google.com)            | works                                             | earlier test      |
| fast            | internet (fast.com), terminal    | fails at the end without a terminal image         | earlier test      |
| forecast        | internet (google.com), terminal  | fails, times out                                  | earlier test      |
| geoip           | internet (google.com maps)       | the lookup works, the map times out               | earlier test      |
| headers         | offline                          | works                                             | run on 2026-10-04 |
| keys            | offline                          | works                                             | run on 2026-10-04 |
| latlon          | internet (google.com maps)       | works                                             | earlier test      |
| logic           | internet (github.com)            | works                                             | earlier test      |
| multi           | offline with a `data:` URL       | works                                             | run on 2026-10-04 |
| pdf             | internet (google.com)            | works                                             | earlier test      |
| proxy           | offline                          | works                                             | run on 2026-10-04 |
| remote          | a Chrome with a debugging port   | works up to the terminal image                    | earlier test      |
| screenshot      | internet (pkg.go.dev, brank.as)  | works                                             | earlier test      |
| submit          | internet (wikipedia.org)         | works                                             | earlier test      |
| subtree         | offline                          | works                                             | run on 2026-10-04 |
| text            | internet (pkg.go.dev)            | works                                             | earlier test      |
| upload          | offline                          | works                                             | run on 2026-10-04 |
| visible         | offline                          | works                                             | run on 2026-10-04 |

Notes:

1. The programs `fast`, `geoip` and `remote` draw the image with `rasterm`. In
   the earlier test the output was not a terminal, and each program ended with
   the error `term graphics not available`. Run them in a terminal that can show
   images.
2. The programs `geoip` and `forecast` embed their data files, `GeoLite2-City.mmdb`
   and `hl.json`, so they run from any folder.
3. The `remote` test used `chrome --headless --remote-debugging-port=9222` and a
   local web server. Without the flag `-nav`, `remote` reads the internet.
4. The failures of `click`, `download_file` and `forecast` can come from a
   change of the live site. The cause is not known.

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
