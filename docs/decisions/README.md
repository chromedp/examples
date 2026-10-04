# Decisions

Every decision of this project is a file in this folder. A file is named by its
date and a short title, as in `2026-10-03-the-programs-use-the-new-typed-api.md`.
This table is the index.

Each file opens with its status. "Decided" means that the maintainer chose it.
"Proposed" means that somebody suggested it and the maintainer has not
confirmed it. "Open" means that nobody has chosen yet. A decision that changes
an earlier one says so in its status, as "Amends <file>". The earlier one says
it back, as "Superseded by <file>". Read the status before the decision.

Refer to a decision by its file name, never by a number. A new decision is a
new file with the date of the decision. Add its row here. `go test ./docs/`
fails when a decision has no row or a row is wrong, and it prints the row to
add.

| Date | Decision | Status |
| --- | --- | --- |
| 2026-10-03 | [The programs use the new typed API](2026-10-03-the-programs-use-the-new-typed-api.md) | Decided |
| 2026-10-04 | [The examples follow the version of chromedp](2026-10-04-the-examples-follow-the-chromedp-version.md) | Decided |
| 2026-10-04 | [The examples use a local test site](2026-10-04-the-examples-use-a-local-test-site.md) | Decided |
