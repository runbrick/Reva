# QA: research R1–R4 cross-module regressions

Branch: `runbrick/research-runner-regressions`. This is a local QA PR description
and evidence record; no push, remote PR, or main merge was performed. The QA
implementation commit is supplied in the worker completion report. Production
fixes remain separate owner commits, merged here by exact hash as authorized by
the coordinator so integration can preserve their ancestry.

## Problem and resulting coverage

The feature suites exercised each execution, runner, CLI, and preview boundary
individually. These regressions use shared native fixtures and observable HTTP,
disk, process, and headless GUI behavior to check the combined delivery. All
network traffic is local `httptest`; all persistence lives in temporary
directories selected with `REVA_HOME`. No dependencies, icons, schema changes,
production edits, shared harness changes, or existing test edits belong to the
QA implementation commit.

QA owns exactly these new files:

- `internal/runner/regression_test.go`
- `internal/gui/research_regression_test.go`
- `cmd/reva-run/regression_test.go`
- `docs/research/pr-qa-regressions.md`

Added tests:

- `TestResearchGUIAndRunnerAgreeOnStoredSendSemantics`: writes with the actual
  store, reopens the native files, then sends the same stored request through
  runner and the shared GUI harness. Endpoint observations must agree on POST,
  path, body, query multiplicity, inherited Bearer/Basic/query API-key auth,
  post-pre-script auth values, explicit empty Authorization override, duplicate
  environment winners, disabled-environment fallback, single expansion, and a
  request redirect override over saved settings. It includes divergent
  URL/Params, URL-only pairs, duplicate pairs, disabled rows, equality only after
  expansion, and a variable-generated key containing `&`. Opening must preserve
  the authored draft. Pre failure reaches no endpoint and writes no history;
  successful GUI sends add exactly one history entry containing the unexpanded
  send-start draft and resolved, unexpanded auth. Runner writes nothing.
- `TestResearchPreviewUsesPersistedEnvironmentWithoutExecuting`: starts with a
  real completed GUI HTTP response and script logs, then changes the draft to a
  throwing pre script and opens preview. Scrolled painted labels and accessible
  hit names must omit short secret canaries in the request body, variable values,
  environment/collection/request names, prior response, and prior script logs.
  The selected persisted environment, auth source, variable row provenance,
  shadowing, masking, and pre-script phase remain visible. Preview must preserve
  file bytes/names/directories, draft, prior response, and diagnostics without
  sending or executing the new script. Closing restores names and response;
  only an explicit Send executes the new script, which aborts before HTTP and
  history writes.
- `TestResearchStoredNestedRunFailureThenCancellationIsReadOnly`: a real reopened
  store supplies a deliberately unsorted nested tree with an empty folder,
  inherited auth, a successful request, failed pre script, interrupted request,
  and later skipped request. Stable positions and endpoint observations prove
  depth-first order, request-local script variables, cancellation priority over
  an earlier failure, no cancelled response tests, no later sends, and unchanged
  persisted settings/collection/environment/history. A separate bounded rerun
  checks nested stop-on-failure and the same persisted baseline.
- `TestResearchCLIBinaryNestedFailuresKeepSafePartialReport`: builds and executes
  the actual binary against native files and a loopback server. Both continuation
  and stop modes must exit 1 and emit exactly one compact versioned report with
  empty stderr. The nested run combines a successful HTTP 503 assertion, failed
  pre script, failed assertion followed by a response-script exception, malformed
  URL transport error, and later success. Secret canaries in arbitrary entity
  names, URL, body, auth, response headers/body, logs, assertion names, and error
  text must be absent. Reports retain safe positions, counts, status and assertion
  totals; endpoints prove ordering and absence of the pre-failed send. Every
  native file and directory remains unchanged.
- `TestResearchCLICancellationAfterFailurePreservesSafeNestedPositions`: calls
  the production CLI `run` entry with a cancellable parent context, a successful
  request, earlier pre failure, nested interrupted request, and skipped later
  request. It verifies exit 130, safe compact partial JSON, all four positional
  outcomes/send flags, exactly two endpoint visits, and no persistence changes.

The CLI tests reuse only the established native-fixture writer, complete disk
snapshot, invocation, and strict single-report decoder helpers. They do not call
the CLI loader or report projection to construct expected results. New GUI tests
reuse the shared `newHarness`, click/wait helpers and existing frame-resize helper;
the real Viva v0.6.0 `app/driver.go` Snapshot, Pump, Click and Scroll APIs were
inspected. The absent `TASK.md` and local Viva skill are the pre-existing baseline
limitation already recorded in the implementation plan.

## Defects found by independent QA

1. **Initial GUI editor load changed valid native requests.** A URL containing
   `tag=a&tag=a&url-only=kept&api-key=old&api-key=older` and Params containing
   `tag=a,tag=a,tag=b` lost its URL-only pairs when opened in GUI, while runner
   retained them. A query-less URL with Params `{{query-key}}=a` and
   `query-key=x&y` sent the extra pairs `x=` and `y=a` after eager GUI URL
   serialization. The owner fix `eb1a21d8af46fce1087f5250a235df0c452ea23d`
   preserves authored URL and Params until explicit editing. All ten independent
   GUI/runner subcases pass after that exact dependency merge.
