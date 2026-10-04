# About chromedp examples

<p align="center">
  <img src="https://raw.githubusercontent.com/chromedp/logo/main/chromedp.svg" alt="chromedp logo" width="160">
</p>

This repository holds 43 example programs for [`chromedp`][1], a Go package that
drives Chrome through the Chrome DevTools Protocol. The package documentation
has small examples that are self-contained. These programs are larger. Some of
them need the internet or an external component.

[![Unit Tests][examples-ci-status]][examples-ci]
[![Go Reference][goref-chromedp-status]][goref-chromedp]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

The programs use the typed API of `chromedp` v0.19.0 and `cdproto` v0.157.5.
The file `docs/API.md` in the `chromedp` repository shows the old code and the
new code side by side. The file `docs/MIGRATION.md` in the same folder lists
every name that changed. See
[the decision for the port](docs/decisions/2026-10-03-the-programs-use-the-new-typed-api.md).

## Build and run

The module needs Go 1.27. It requires `chromedp` v0.19.0, `cdproto` v0.157.5 and
`chromedp/remote` v0.1.0, and Go downloads them when you build a program. The
programs use `remote` for the flag `-visible`, which keeps the browser open, and
for the `remote` program.

The tags of this repository follow the tags of `chromedp`. The tag `v0.19.0` holds
programs that use `chromedp` v0.19, and a tag such as `v0.19.1` is a later change in
this repository.

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
The `tabs` program works a little differently with `-visible`. It keeps every tab
open and waits until you close the browser, and it does not leave the browser
running. Its flag `-tabs-in-one-window` opens the tabs in one window, as real
tabs, and switches between them with the protocol command `Target.activateTarget`.
Without that flag, each tab opens in a window of its own. Try both with
`-visible` to see the difference.

Some programs need arguments:

- `forecast` needs the flag `-q`, for example `go run ./forecast -q Jakarta`.
- `geoip` takes IP addresses, for example `go run ./geoip 8.8.8.8`.
- `multi` takes URLs. See [multi/README.md](multi/README.md).
- `remote` needs a running browser. See [remote/README.md](remote/README.md).
- `upload` uploads its own source file, so you can run it from any folder.

The programs `download_file`, `download_image`, `emulate`, `har`, `pdf`,
`pdfstream` and `screenshot` write files into the current directory. The
programs `fast` and `forecast` write a file only when you give the flag `-out`,
and `multi` does so only with its flag `-out`. The programs `pdfoptions` and
`screencast` write their files into a new temporary directory, or into the
directory of their flag `-out`.

## The programs

