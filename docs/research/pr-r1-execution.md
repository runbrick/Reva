# R1: reusable request execution

This local feature change extracts one HTTP send into `internal/execution` and
uses it from the existing GUI `ui.Effect`. It adds no dependencies, schema fields,
icons, persistence, or runner/CLI entry points.

## Public API for R2

```go
plan, err := execution.New(execution.Input{
    Request:    request,    // the draft; Request.ID identifies its collection item
    Collection: collection, // zero value is fine for explicit auth
    Variables:  values,     // ordered scope snapshot: collection, then environment
    Settings:   settings,   // transport defaults captured at execution start
})
if err != nil {
    return err // missing collection/item context for inherited auth
}
result := plan.Run(ctx, requests) // requests implements service.RequestService
if result.Failed() {
    // Inspect Err, Pre.Err, Tests.Err and Tests.Assertions separately.
}
// result.Response may hold any HTTP status, including 4xx/5xx.
// result.Variables contains send-local pre-script variable changes.
// A runner does not need to write history, collections, or environments.
```

`New(Input) (*Plan, error)` copies the request and variable slices, resolves
`auth.Resolve(collection, request.ID, request.Auth)`, captures unexpanded effective
auth, and resolves `Request.Options` against settings. Collection tree pointers
are used only during construction and are not retained. Requests with identical
names are distinguished by actual item ID. Existing folder None barriers,
nearest-parent inheritance, explicit own auth, and legacy empty auth semantics
come from the existing auth resolver.

`(*Plan).Request() model.Request` and `Variables() []model.KV` return independent
copies of the pre-script snapshot. `ValidateAuth() error` is an optional auth
preflight using the original variable snapshot; skip it when a pre-script can
change auth variables. The GUI uses it only for script-free requests to preserve
immediate localized invalid-auth feedback and the previous response on rejection.
`Run` always validates after the pre-script, regardless of preflight.

`(*Plan).Run(context.Context, service.RequestService) Result` can be called more
than once or concurrently; each call starts from independent copies. The injected
service must support concurrent use when callers run concurrently. Callers must
not mutate input concurrently with `New`, just as with normal snapshot creation.

`Result` exposes:

| Field | Meaning |
| --- | --- |
| `Stage` | Last stage entered: `pre`, `validation`, `transport`, `tests`, or `complete` |
| `Response *model.Response` | Response after successful transport, including HTTP error statuses |
| `Err error` | Operational validation, cancellation/deadline, or transport error |
| `Pre`, `Tests` | `ScriptResult{Logs []string, Assertions []script.Result, Err error}`; pre-script assertions remain empty under the existing sandbox API |
| `TransportStarted bool` | The request service was invoked; this does not prove that an endpoint received bytes |
| `Prepared model.Request` | Raw post-pre-script request, with resolved auth/options but unexpanded user fields |
| `Variables []model.KV` | Post-pre-script variables, including send-local changes |
| `HistoryRequest model.Request` | Original unexpanded send-start draft with resolved unexpanded auth |

`Result.Failed()` returns true for operational errors, either script error, or
any failed assertion. An HTTP status alone does not fail execution. A failed
assertion can leave `Stage == complete`; an uncaught response-script exception
leaves `Stage == tests`. Missing inheritance context returns a construction error
instead of a `Result`.

Runtime response data, prepared requests, variable values, script logs, and error
messages can contain secrets. These structs are an internal execution API, not a
redacted JSON report schema. R3 must explicitly select and redact report fields.

## Deterministic stages and failures

1. Construction captures scopes, resolves inherited auth, and resolves options.
2. `Run` checks cancellation, then runs the existing bounded pre-script sandbox
   on a private raw request and private variables.
3. A pre-script syntax error, exception, or timeout aborts before transport.
   Pre logs and partial raw runtime mutations remain in the result for diagnosis.
4. After another cancellation check, `vars.Apply` expands exactly once and
   `auth.Validate` checks the expanded authentication.
5. After another cancellation check, the service receives that expanded request
   with **nil variables**. This preserves existing lookup precedence, disabled-row
   fallback, unknown-placeholder behavior, and single-pass substitution when a
   variable value itself contains `{{another}}`.
6. Successful transport retains the response and runs response tests with the
   same once-expanded request and the raw post-pre variables. Assertions, logs,
   and an uncaught script error are retained independently. Transport errors skip
   response tests. Tests use the send-start script even if the editor changes.
7. Cancellation is checked after transport and scripts. A canceled transport
   result is discarded before response tests. Cancellation during a running
   sandbox is observed after its current bounded execution returns; scripts keep
   the existing two-second `script.DefaultTimeout` and are not forcefully killed.

The service still owns HTTP URL/body/header validation, file opening, actual
auth application, redirects, TLS, timeouts, and response body limits. Therefore a
service-side request construction error has `TransportStarted == true` even when
no network bytes were sent. The engine distinguishes these operational errors
from HTTP response statuses; it does not reclassify status codes as transport
failures.

## GUI and history compatibility

The former GUI sent after pre-script failure and then appended a pre-script
failure to response tests. **This intentionally changes:** pre-script failure now
stops sending and skips response tests. Its pre logs and failure appear directly
in the response area without inventing a response/status or a successful history
record. Starting the next send clears previous logs, errors, assertions and
response state. Existing localized script/auth strings are reused in both
languages.

