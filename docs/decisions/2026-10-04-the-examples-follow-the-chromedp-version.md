# The examples follow the version of chromedp

Status: Decided.

The maintainer decided on 2026-10-04 that the tags of this repository follow
the tags of `chromedp`. The tag `v0.18.0` of this repository holds programs
that build with `chromedp` v0.18.0. A later tag `v0.18.1` holds a change of the
programs or of the documents, and keeps the same minor version. When `chromedp`
gets the minor version 0.19, this repository gets the tag `v0.19.0` after the
`go.mod` file requires it.

## What it means

- The minor version of a tag here is the minor version of `chromedp` that the
  programs use. The patch version is for changes in this repository.
- A new tag follows a change of the `go.mod` file to a new minor version of
  `chromedp`, and the change of the programs that the new version needs.
- The maintainer makes the tags. An agent does not.

## Tests

The workflow `.github/workflows/test.yml` builds and vets all the programs on
every push and pull request. The workflow `.github/workflows/nightly.yml` does
the same every night at 05:17 UTC, against the newest `main` of `chromedp` and
of its `remote` module, and the newest `cdproto`. Neither workflow runs the
programs. Most programs read a live website. When one fails, the cause can be a
site that changed, and not a fault in this repository. A build that fails shows
a real break of the API.
