**Title:** `fix: populate HasArchive/HasEbook/HasThumbnail in print --json (#1140)`

---

## What does this PR do?

Fixes #1140 by having `print` read bookmarks through the bookmarks domain (`SearchBookmarks`) instead of the raw database query, so the file-derived fields `HasArchive`, `HasEbook`, `HasThumbnail` and `ImageURL` are filled in.

## How was this tested?

* Added test case: `Test_printHandler_JSONIncludesFileFlags` to cover `print --json` with one bookmark that has archive, ebook and thumbnail files and one that has none.
* Output: Pending. Run via `go test -race -count=1 -tags osusergo,netgo,fts5 ./internal/cmd/ -run Test_printHandler`; CI on this PR runs the full suite.

## Diff Summary

* `internal/cmd/print.go`: call `Domains().Bookmarks().SearchBookmarks` instead of `Database().GetBookmarks`; same filters and ordering.
* `internal/cmd/print_test.go`: Added reproduction test