<!-- the following section is updated by running `go run gen.go` -->
<!-- START EXAMPLES -->
| Example                           | Description                                                                                                                              |
|-----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------|
| [click](/click)                   | use a selector to click on an element                                                                                                    |
| [console](/console)               | read the console of a page and its uncaught exceptions with chromedp                                                                     |
| [cookie](/cookie)                 | set an HTTP cookie on requests                                                                                                           |
| [dialogs](/dialogs)               | handle the JavaScript dialogs of a page                                                                                                  |
| [download_file](/download_file)   | do headless file downloads                                                                                                               |
| [download_image](/download_image) | do headless image downloads                                                                                                              |
| [dragdrop](/dragdrop)             | drag and drop with the actions DragAndDrop and DragAndDropXY                                                                             |
| [emulate](/emulate)               | emulate a specific device such as an iPhone                                                                                              |
| [eval](/eval)                     | evaluate JavaScript and retrieve the result                                                                                              |
| [eventsiter](/eventsiter)         | listen to the events of a page with iterators                                                                                            |
| [exposefunc](/exposefunc)         | call Go functions from a page with chromedp                                                                                              |
| [extension](/extension)           | install a browser extension, uBlock Origin Lite, into the browser that chromedp starts, and how to show that it blocks the ads of a page |
| [fast](/fast)                     | measure the speed of the internet connection and show the result in the terminal                                                         |
| [forecast](/forecast)             | render the weather forecast of a city in the terminal                                                                                    |
| [frames](/frames)                 | reach elements inside an iframe and inside a shadow root                                                                                 |
| [geoip](/geoip)                   | look up the location of an IP address and show its map in the terminal                                                                   |
| [har](/har)                       | generate a HAR file from the network events of a page                                                                                    |
| [headers](/headers)               | add extra HTTP headers to browser requests                                                                                               |
| [intercept](/intercept)           | block, mock and change the requests of a page with the Fetch domain                                                                      |
| [keys](/keys)                     | send key events to an element                                                                                                            |
| [latlon](/latlon)                 | retrieve the latitude and the longitude of a map from the URL of the page with the navigation events of the page                         |
| [logic](/logic)                   | combine actions and Go code in a function that reads a list from a page                                                                  |
| [multi](/multi)                   | use headless-shell and a container (Docker, Podman, other)                                                                               |
| [pdf](/pdf)                       | capture a PDF of a page                                                                                                                  |
| [pdfoptions](/pdfoptions)         | print a page to PDF files with different options of chromedp                                                                             |
| [pdfstream](/pdfstream)           | print a page to a PDF file with a stream                                                                                                 |
| [popups](/popups)                 | work with the popups of a page and with several targets                                                                                  |
| [proxy](/proxy)                   | authenticate to a proxy server that requires authentication                                                                              |
| [rawcall](/rawcall)               | send protocol commands that chromedp has no action for                                                                                   |
| [remote](/remote)                 | connect to an existing Chrome DevTools instance using a remote WebSocket URL                                                             |
| [screencast](/screencast)         | record the screen of a page as a series of JPEG images                                                                                   |
| [screenshot](/screenshot)         | take a screenshot of a specific element and of the entire browser viewport                                                               |
| [selectors](/selectors)           | choose the elements of a page with the typed selectors                                                                                   |
| [session](/session)               | save the state of a session and restore it in another browser                                                                            |
| [structeval](/structeval)         | evaluate JavaScript into typed Go values                                                                                                 |
| [submit](/submit)                 | fill out and submit a form                                                                                                               |
| [subtree](/subtree)               | populate and travel a subtree of the DOM                                                                                                 |
| [tabs](/tabs)                     | use several tabs of one browser                                                                                                          |
| [termcast](/termcast)             | stream the screen of the browser to the terminal with terminal graphics                                                                  |
| [text](/text)                     | extract text from a specific element                                                                                                     |
| [upload](/upload)                 | upload a file on a form                                                                                                                  |
| [visible](/visible)               | wait until an element is visible                                                                                                         |
| [workers](/workers)               | run many jobs at the same time in one browser with a pool of goroutines                                                                  |
<!-- END EXAMPLES -->

