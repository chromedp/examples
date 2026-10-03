# The programs use the new typed API

Status: Decided.

The maintainer asked on 2026-10-03 for the programs of this repository to use
the new typed API of `chromedp`. See
`2026-10-03-generic-iterator-api-instead-of-action.md` in the `chromedp`
repository. The maintainer approved the API and this port, and the work was
merged on 2026-10-04. The programs use `chromedp` v0.17.0 and `cdproto`
v0.157.2.

## What changed

All 23 programs moved from the old API to the typed API. The work was done on a
branch with one commit for each program, named "Port the <name> example to the
typed API". The branch `typed-api` of `chromedp` held the typed API for the
port. The maintainer merged the work into `main` on 2026-10-04, and the branch
no longer exists. The old code used `Tasks`, `ActionFunc`,
`ListenTarget`, the `By` options and a pointer for each result. The new code
uses `chromedp.Do` and `chromedp.Run`, which return the value of an action. It
reads events with `chromedp.Events` and `chromedp.WaitEvent`. It sends a raw
protocol command with `cdp.Call`.

Two later commits follow the typed `cdproto`. The `headers` program sends
`network.Headers`, which is a map. The `download_image` program uses the body
that the typed `network.GetResponseBody` result already decodes.

Other commits set the `go` line of `go.mod` to 1.27, rewrote the comments, and
updated the documents.

A later commit added the flag `-v` to every program, and the flag `-visible` to
every program except `remote`. It also changed `submit` to search Wikipedia.
No other program changed its logic.

## How it was verified

Each program ran twice with Chrome 154 on Linux. The first run used the old
code on the branch `main` with `chromedp` v0.16.0. The second run used the new
code. The table in `README.md` has the result for each program of the new code.
On 2026-10-04 the offline programs ran again, and they all finished with exit
code 0.

- The offline programs `cookie`, `headers`, `keys`, `subtree`, `upload` and
  `visible` printed the same output as before. `multi` wrote the same files,
  and `proxy` sent the same requests and logged one `Fetch.authRequired` event.
- The live programs `download_image`, `emulate`, `eval`, `latlon`, `logic`,
  `pdf`, `screenshot` and `text` worked as before. `download_image` wrote 38371
  bytes in both runs.
- Some programs failed in both runs in the same way. `click` timed out after 15
  seconds, `forecast` timed out, and `submit` waited for a search result. `fast`,
  `geoip` and `remote` ended with `term graphics not available`, because the
  output was not a terminal. The map of `geoip` timed out.
- `download_file` failed in both runs in a different way. The old program waited
  for a download event. The new program timed out after 60 seconds. Nobody has
  found the cause.

## What remains

- The cause of the failure of `download_file` is not known.
