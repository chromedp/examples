# The local test site

The package `github.com/chromedp/examples/internal/testsite` is a web site that runs on your machine. It serves long, real looking pages, so that an example program has something real to screenshot, print, scroll, read, click and submit. It needs no internet. See `docs/decisions/2026-10-04-the-examples-use-a-local-test-site.md` for the reason.

## Use it in a program

```go
site := testsite.New()
defer site.Close()
err := chromedp.Do(ctx, chromedp.Navigate(site.URL+"/docs/time"))
```

`testsite.New` starts the site on a free port of `127.0.0.1` and returns a `*testsite.Site`. The field `URL` holds the address without a trailing slash. The field `OtherURL` holds the same server under the name `localhost`, so a browser treats it as another origin and another site. `Close` stops the site. `testsite.Handler` returns the `http.Handler` for a program that wants its own listener.

A program that reads the site takes a flag `-url` with the base address. When the flag is empty, the program starts the test site and uses its address.

## Routes

Every page has a header with a menu, a footer and the style sheet `/static/site.css`. Every page uses relative links, so it works under `URL` and under `OtherURL`.

| Route | What it is | Used by |
| --- | --- | --- |
| `/` | The home page. It links to every other page and has `img.Homepage-logo`. | `screenshot`, `eval` |
| `/tools` | An index of the pages for lookups, devices and printing. | |
| `/docs/` | The start page of a package reference. | |
| `/docs/time` | A reference of the package `time`, in the style of a package documentation page. It has `.Documentation-overview`, an index, 65 sections, and 14 `details#example-<Name>.Documentation-exampleDetails` elements, each with a `summary` and a `textarea.Documentation-exampleCode`. `#example-After` holds a program that uses `time.After`. | `click`, `text`, `eval` |
| `/wiki/` | The main page of the encyclopedia, with 32 articles in 8 topics. | |
| `/article/<Slug>` | An article. `Harbour_Line` has more than 5000 words, 14 sections, a numbered contents list, an infobox, 11 figures, a table of 60 rows and 46 references. The other 31 articles are shorter. Titles and slugs come from `wiki.go`. | `submit`, `screenshot` |
| `/search?q=<words>&page=<n>` | The search. A form with `#searchInput` is in the header of every wiki page. Results are `ul.mw-search-results > li.mw-search-result` with 10 on a page. The page has `#firstHeading` and `#mw-content-text`. `/wiki/Special:Search` redirects here. | `submit` |
| `/repo/` | A list of 41 projects in 8 sections. The structure is `div > h3`, then `ul > li > a` followed by the text ` - description`. The star count is in `data-stars`. | `logic` |
| `/repo/<owner>/<name>` | A repository page with a file table, a README, and the button `<button><span>Code</span></button>`. The click opens a dropdown with `<a href=".../archive/main.zip" download><span>Download ZIP</span></a>`. `/repo/chromedp/examples` is the full page. | `download_file` |
| `/repo/<owner>/<name>/archive/main.zip` | A ZIP that the server builds, with `Content-Disposition: attachment`. It holds an image, a table of data and sources, and it is larger than 300 KB. | `download_file` |
| `/gallery` | 24 pictures in JPEG, PNG and SVG, from 400 to 2000 pixels wide, with `loading="lazy"` and `srcset`. | `screenshot` |
| `/images/<name>` | One file, with the right `Content-Type` and `Content-Length`. `?w=400` gives a smaller copy of a JPEG or PNG. `gallery-01.jpg` is larger than 1 MB. `photo.png` is 800 by 600 pixels. `chart-*.svg` are charts. | `download_image` |
| `/weather/` | A list of 8 cities. | |
| `/weather/<city>` | A forecast. The cities are `jakarta`, `london`, `new-york`, `tokyo`, `sydney`, `nairobi`, `reykjavik` and `mumbai`. See the section Weather. | `forecast` |
| `/weather?q=<city>` | The search form. It redirects to the page of the city and keeps the other parameters. | `forecast` |
| `/api/geoip?ip=<address>` | JSON with `ip`, `range`, `country`, `country_code`, `region`, `city`, `latitude`, `longitude`, `timezone`, `organization` and `accuracy_radius_km`. It answers 404 for an address outside the 200 ranges and 400 for a bad address. Without `ip` it uses the address of the caller. | `geoip` |
| `/api/geoip/ranges` | The 200 ranges as JSON. | |
| `/geoip` | A page to look up an address, with a link to the map. | `geoip` |
| `/map?lat=&lon=&zoom=` | A map with `#app-container`, `#map`, `#zoom-in` and `#zoom-out`. See the section Map. | `geoip`, `latlon` |
| `/tiles/<z>/<x>/<y>.png` | A 256 by 256 tile that the server draws. The zoom goes from 0 to 19. | |
| `/ua` and `/whoami` | The user agent, the viewport, the device pixel ratio and the touch support, in `#ua`, `#viewport`, `#dpr` and `#touch`. | `emulate` |
| `/viewport-test` | A page that has three layouts, at 480 and 1099 pixels. `.vp-label` names the active one. | `emulate` |
| `/news/` | A news front page, "The Harbour Courier", with a lead story, 7 story cards, a sidebar and five ad slots. See the section News and ads. | `extension` |
| `/news/<slug>` | One story, with an ad slot inside the text. The slugs are in `news.go`, for example `harbour-line-night-trains`. | `extension` |
| `/print/report` | A report with a cover, 8 chapters and 8 tables. See the section Print. | `pdf` |
| `/print/report?paper=css` | The same report with a fixed A4 paper size and margins, and a header and a page number in the margin boxes. | `pdf` |
| `/studio` | A tall landing page of a design studio. | `screenshot` |
| `/static/...` | Style sheets, scripts, icons, badges and the logo. | |

