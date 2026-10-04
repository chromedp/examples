# chromedp examples

This repository holds 41 example programs for `chromedp`, a Go package that
drives Chrome through the Chrome DevTools Protocol. The programs are larger
than the examples in the package documentation. They show how to solve a task
with `chromedp`: click an element, download a file, emulate a device, use a
proxy and more. The module is `github.com/chromedp/examples`.

The programs use the typed API of `chromedp` v0.19.0 and `cdproto` v0.157.5. See
`docs/decisions/2026-10-03-the-programs-use-the-new-typed-api.md`.

## Standing rules

These hold in every `chromedp` repository, for every coding agent.

1. Stage changes for review. Commit and push only when the maintainer says so.
2. Load the `simple-english` skill before you write text that a person reads.
   Examples are a document, a code comment, an error message and a commit
   message. Follow the skill for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

Questions and feature ideas go to GitHub Discussions of `chromedp/chromedp`,
and bugs go to issues. Do not open an issue for a question.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

`docs/decisions/README.md` is the index of every decision. A decision is one
file named by its date and a short title. Read the status of a decision before
you trust it, because a later decision can amend or replace it.

| If you are | Read |
| --- | --- |
| looking for what the repository is, and how to build and run a program | `README.md` |
| asking which program needs the internet and which runs offline | the verification table in `README.md` |
| changing a program for the typed API | `docs/API.md` and `docs/MIGRATION.md` in the `chromedp` repository |
| asking why the programs use the typed API | `docs/decisions/2026-10-03-the-programs-use-the-new-typed-api.md` |
| asking why something is the way it is | the index in `docs/decisions/README.md` |
| recording a decision | `docs/decisions/README.md`, and the section Writing documentation in this file |
| preparing a change as a person | `CONTRIBUTING.md` |
| adding or changing a program | the sections Hard rules and Before you commit in this file |
| running a program against its expected output | the section Verify an offline program in this file |

## Hard rules

1. Each program is one folder with one `main.go`. The folder can also hold a
   data file or a `README.md` that the program needs.
2. A program uses only the public API of `chromedp` and the packages of
   `cdproto`. Do not copy code from the `chromedp` repository into a program.
3. Keep a program small and readable. A reader must follow it from the top.
   Prefer a plain loop and a plain call over a clever helper.
4. Keep the flags of a program. Do not rename a flag, remove it or change its
   default without asking the maintainer.
5. Start the doc comment above `package main` with `Command <name> is a
   chromedp example demonstrating how to`. Name the task in that first
   sentence. `gen.go` reads it for the table in `README.md`. The sentence must
   not hold a period before its end.
6. A program that reads a live site says in its doc comment which site it
   needs, with the words "It reads <site>." A program that needs a service, a
   file or a terminal that can show images says so too.
7. A program that runs offline serves its own page from a local server. Do not
   make it read the internet. Its doc comment says "It starts a local server and
   needs no internet."
8. Wrap every error with `%w`. Write error messages in lower case, and do not
   start them with "failed to".
9. After you change a doc comment, run `go run gen.go` and commit the new table
   in `README.md`.
10. Do not run a program in a session that has no browser. Run `go build ./...`,
    `go vet ./...` and `go test ./docs/` only.
11. Never put a password, a key or a token in a file or in a message.
12. Every program has the flag `-v` and calls `flag.Parse()`. With `-v`, the
    program adds `chromedp.WithDebugf(log.Printf)` to the options of
    `chromedp.NewContext`. Without `-v`, it prints no protocol messages. Every
    program except `remote` also has the flag `-visible`. With `-visible`, the
    program adds `chromedp.WithVisibleWindow()` and `chromedp.WithKeepOpen()` to
    the same options, and it prints `chromedp.KeptOpen(ctx)` to the standard
    error. A program that builds its own allocator, such as `proxy`, adds
    `chromedp.VisibleWindow` and `chromedp.KeepOpen` to the options of the
    allocator instead. Say this in the doc comment, with the words "Use -v to
    print the protocol messages and -visible to show the browser window and
    leave it open." Use the help text "print the protocol messages" for the flag
    `-v`, and "show the browser window and leave it open" for the flag
    `-visible`.

## Layout

| Path | Holds |
| --- | --- |
| `<name>/main.go` | one example program, for each of the 41 programs |
| `forecast/hl.json` | the language codes that `forecast` embeds |
| `geoip/GeoLite2-City.mmdb` | the IP database that `geoip` embeds |
| `geoip/README.md`, `multi/README.md`, `remote/README.md` | the usage notes of the program |
| `multi/Dockerfile` | the container image of `multi` |
| `gen.go` | writes the table of the programs in `README.md`. Run `go run gen.go` |
| `docs/decisions/` | the decisions and their index |
| `docs/docs_test.go` | the test of the documents and the Go comments |
| `.agents/skills/` and `.claude/skills/` | the two agent skills, as copies |
| `skills-lock.json` | the source of each skill |
| `go.mod`, `go.sum` | the module, which needs Go 1.27 |

