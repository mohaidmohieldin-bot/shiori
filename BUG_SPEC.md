# BUG_SPEC: `print --json` reports `HasArchive: false` for archived bookmarks

- **Issue:** go-shiori/shiori#1140
- **Reported on:** Shiori 1.8.0-rc.1, SQLite, Windows 10, portable CLI
- **Still present on:** `master` at `aebb402`

## Current Behavior

When a bookmark has an offline archive saved, the web interface shows the archive, but `shiori print --json` (also `-j`) prints `"HasArchive": false` for that bookmark. Every bookmark in the output shows `false`, whether or not an archive exists.

The same happens to three related fields in the same output:

| Field | Value printed today |
|---|---|
| `HasArchive` | always `false` |
| `HasEbook` | always `false` |
| `HasThumbnail` | always `false` |
| `ImageURL` | always `""` |

All other fields (ID, URL, title, tags and so on) print correctly.

## Expected Behavior

Each of these fields reports whether the matching file exists in the Shiori data directory for that bookmark. The values must match what the web interface and API v1 report for the same bookmark.

| Field | Must be `true` (or set) exactly when this path exists under the data directory |
|---|---|
| `HasArchive` | `archive/<bookmark id>` (a file) |
| `HasEbook` | `ebook/<bookmark id>.epub` (a file) |
| `HasThumbnail` | `thumb/<bookmark id>` (a file) |
| `ImageURL` | equals `/bookmark/<bookmark id>/thumb` when the thumbnail exists; empty otherwise |

Nothing else about `print` changes:

- The plain-text output (without `--json`) looks the same.
- `--latest`, `--search`, `--tags`, `--exclude-tags`, `--index-only` and index arguments select and order the same bookmarks as today.

## Target Area

These four fields are **derived, not stored**. No database column holds them. The bookmarks domain computes them by checking the filesystem after reading from the database. The `print` command skips that step.

| File (at `aebb402`) | Function / symbol | Role in the bug |
|---|---|---|
| `internal/cmd/print.go` | `printHandler` (line 34) | **Where it goes wrong.** Builds `model.DBGetBookmarksOptions` (line 58) and calls `deps.Database().GetBookmarks` (line 66), the raw database layer. Then marshals the results to JSON as they are (line 86). |
| `internal/domains/bookmarks.go` | `BookmarksDomain.SearchBookmarks` (line 81) | The domain-level read. Calls the same database query, then runs `populateAdditionalFields` on each result. `print` never calls it. |
| `internal/domains/bookmarks.go` | `BookmarksDomain.populateAdditionalFields` (line 34) | Sets `HasEbook`, `HasArchive`, `HasThumbnail`, then `PopulateImageURL`. The only place these fields get filled. |
| `internal/domains/bookmarks.go` | `HasArchive`, `HasEbook`, `HasThumbnail` (lines 18–31) | Each checks for a file via the storage domain. |
| `internal/domains/storage.go` | `StorageDomain.FileExists` (line 37) | True only when the path exists and is a file, not a directory. |
| `internal/model/bookmark.go` | `BookmarkDTO` (line 25); `GetArchivePath`, `GetEbookPath`, `GetThumbnailPath`, `GetThumbnailURL` | The output struct (fields default to `false`/`""`) and the path helpers. |
| `internal/http/handlers/api/v1/bookmarks.go` | bookmarks list handler (line 742) | Shows the working path: API v1 calls `SearchBookmarks`, so it reports correct values. |

**Why the failure happens:** a Go struct starts with every boolean `false` and every string empty. The database query only fills the stored columns. Because `print` uses the database query directly, nothing ever sets the four derived fields, so JSON marshalling writes their zero values.

The legacy web API (`internal/webserver/handler-api.go`, around line 94) computes the same fields its own way, which is why the web interface shows archives correctly. It isn't involved in this bug.

## Reproduction Case

Language-agnostic steps. Every precondition and check below can be scripted.

**Setup**

1. Create an empty temporary directory, `DATA`.
2. Initialise a Shiori SQLite database at `DATA/shiori.db`.
3. Save two bookmarks:
   - **A:** URL `https://example.com/with-files`
   - **B:** URL `https://example.com/without-files`
4. Note their database IDs, `A_ID` and `B_ID`.
5. Create these three non-empty files for **A only**:
   - `DATA/archive/<A_ID>`
   - `DATA/ebook/<A_ID>.epub`
   - `DATA/thumb/<A_ID>`

**Action**

Run `shiori --storage-directory DATA print --json`, with `SHIORI_DATABASE_URL` and `SHIORI_DBMS` unset so SQLite inside `DATA` is used. Capture standard output.

**Checks**

| ID | Check | Today | Expected |
|---|---|---|---|
| C1 | Output parses as JSON: a list of exactly 2 bookmarks | pass | pass |
| C2 | A: `HasArchive == true` | **fail** | pass |
| C3 | A: `HasEbook == true` | **fail** | pass |
| C4 | A: `HasThumbnail == true` | **fail** | pass |
| C5 | A: `ImageURL == "/bookmark/<A_ID>/thumb"` | **fail** | pass |
| C6 | B: `HasArchive`, `HasEbook`, `HasThumbnail` all `false` | pass | pass |
| C7 | B: `ImageURL == ""` | pass | pass |

C6 and C7 guard against a change that turns every flag on; C1 guards the output shape.