The programs `fast`, `forecast`, `geoip` and `remote` draw an image in the
terminal. Run them in a terminal that can show images. Most other programs have
the flag `-visible-on-terminal`, which the section [Draw the page in the
terminal](#draw-the-page-in-the-terminal) describes.

## Draw the page in the terminal

Most programs take the flag `-visible-on-terminal`. It draws the page of the
program in the terminal while the program runs, with terminal graphics (Kitty,
iTerm2 or Sixel). The flag also works with a headless browser over `ssh`. The
package [`termcast`][termcast] does this work. It uses the screencast of the
Chrome DevTools Protocol and 4 frames each second by default. Use the flag
`-terminal-fps` to change the rate.

```
go run github.com/chromedp/examples/click@latest -visible-on-terminal
```

The stream clears the terminal at each redraw. It holds the log lines and the
results of the program, and prints them after the final frame. The flag does not
work with `-v`, and it stops with an error when the terminal has no graphics.
The programs `tabs`, `popups`, `workers`, `multi`, `session`, `screencast`,
`pdfstream`, `har` and `rawcall` do not have the flag. The program `termcast`
shows how to use the package in your own code. It plays an animated SVG and
streams it to the terminal.

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
| click           | offline                          | works                                             | run on 2026-10-04 |
| console         | offline                          | works                                             | run on 2026-10-04 |
| cookie          | offline                          | works                                             | run on 2026-10-04 |
| dialogs         | offline                          | works                                             | run on 2026-10-04 |
| download_file   | offline                          | works                                             | run on 2026-10-04 |
| download_image  | offline                          | works                                             | run on 2026-10-04 |
| dragdrop        | offline                          | works                                             | run on 2026-10-04 |
| emulate         | offline                          | works                                             | run on 2026-10-04 |
| eval            | offline                          | works                                             | run on 2026-10-04 |
| eventsiter      | offline                          | works                                             | run on 2026-10-04 |
| exposefunc      | offline                          | works                                             | run on 2026-10-04 |
| extension       | offline, the uBlock Origin Lite files on disk | works                                             | run on 2026-10-04 |
| fast            | internet (fast.com), terminal    | needs a terminal that shows images                | earlier test |
| forecast        | terminal                         | works in a terminal, stops without one            | run on 2026-10-04 |
| frames          | offline                          | works                                             | run on 2026-10-04 |
| geoip           | terminal                         | works in a terminal, stops without one            | run on 2026-10-04 |
| har             | offline                          | works                                             | run on 2026-10-04 |
| headers         | offline                          | works                                             | run on 2026-10-04 |
| intercept       | offline                          | works                                             | run on 2026-10-04 |
| keys            | offline                          | works                                             | run on 2026-10-04 |
| latlon          | offline                          | works                                             | run on 2026-10-04 |
| logic           | offline                          | works                                             | run on 2026-10-04 |
| multi           | offline                          | works                                             | run on 2026-10-04 |
| pdf             | offline                          | works                                             | run on 2026-10-04 |
| pdfoptions      | offline                          | works                                             | run on 2026-10-04 |
| pdfstream       | offline                          | works                                             | run on 2026-10-04 |
| popups          | offline                          | works                                             | run on 2026-10-04 |
| proxy           | offline                          | works                                             | run on 2026-10-04 |
| rawcall         | offline                          | works                                             | run on 2026-10-04 |
| remote          | a Chrome with a debugging port   | works up to the terminal image                    | earlier test      |
| screencast      | offline                          | works                                             | run on 2026-10-04 |
| screenshot      | offline                          | works                                             | run on 2026-10-04 |
| selectors       | offline                          | works                                             | run on 2026-10-04 |
| session         | offline                          | works                                             | run on 2026-10-04 |
| structeval      | offline                          | works                                             | run on 2026-10-04 |
| submit          | offline                          | works                                             | run on 2026-10-04 |
| subtree         | offline                          | works                                             | run on 2026-10-04 |
| tabs            | offline                          | works                                             | run on 2026-10-04 |
| termcast        | offline, a terminal              | works in a terminal, stops without one            | run on 2026-10-04 |
| text            | offline                          | works                                             | run on 2026-10-04 |
| upload          | offline                          | works                                             | run on 2026-10-04 |
| visible         | offline, a window                | works                                             | earlier test      |
| workers         | offline                          | works                                             | run on 2026-10-04 |

Notes:

1. The programs `forecast` and `geoip` draw an image with `rasterm`. Without a
   terminal that shows images, they stop with the error `term graphics not
   available`. `fast` does the same, and it reads the live site `fast.com`.
2. The programs `geoip` and `forecast` embed their data files, `GeoLite2-City.mmdb`
   and `hl.json`, so they run from any folder.
3. `remote` needs a Chrome that runs with a debugging port, for example
   `chrome --headless --remote-debugging-port=9222`. Use the flag `-start` to
   let `remote` start that Chrome.
4. Every other program reads the local test site in `internal/testsite`. Use the
   flag `-url` to read another site.
5. `extension` needs the files of uBlock Origin Lite on disk. Use the flag
   `-ext`.

## Live sites can change

Only `fast` reads a live site. The result can be different on another day.

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
[termcast]: https://github.com/chromedp/termcast
[2]: https://github.com/chromedp/chromedp/discussions
[3]: https://github.com/chromedp/chromedp/issues
[examples-ci]: https://github.com/chromedp/examples/actions/workflows/test.yml (Test CI)
[examples-ci-status]: https://github.com/chromedp/examples/actions/workflows/test.yml/badge.svg (Test CI)
[goref-chromedp]: https://pkg.go.dev/github.com/chromedp/chromedp
[goref-chromedp-status]: https://pkg.go.dev/badge/github.com/chromedp/chromedp.svg
[release-status]: https://img.shields.io/github/v/release/chromedp/examples?display_name=tag&sort=semver (Latest Release)
[releases]: https://github.com/chromedp/examples/releases (Releases)
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"
