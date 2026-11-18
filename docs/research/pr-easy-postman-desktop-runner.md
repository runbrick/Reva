# Desktop collection runs

Open a collection’s root **Overview**, choose **Run collection**, then select
**Start run**. The dialog defaults to **No environment** and continuing after a
failure. The selected saved environment overrides collection variables. Start
snapshots the saved collection, selected environment, and saved transport
settings. The run leaves the editor drafts and saved data untouched.

The desktop and CLI share depth-first execution, transport and script behavior,
stop-on-failure, and the v1 report projection. The desktop projects each result
as soon as its request ends. It retains one compact entry per request, including
skipped requests, and Viva lays out only visible result rows. The dialog displays
fixed outcomes, request and assertion counts, ordinals, and HTTP status codes.
Ordinal positions follow the saved collection’s depth-first order at Start.

**Cancel run** and Escape request cancellation and keep the sheet open until the
partial report is available. The Viva page context also cancels the run when the
window closes. The sheet replaces the workspace’s paint so a visible report
contains only the summary. Previously authorized sends, WebSockets, downloads,
and settings effects keep their existing declarations during this display.

## Verification

- `TestCollectionRunnerSavedInputsAndReportParity` uses a real store and local
  HTTP to verify explicit start, root scope, current saved data, environment
  selection, depth-first order, failure handling, v1 parity, and unchanged
  persistence bytes.
- `TestCollectionRunnerSnapshotSurvivesNavigationAndPreview` changes saved
  inputs after the first request starts and exercises Page changes in Driver.
- `TestCollectionRunnerCancellation` checks Cancel, Escape, and window closure
  during local HTTP.
- `TestCollectionRunnerAllowsPreviouslyAuthorizedDownload` covers pending
  responses and already running file writes through the sheet.
- `TestCollectionRunnerVirtualizedAndRedactedAtMinimumViewport` navigates to
  ordinal 10,000 at 960 × 600 in English/light and Chinese/dark, with a canary
  in saved names, transport errors, response bodies, script logs, and assertions.
- `TestCompactReportParityAndRedaction` compares the full and compact v1 paths
  for continuation, stop-on-failure, and pre-cancellation.

Windows validation output excerpts

```text
$ gofmt -l .
[empty, exit 0]
$ go vet ./...
[empty, exit 0]
$ go tool viva vet ./...
[empty, exit 0]
$ go test ./... -count=1
ok  github.com/runbrick/reva/cmd/reva-run  8.267s
ok  github.com/runbrick/reva/internal/gui  19.344s
ok  github.com/runbrick/reva/internal/runner  2.769s
ok  github.com/runbrick/reva/internal/websocket  2.682s
[exit 0]
$ git diff --check
[empty, exit 0]
```
