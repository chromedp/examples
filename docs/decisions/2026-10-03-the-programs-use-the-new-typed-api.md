# The programs use the new typed API

Status: Proposed.

The maintainer asked on 2026-10-03 for the programs of this repository to use
the new typed API of `chromedp`. The API is a proposal and nobody has approved
it. See `2026-10-03-generic-iterator-api-instead-of-action.md` in the `chromedp`
repository. This decision is Proposed for the same reason. If the maintainer
does not approve the API, the port goes back.

## What changed

All 23 programs moved from the old API to the typed API on the branch
`typed-api` of `chromedp`. There is one commit for each program, named "Port
the <name> example to the typed API". The old code used `Tasks`, `ActionFunc`,
`ListenTarget`, the `By` options and a pointer for each result. The new code
uses `chromedp.Do` and `chromedp.Run`, which return the value of an action. It
reads events with `chromedp.Events` and `chromedp.WaitEvent`. It sends a raw
protocol command with `cdp.Call`.

Two later commits follow the typed `cdproto`. The `headers` program sends
`network.Headers`, which is a map. The `download_image` program uses the body
that the typed `network.GetResponseBody` result already decodes.

Other commits set the `go` line of `go.mod` to 1.27, rewrote the comments, and
updated the documents.

No program changed its flags, its output or its logic. `go.mod` still names the
released versions of `chromedp` and `cdproto`, so a build needs a `go.work`
file that points at the local copies. Git ignores that file.

## How it was verified

Each program ran twice with Chrome 154 on Linux. The first run used the old
code on the branch `main` with `chromedp` v0.16.0. The second run used the new
code. The table in `README.md` has the result for each program.

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

- The maintainer must approve or reject the API.
- When `chromedp` and `cdproto` release the typed API, name those versions in
  `go.mod`. The `go.work` file is then not needed.
- The cause of the failure of `download_file` is not known.
