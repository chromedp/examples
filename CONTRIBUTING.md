# Contributing to the chromedp examples

This repository holds example programs for `chromedp`. Read these three things
before you change anything.

[`AGENTS.md`](AGENTS.md) holds the rules, the layout and the commands. It is
written for a coding agent, and everything in it applies to a person.

[`README.md`](README.md) says how to build and run a program. It also says which
programs need the internet.

[`docs/decisions/`](docs/decisions/README.md) holds every decision, one file
each, named by date. Read the status of a decision before you trust it.

## Before you send a change

```sh
gofmt -l .
go build ./...
go vet ./...
go test ./docs/
```

`gofmt -l .` must print nothing. `go test ./docs/` needs no browser. If you
change the doc comment of a program, run `go run gen.go` and include the new
table in `README.md`.

## Writing

Write in plain English. Use short sentences and the active voice. The
`simple-english` skill in `.agents/skills` has the full rules. `go test ./docs/`
finds the violations that a machine can find, in the documents and in the Go
comments.

To record a decision, add a file to `docs/decisions/`. Name it with the date, as
in `2026-10-03-short-title.md`. Then add its row to the index. The test prints
the row for you.

## Questions

Ask in the [GitHub Discussions of `chromedp`](https://github.com/chromedp/chromedp/discussions),
and not in an issue. The issue tracker is for bugs.