The root of the repository holds `README.md`, `AGENTS.md`, `CLAUDE.md`,
`CONTRIBUTING.md` and `LICENSE` as text documents. Every other document goes in
`docs/`.

## Build and run

Run these commands in the repository root.

```sh
go build ./...
go vet ./...
go run ./<name>
```

Add `-v` to any program to print the protocol messages. Add `-visible` to show
the browser window and leave it open. The variable `CHROMEDP_VISIBLEWINDOW=1`
shows the window with no flag. A visible window needs a display.

Go downloads `chromedp` v0.19.0 and `cdproto` v0.157.5 when it builds a program.
Do not edit `go.mod` or `go.sum` unless the maintainer asks.

A program that needs a browser starts it. If Chrome is not on the `PATH` under
the name `google-chrome`, `chromium` or `chrome`, link it there:

```sh
TMP=$(mktemp -d); ln -s /opt/google/chrome/chrome $TMP/google-chrome
PATH=$TMP:$PATH go run ./<name>
```

## Verify an offline program

The offline programs are `console`, `cookie`, `dialogs`, `dragdrop`,
`eventsiter`, `exposefunc`, `frames`, `har`, `headers`, `intercept`, `keys`,
`multi`, `pdfoptions`, `pdfstream`, `popups`, `proxy`, `rawcall`, `screencast`,
`selectors`, `session`, `structeval`, `subtree`, `upload`, `visible` and
`workers`. Run one with the command above and a time limit, for example `timeout 90 go run ./cookie`. It must finish with exit code 0. Compare the output with the table that follows. Timestamps, ports and the
paths of temporary files differ on each run. Do not run `-visible` in a session
that has no display.

| Program | Expected output |
| --- | --- |
| `console` | the heading `all the messages:` with the messages in order. Each one has a line with its type and text and a line with its place. They are `log page loaded version 3`, `log user {name: "Ada", age: 36}`, a `warning`, an `error` with `code 7`, the `error` of the missing image with `[network]`, the exception `Uncaught Error: boom` with two `stack:` lines, the exception `Uncaught (in promise) Error: nobody handles this` and `log end of the run`. Then the heading `only the errors and the exceptions:` with the missing image and the two exceptions again, then `stopped: context deadline exceeded` |
| `cookie` | the log lines `server received cookie 0: cookie1=value1` and `cookie 1: cookie2=value2`, each one twice, then two `chrome cookie` lines and `chrome received cookies` |
| `dialogs` | four dialogs: `alert dialog, message "Hello from the page": accepted`, `confirm gave false`, `prompt gave Ada` and a `beforeunload` dialog with an empty message, then `the browser is now on the page that says "the next page"` |
| `dragdrop` | three lines for the slider (`value 75`, `value 34` and `value 100`, with the handle at 277, 127 and 370 px), the list order after two drags (`beta, gamma, alpha, delta`, then `delta, beta, gamma, alpha`), two lines for the HTML5 drops (`"buy milk"` on `done` and `"call Ann"` on `later`, with the data type `text/plain`), and the card count of each zone |
| `eventsiter` | the line `the network was idle after 1s`, the 5 requests and the 5 responses (the browser asks for `/favicon.ico` by itself), the first three console messages, and `the message of the click: console.log: "clicked" 7` |
| `exposefunc` | eight lines about what the page got: `an object in and out: Ada lives in London and knows go and javascript`, `sum(1, 2, 3.5) = 6.5`, the Go error as a rejected promise with `"division by zero"`, `no user with the id "u9"`, five calls at the same time with `squares 1, 4, 9, 16, 25`, `the largest number of calls that ran together in Go: 5`, the call after the navigation, and the line `in the iframe: the iframe got Budi from Jakarta` |
| `frames` | five lines about the same-site iframe and the shadow root (`title: "Page of 127.0.0.1"`, `"clicked"` and `"started"`), then the lines for the cross-site iframe, with `document in the node tree: false`, a failed query, and `title from its own target: "Page of localhost"` |
| `har` | `wrote out.har (N bytes) with 4 entries`, the page timings, and one line for each of `/`, `/logo.png`, `/app.js` and `/api/data?id=1`, all with status 200. It writes `out.har` in the current directory |
| `headers` | one log line `received headers:` that lists the headers, with `X-Header` and the value `my request header` |
| `intercept` | the page text, with `user: Ada Lovelace (from the program)`, `analytics script ran: false` and `image loaded: false`, then the lines `blocked 2`, `mocked 1` and `continued 1` |
| `keys` | the values of `#input1`, `#textarea1`, `#input2` and `#select1`, which are `test4`, a text that starts with `textar`, `test3` and `three` |
| `multi` | run it with `-out <dir> data:text/html,<h1>hello</h1>`. It prints `image 0 (...) width: 780 height: 437` and writes `<dir>/0.png` |
| `pdfoptions` | the line `writing the PDF files in <dir>` and one line for each of 10 files, from `default.pdf` to `css-page-off.pdf`. Each line has the page count and the page size, for example `default.pdf 5 pages 612 x 792 pt`, `landscape.pdf` with 792 x 612, `scale-2.pdf` with 10 pages, `pages-2-3.pdf` with 2 pages, `outline-tagged.pdf` with `outline true` and `css-page-size.pdf` with 432 x 288. It writes the files in the directory of `-out`, or in a new temporary directory |
| `pdfstream` | `wrote out.pdf: N bytes` after `read the stream with N calls of IO.read`, then `out.pdf starts with %PDF`. It writes `out.pdf` in the current directory |
| `popups` | two lines `popup of the link: title "popup of the link"` and `popup of the button: ...` with a target ID, then `a tab in the same browser context sees: session=abc`, `a tab in a new browser context sees: no cookie`, and the line about the first request of the new tab, which got the page `"answered by the program"` |
| `proxy` | no stdout. The log shows `proxy: not authorized` for the first request, then the requests with `Proxy-Authorization: Basic dTpw` |
| `rawcall` | the method name of one command, the page before and after the time zone and locale overrides (`America/New_York`, `1.234.567,891`, `navigator.language ... id-ID`), `permission geolocation: granted`, the position `latitude:-6.2088 longitude:106.8456`, the CPU slowdown (about 4 times), the latency and offline results, and two lines with `Browser.getVersion` |
| `screencast` | `saving the frames in <dir>`, then about `30 frames in 3s, 10.0 frames per second`. It writes `frame-0001.jpg` and more files in `<dir>` |
| `selectors` | one line for each selector type, such as `CSS("li.fruit"): "Apple"`, `CSSAll("li.fruit"): 3 nodes`, `NodeIDs(the second of 3 ids): "Banana"`, `ByFunc(the last li.fruit): "Cherry"`, then `AtLeast(2): 2 rows`, `AtLeast(0) on .missing: 0 elements` and `NodeVisible on the hidden item: context deadline exceeded` |
| `session` | the login line, `saved the state in <file> (N bytes)` with the cookie `session, HttpOnly: true`, then `second browser before the restore: please log in` and `second browser after the restore: hello ada, theme dark, cart 3 items`. It writes the state in a temporary file, or in the file of `-state` |
| `structeval` | one line for each Go type (`struct: Ada, 36, [go javascript], lives in London`, `slice of structs: 3 items`, `map:`, `number: 3, decimal number: 43.75, boolean: true`), then the errors (`exception: Uncaught, description: TypeError: ...`, `ErrJSUndefined: true`, `a string into int`), and `a promise with AwaitPromise: "slow answer"` |
| `subtree` | the tree of the element `h1`, with its attributes and its children `a`, `span` and a text |
| `upload` | it logs `original size: N, upload size: N` with the same number twice |
| `visible` | the log lines `waiting 3s for box to become visible`, `BOX1 IS VISIBLE` and `BOX2 IS VISIBLE`, after about 4 seconds |
| `workers` | one line for each of the 8 jobs (7 with a title and a value, and `job 8, delay 5s: timed out after 2s`), `8 jobs, 4 workers, 7 finished, 1 failed`, the sum of the delays and of the times of the jobs, and the total time with a speedup of about 2 times |