All JavaScript runs off the UI thread within the existing keyed effect. Each tab
retains its own effect and script state. Canceling or closing a tab stops declaring
its effect; Viva cancels its context and drops late results. A changed job key
cancels the old effect, and the GUI also checks returned job identity. A slow old
send cannot replace a retry's response, tests, logs, or history. The existing
`sendJob.req` and `sendJob.vars` fields and job pointer identity remain available
to the request/download feature. Download uses the completed real response only.

The engine performs no persistence. The GUI adds history only after the request
service was invoked. Pre-script/auth preparation failures create no history.
Transport failures create error history; HTTP responses, including failed
response assertions or script exceptions, retain the actual HTTP status/duration.
GUI cancellation continues to suppress history through effect cancellation.

History now consistently uses the original **send-start** draft, including its
original options, scripts, URL, headers, body, and placeholders; it replaces only
auth with the resolved **unexpanded** auth snapshot. Runtime pre-script edits,
environment-expanded values, and later editor edits never replace the draft.
An inherited history request remains replayable after deleting its former
collection. Raw drafts can already contain configured literal credentials or
literal values inside script source; this change does not introduce a vault or
redact those existing draft fields.

Other compatibility boundaries remain unchanged: the sandbox implements its
existing ES5 subset, test-script variable mutations are not returned or persisted,
and the request service may apply generated auth/URL normalization beyond what
`pm.request` displays. R2 can use `Result.Variables` for pre-script changes within
an explicitly chosen run scope, but must choose that scope itself.

## Behavior coverage

New execution tests use only local `httptest` servers and cover scripted method,
URL, body and headers; post-script auth variables; inherited folder auth despite
duplicate names; real TLS/redirect/timeout policies; single-pass expansion in
transport and response-script context; raw history and immutable snapshots;
pre-script errors/timeouts preventing endpoint access; HTTP 503 and separate
assertion/script errors; None barriers and missing membership; cancellation before
pre, during bounded pre, during transport, and before response tests.

New headless harness tests exercise actual HTTP pre-abort/retry, visible logs and
diagnostic reset, scripted HTTP plus original send-start history during editor
changes, canceled stale job rejection, and responsive send/cancel with a runaway
pre-script. Existing script tests were updated only for intentional pre-abort
semantics; existing GUI/auth/options/download/tab/history behavior remains covered
by the full test suite.

## Delivery evidence

Branch: `runbrick/research-send-pipeline`. Commit is supplied in the coordinator's
completion message. No files outside the R1 ownership contract changed; there
are no requested cross-owner changes, blockers, new icons, or new dependencies.
Integration into the parent branch remains the coordinator's task.

Files changed:

- `internal/execution/execution.go`
- `internal/execution/execution_test.go`
- `internal/gui/app.go`
- `internal/gui/scripts.go`
- `internal/gui/scripts_test.go`
- `internal/gui/execution_test.go`
- `docs/research/pr-r1-execution.md`

Added tests:

- `TestScriptedRequestAuthOptionsAndRawHistory`
- `TestPreScriptErrorsAbortBeforeEndpoint`
- `TestResponseAssertionsAndScriptErrorsAreSeparateFromHTTPStatus`
- `TestSingleExpansionAlsoMatchesResponseScriptContext`
- `TestAuthValidationUsesPostScriptVariables`
- `TestInheritedAuthRequiresMembershipAndHonorsNoneBarrier`
- `TestCancellationAndTimeoutPreventLaterStages`
- `TestExecutionGUIAbortsPreErrorBeforeHTTPAndResetsDiagnostics`
- `TestExecutionGUIHistoryKeepsOriginalSendStartDraft`
- `TestExecutionGUICancelledOldJobCannotReplaceRetry`
- `TestExecutionGUIScriptsDoNotBlockSendOrCancel`

Actual check output from this Windows worktree (all exit codes 0):

```text
$ gofmt -l .
[empty output]
$ go vet ./...
[empty output]
$ go tool viva vet ./...
[empty output]
$ go test ./... -count=1
?    github.com/runbrick/reva/cmd/reva [no test files]
ok   github.com/runbrick/reva/internal/auth 2.612s
?    github.com/runbrick/reva/internal/cloudsync [no test files]
ok   github.com/runbrick/reva/internal/execution 4.851s
ok   github.com/runbrick/reva/internal/format 1.761s
ok   github.com/runbrick/reva/internal/gist 4.020s
ok   github.com/runbrick/reva/internal/gui 18.006s
?    github.com/runbrick/reva/internal/gui/icons [no test files]
ok   github.com/runbrick/reva/internal/i18n 2.599s
ok   github.com/runbrick/reva/internal/importer 8.587s
ok   github.com/runbrick/reva/internal/model 2.537s
ok   github.com/runbrick/reva/internal/s3 3.372s
ok   github.com/runbrick/reva/internal/script 3.370s
?    github.com/runbrick/reva/internal/service [no test files]
?    github.com/runbrick/reva/internal/service/fake [no test files]
ok   github.com/runbrick/reva/internal/service/request 5.055s
ok   github.com/runbrick/reva/internal/service/store 5.165s
ok   github.com/runbrick/reva/internal/storage 2.499s
ok   github.com/runbrick/reva/internal/syncing 5.742s
ok   github.com/runbrick/reva/internal/update 4.436s
ok   github.com/runbrick/reva/internal/vars 1.096s
ok   github.com/runbrick/reva/internal/websocket 3.458s
```
