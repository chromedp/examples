# The examples use a local test site

Status: Decided.

The maintainer decided on 2026-10-04 that every example program except `fast` uses local content. A program no longer reads a live site such as `pkg.go.dev`, GitHub, Wikipedia, Google or an IP lookup service.

## Context

Many programs read a live site. A live site changes, goes down, blocks a browser that it does not know and needs a network. A change of the markup of one site breaks the program, and the program then shows nothing useful. A program also cannot be checked in a build with no internet.

A tiny page does not solve this. A program that takes a screenshot, prints a PDF, scrolls, reads text, clicks or submits a form needs a page that is as large and as busy as a real one.

## Decision

The repository has a shared test site in `internal/testsite`. It serves long, real looking pages from embedded files and from Go code. The pages cover a package reference, an encyclopedia with a search, a repository with a ZIP download, an image gallery, weather forecasts, an IP lookup, a map with tiles, a page for device emulation and a report for printing. All content is original, and no page loads a font, an image or a script from the internet.

Every program uses the test site instead of a live site, with one exception. The program `fast` keeps reading the internet. It measures a real connection, and the maintainer runs it on remote systems.

A program that reads the test site takes the flag `-url`. The flag holds the base address of a site. When the flag is empty, the program starts the test site and uses it. A person can give the address of a live site to see the program work on the real thing. The live site must then have the same structure.

The doc comment of such a program says "It starts a local server and needs no internet." The rules of `AGENTS.md` for a program that runs offline apply.

## Consequences

- A program works without a network, and a build with no internet can run it.
- A page of the test site does not change, so the output of a program is stable and a person can compare it with the expected output.
- A change of a page in `internal/testsite` can change a program. A person who edits a page must run the tests of the package and the programs that use the page.
- The test site is code that this repository must keep. `internal/testsite/README.md` lists every route and says what it is for.
- A binary asset is made by `internal/testsite/gen/main.go`. A person who changes it must run it and commit the new files.