A live program has no fixed output, because the site changes. The verification
table in `README.md` says what each program did and how it was checked.

## Before you commit

```sh
gofmt -l .
go build ./...
go vet ./...
go test ./docs/
```

`gofmt -l .` must print nothing. `go test ./docs/` needs no browser. It tests
the links, the decision index, the document tables and the skill copies. It
also tests the table of the programs in `README.md`. It applies the prose rules
to the documents and to the Go comments.

## Continuous integration and tags

The workflow `.github/workflows/test.yml` builds and vets every program on each
push and pull request. The workflow `.github/workflows/nightly.yml` does it every
night at 05:17 UTC against the newest `main` of `chromedp` and of `chromedp/remote`,
and the newest `cdproto`. Neither runs a program, because most programs read live
websites. Do not add a test that runs a program in CI.

The tags follow `chromedp`. The tag `v0.19.x` holds programs that use `chromedp`
v0.19.x, and the patch number counts the changes of this repository. See
`docs/decisions/2026-10-04-the-examples-follow-the-chromedp-version.md`. Do not
create a tag. The maintainer does.

## Writing documentation

Follow the `simple-english` skill for every word. Write sentences of 20 words
or fewer for a procedure, and 25 words or fewer for a description.

A new document goes in `docs/`. Add it to the tables in `README.md` and in this
file, and `go test ./docs/` fails if you do not.

Record a decision in a file in `docs/decisions/`. Name it
`YYYY-MM-DD-short-slug.md` with the date of the decision. Open it with
`# <Title>`, a blank line and `Status: Decided.`. Use `Proposed.`, `Open.`,
`Amends <file>.` or `Superseded by <file>.` when that is the status. Refer to a
decision by its file name, never by a number. State an amendment in both files.
Add a row to `docs/decisions/README.md`. The test prints the row for you.

The skills live in `.agents/skills` and `.claude/skills` as identical copies.
Never replace a copy with a symbolic link. `skills-lock.json` names the source
of each skill. The Claude Code permissions of one person go in
`.claude/settings.local.json`, which `.gitignore` lists.