2. **Preview redaction did not cover background shell content.** The modal
   redacted a known variable value inside an environment name, but the titlebar
   still painted its raw name. With an innocuous environment name, the collections
   sidebar independently exposed the same canary in collection/request metadata.
   A short canary (`QAPRIV17`) is necessary to avoid mistaking text truncation for
   redaction. The retained regression also includes real prior response/log
   canaries so hiding only the titlebar/sidebar cannot silently leave a response
   surface exposed. Owner fix `73e6d765c5ea57405f814f6c3dd931ebcacb3fef`
   sanitizes environment display names and suppresses sensitive background
   surfaces while retaining existing Effect declarations. The independent
   privacy regression passes after merging that exact dependency commit.

Before the owner fixes, the full suite failed only in these new GUI regressions;
all other packages passed. QA reported concrete reproductions to the coordinator
and made no production workaround.

## Risk review and exact validation limits

GUI evidence is **headless snapshot validation** of actually painted labels,
accessible hit names, input actions, and async completion with the established
Driver harness. No native window was opened. Native-window rendering, screenshots,
pixel correctness, font/platform appearance, and manual visual acceptance were
**not performed**. Existing owner layout tests cover Chinese/English and light/dark
headless bounds; this QA addition uses saved English settings and the default light
theme to focus on cross-module behavior.

The actual binary test covers failure exit 1; cancellation exit 130 is tested at
the production injectable CLI entry point, not by injecting an OS Ctrl+C event
into a Windows subprocess. Existing CLI tests cover help, input exits, successful
runs, input validation, and the GUI-free dependency closure. This addition checks
outcomes and schema decoding, not exact duration values.

Safe reports intentionally disclose request positions/counts, outcomes, stages,
sender invocation, HTTP status and timing. Canary checks add independent evidence
for field omission; they do not claim that arbitrary formatting of sensitive
in-memory execution results is safe. Native collections and their scripts remain
trusted input; upload-file access and the existing bounded ES5 sandbox are the
documented feature boundaries. No expanded runtime, secret vault, cookie jar,
parallelism, remote execution, or later research feature is assessed or delivered
by this QA PR.

## Final integrated validation

Both reproduced defects are resolved by their owner commits; no unresolved
defects or requested production changes remain. Exact dependency merges in this
QA branch are `33e29fd` (parity) and `2223d48` (privacy and its integrated baseline).
The latter also brings the R3 bilingual-help followup `5c046c2`, so the final checks
include all R1–R4 features and the known localization followup.

All five new tests, all ten GUI/runner parity subcases, and both actual-binary
failure-strategy subcases passed in the focused verification. The four required
checks then passed on the complete integrated Go tree; only this evidence document
was updated afterward. Actual output (every command exit 0):

```text
$ gofmt -l .
[empty output]
$ go vet ./...
[empty output]
$ go tool viva vet ./...
[empty output]
$ go test ./... -count=1
?    github.com/runbrick/reva/cmd/reva [no test files]
ok   github.com/runbrick/reva/cmd/reva-run 7.836s
ok   github.com/runbrick/reva/internal/auth 1.231s
?    github.com/runbrick/reva/internal/cloudsync [no test files]
ok   github.com/runbrick/reva/internal/execution 4.538s
ok   github.com/runbrick/reva/internal/format 0.822s
ok   github.com/runbrick/reva/internal/gist 3.154s
ok   github.com/runbrick/reva/internal/gui 17.368s
?    github.com/runbrick/reva/internal/gui/icons [no test files]
ok   github.com/runbrick/reva/internal/i18n 1.536s
ok   github.com/runbrick/reva/internal/importer 6.258s
ok   github.com/runbrick/reva/internal/model 1.202s
ok   github.com/runbrick/reva/internal/preview 2.751s
ok   github.com/runbrick/reva/internal/runner 2.959s
ok   github.com/runbrick/reva/internal/s3 2.791s
ok   github.com/runbrick/reva/internal/script 2.538s
?    github.com/runbrick/reva/internal/service [no test files]
?    github.com/runbrick/reva/internal/service/fake [no test files]
ok   github.com/runbrick/reva/internal/service/request 3.808s
ok   github.com/runbrick/reva/internal/service/store 3.673s
ok   github.com/runbrick/reva/internal/storage 1.617s
ok   github.com/runbrick/reva/internal/syncing 4.419s
ok   github.com/runbrick/reva/internal/update 2.826s
ok   github.com/runbrick/reva/internal/vars 1.616s
ok   github.com/runbrick/reva/internal/websocket 2.642s
```

`git diff --check` and the staged diff check produced no output. The coordinator
owns final integration into the parent branch; QA has not merged there or into
main and has not pushed or opened a remote PR.