## Print

`/print/report` has a title block at the top of the first page, a page break before each chapter, and tables with a header row that repeats on each page (`thead { display: table-header-group }`). Its style sheet `print.css` sets no `@page` rule. The program that prints chooses the paper, the orientation and the margins with the options `PDFPaper`, `PDFLandscape` and `PDFMargins`, and it can add a header and a footer with `PDFHeaderTemplate` and `PDFFooterTemplate`. The report has about 9 pages on Letter paper and 17 pages on A4 with margins of 0.9 inch.

The page has no fixed header or footer. Chrome does not support running elements, and an element with `position: fixed` repeats at the same place on each page but takes no room. It would land on the content.

`/print/report?paper=css` loads the second style sheet `print-a4.css` after `print.css`. It has the rule `@page { size: A4; margin: 24mm 16mm 22mm }` and the margin boxes `@top-center` and `@bottom-center`, which Chrome supports from version 131. They print the title and the text `Page N of M` inside the margins, so they never cover the content. Chrome uses the paper size of the rule only with `PDFPreferCSSPageSize`. An `@page` size also makes `PDFLandscape` and `PDFPaper` have no effect, so only this page has it. The margin boxes add to the templates of `PDFHeaderTemplate` and `PDFFooterTemplate`, so a program uses one of the two ways.

## News and ads

The news pages are the one exception to the rule that a page never refers to another host. A content blocker only has something to block when a page asks for the real hosts of the ad networks, so the pages do. The markup is the one that a real site uses:

- `#ad-top` and `#ad-bottom` are `.ad-slot.ad-leaderboard` with an `ins.adsbygoogle`.
- `#ad-sidebar` is `.sidebar-ad` with `#div-gpt-ad-1700000000000-0`, which the Google Publisher Tag fills.
- `#ad-box` is `.ad-slot.ad-box` with a second `ins.adsbygoogle`.
- `.advert-banner` holds `img.ad-image`, which comes from `ad.doubleclick.net`.
- The scripts come from `pagead2.googlesyndication.com`, `securepubads.g.doubleclick.net` and `www.googletagmanager.com`, and the pixel `img.pixel` comes from `www.google-analytics.com`.

Without internet, the requests to these hosts fail and the slots stay empty. A program that needs filled slots answers the requests itself with the Fetch domain. The test `TestNewsPage` makes sure that the news pages refer to no other host than these five.

## Weather

The state of a forecast lives in three attributes of `#wob_wc`: `data-unit` (`c` or `f`), `data-type` (`temp`, `rain` or `wind`) and `data-day` (`0` to `7`). A script sets them, and the style sheet shows the matching parts. The query parameters `unit=f`, `type=rain`, `day=3` and `hl=id` set the first state and the page language.

- `#taw` is the header with the search form.
- `#wob_wc` is the data block. `#wob_loc` holds the city, `#wob_tm` the temperature, `#wob_pp`, `#wob_hm` and `#wob_ws` the details, and `#wob_dts` and `#wob_dc` the time and the condition.
- The spans with the text `°C` and `°F` switch the unit. They have `role="button"`.
- `#wob_temp`, `#wob_rain` and `#wob_wind` are the tabs. They have `role="tab"` and the `aria-label` values Temperature, Precipitation and Wind. The charts are inline SVG.
- `#wob_dp` is the table of 8 days. Each day has a button `[data-wob-di="0"]` to `[data-wob-di="7"]`.

## Map

The map lays out the tiles with absolute positions. A user can drag it, double click it, use the wheel, use the buttons `#zoom-in` and `#zoom-out`, and use the arrow keys and the keys plus and minus when the map has focus. While tiles load, `#map` has `data-loading` with the number of pending tiles, and it is `0` when the map is ready.

The page writes the center into the address with `history.replaceState`, once at load and again 120 ms after each move. The address has the form `/map?lat=-6.20880&lon=106.84560&zoom=11.00#@-6.20880,106.84560,11.00z`. The fragment follows the style of Google Maps. The browser sends `Page.navigatedWithinDocument` for each change. A change of the fragment from outside moves the map.

## Generated files

`gen/main.go` writes the binary and the vector files: `images/photo.png`, `images/gallery-01` to `images/gallery-24`, `images/chart-*.svg`, `images/hero.svg`, `static/icons/*.svg`, `static/badges/*.svg`, `static/logo.svg` and `static/favicon.svg`. The output is the same on each run. Run it from the root of the repository, and commit the files that it writes:

```sh
GOWORK=off go run internal/testsite/gen/main.go
```

The pages, the articles, the forecasts, the IP ranges, the tiles and the archive are made in Go when the server starts or when a request arrives. They come from fixed seeds, so every run serves the same content.

## Tests

`go test ./internal/testsite/` needs no browser. It loads each page with an HTTP client. It checks the status, the landmarks that the programs use, the sizes, the unique ids and that no page refers to another host.
