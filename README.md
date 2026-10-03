# About chromedp examples

This folder holds examples for [`chromedp`][1]. The package documentation has
simple examples that are self-contained. These examples are larger. Many need
internet access or an external component.

These examples use the new typed API of `chromedp`. See the section "The new
API" below.

The examples can break. Most of them read live websites such as `pkg.go.dev`,
`github.com` and `google.com`. When a site changes its HTML, the selectors of
the example stop to match. To report a problem, use the [chromedp issue
tracker][3].

## Building and Running an Example

The module needs Go 1.27. The typed API is not in a released version of
`chromedp` yet. The `go.mod` file still names the old versions, so a build
needs a `go.work` file that points at local copies of `chromedp` and `cdproto`
on the branch `typed-api`. The file is not part of the repository.

If a program needs a browser, `chromedp` looks for `google-chrome`, `chromium`
or `chrome` on the `PATH`. Run an example from the root of this repository:

```sh
# run example <prog>
$ go run ./<prog>

# build example <prog>
$ go build -o /tmp/<prog> ./<prog> && /tmp/<prog>
```

The programs `upload` and `geoip` read files, so run `upload` from its own
directory with `cd upload && go run .`.

### Available Examples

The following examples are currently available:

<!-- the following section is updated by running `go run gen.go` -->
<!-- START EXAMPLES -->
| Example                           | Description                                                                         |
|-----------------------------------|-------------------------------------------------------------------------------------|
| [click](/click)                   | use a selector to click on an element                                               |
| [cookie](/cookie)                 | set a HTTP cookie on requests                                                       |
| [download_file](/download_file)   | do headless file downloads                                                          |
| [download_image](/download_image) | do headless image downloads                                                         |
| [emulate](/emulate)               | emulate a specific device such as an iPhone                                         |
| [eval](/eval)                     | evaluate javascript and retrieve the result                                         |
| [fast](/fast)                     | extract and render data from a page                                                 |
| [forecast](/forecast)             | extract and render data from a page                                                 |
| [geoip](/geoip)                   | extract and render data from a page                                                 |
| [headers](/headers)               | add extra HTTP headers to browser requests                                          |
| [keys](/keys)                     | send key events to an element                                                       |
| [latlon](/latlon)                 | retrieve the latitude/longitude from google maps, using the browser's target events |
| [logic](/logic)                   | more complex logic beyond simple actions                                            |
| [multi](/multi)                   | use headless-shell and a container (Docker, Podman, other)                          |
| [pdf](/pdf)                       | capture a pdf of a page                                                             |
| [proxy](/proxy)                   | authenticate a proxy server which requires authentication                           |
| [remote](/remote)                 | connect to an existing Chrome DevTools instance using a remote WebSocket URL        |
| [screenshot](/screenshot)         | take a screenshot of a specific element and of the entire browser viewport          |
| [submit](/submit)                 | fill out and submit a form                                                          |
| [subtree](/subtree)               | populate and travel a subtree of the DOM                                            |
| [text](/text)                     | extract text from a specific element                                                |
| [upload](/upload)                 | upload a file on a form                                                             |
| [visible](/visible)               | wait until an element is visible                                                    |
<!-- END EXAMPLES -->

## The new API

The examples use the generic action API. An action returns its value, so no
program passes a pointer to receive it. `chromedp.Do` runs actions that return
nothing, and `chromedp.Run` runs one action and returns its value. A program
reads events with the iterators `chromedp.Events` and `chromedp.WaitEvent`. A
program sends a raw protocol command with `cdp.Call`. A selector is a string or
a typed value such as `chromedp.CSS`, `chromedp.ID` and `chromedp.NodeIDs`.

The file `docs/API.md` in the `chromedp` repository shows 13 examples of the old
code and the new code side by side. The file `docs/MIGRATION.md` in the same
directory lists every changed name.

Three things are good to know when you read these examples:

1. `network.Headers` has no fields in the typed `cdproto`. The `headers` example
   sends the command `Network.setExtraHTTPHeaders` with `Target.Call` and a map.
2. `network.GetResponseBody` does not decode the body. The `download_image`
   example decodes the base64 text itself.
3. `chromedp.Events` starts the browser if the context has none. A browser that
   starts this way lives only as long as the context that you pass. The `proxy`
   example calls `chromedp.Do(ctx)` first for this reason.

## Verification

The table below shows what happens when each program runs, with Chrome 154 on
Linux. "Offline" means that the program needs no internet and no service. The
column "Old" is the program at the `main` branch with `chromedp` v0.16.0. The
column "New" is the program at this branch.

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
| submit          | internet (github.com)            | fails, waits for a search result     | same                                 |
| subtree         | offline                          | works                                | works, same output                   |
| text            | internet (pkg.go.dev)            | works                                | works, same output                   |
| upload          | offline                          | works                                | works, same output                   |
| visible         | offline                          | works                                | works, same output                   |

Notes:

1. The programs `fast`, `geoip` and `remote` draw an image with `rasterm`. This
   needs a terminal that can show images. In the test the output was not a
   terminal, so the programs ended with the error `term graphics not available`
   in the old and the new code.
2. The program `geoip` needs the file `GeoLite2-City.mmdb`, and `forecast` needs
   the file `hl.json`. Both are in the repository.
3. The `remote` test used `chrome --headless --remote-debugging-port=9222` and a
   local web server.
4. The live sites can change at any time, so the results of the live programs
   can differ on another day.

## Contributing

Pull Requests and contributions to this project are encouraged and greatly
welcomed! The `chromedp` project always needs new examples, and needs talented
developers (such as yourself!) to submit fixes for the existing examples when
they break (for example, when a website's layout/HTML changes).

[1]: https://github.com/chromedp/chromedp
[2]: https://pkg.go.dev/github.com/chromedp/chromedp#pkg-examples
[3]: https://github.com/chromedp/chromedp/issues
