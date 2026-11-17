# R2: deterministic serial native collection runner

`internal/runner` executes a native collection synchronously with an injected
`service.RequestService`. It imports no GUI, importer, storage or store packages;
only the integration test imports the real store. This change adds no dependencies,
model/schema fields, icons, persistence, or CLI entry point.

## API for R3 and future GUI consumers

```go
result := runner.Run(
    ctx,
    requests,              // non-nil service.RequestService
    collection,            // model.Collection
    settings,              // model.Settings snapshot
    environment.Variables, // []model.KV; nil means no environment
    runner.Options{StopOnFailure: false},
)

switch result.Outcome {
case runner.Cancelled:
    // result.Err is ctx.Err(): context.Canceled or context.DeadlineExceeded.
case runner.Failed:
    // At least one request failed; inspect fixed codes or report counts.
case runner.Passed:
    // Includes an empty collection; HTTP status alone does not fail a request.
}

// Safe CLI output. Neither form includes detailed diagnostics.
err := json.NewEncoder(output).Encode(result)
report := result.Report() // runner.Report, version 1
```

`Run` returns one `Result`, with `Outcome`, `Err`, total wall-clock `Duration`, and
an ordered `Requests []RequestResult`. `Result.Err` is reserved for the **parent
context error**; individual request failures remain in their request result.
Failures are normal result data rather than a second returned Go error. The CLI
must inspect `Outcome` or report counts rather than only testing `Err`.

Each request result has `Ordinal` (one-based position), `ItemID`, `Outcome`,
`Code`, `Stage`, wall-clock `Duration`, `Sent`, `Err`, and optional
`Execution *execution.Result`. `Err` holds operational or plan-construction
errors; pre/test errors live separately in `Execution.Pre.Err` and
`Execution.Tests.Err`. `Execution.Tests.Assertions` retains each actual
assertion, including its detailed name/error, for in-process GUI use.
Construction failures and skipped requests have no `Execution`.

## Ordering, snapshots and scope

Before the first send, the runner deeply clones the collection tree, request
slices and collection variables, copies environment rows, and constructs every
execution plan with the settings value. Neither later caller edits nor scripts
can change a subsequent request's snapshot. As with any snapshot API, callers
must not concurrently mutate inputs while the initial snapshot is being copied.
The supplied sender owns its own mutable transport state; caller-provided Cookie
jars or other stateful custom senders are outside this isolation boundary.

Traversal preserves native item order, depth first, without sorting names or
IDs. Empty folders produce no outcomes. Request items with nil payloads and
non-request nodes are not requests; children are still visited in original
order, matching `model.WalkRequests`. The runner expects a valid native tree;
schema validation/import, duplicate item-ID repair, cyclic programmatic trees,
and input bounds belong to the caller. Missing inherited-auth context is still
reported as a request failure. A request's **item ID** replaces its embedded
`Request.ID` for execution, so a stale embedded ID does not resolve auth against
the wrong folder. Names are never used for matching.

Collection variables precede the explicit environment snapshot. The last
enabled matching row wins; a disabled environment row falls back to an enabled
collection row. Each request starts from that same baseline. Pre-script variable
mutations apply to that request's expansion and response tests only. Both
`pm.variables` and the sandbox's existing `pm.environment` alias are local to
that request. Test-script mutations are discarded by the existing sandbox.
There is no implicit response capture, cross-request variable propagation,
active-environment lookup, or collection/environment/settings/history write.

Auth inheritance/None barriers, per-request options over settings, bounded pre
scripts, single expansion, post-pre auth validation, transport, and response
tests all use `execution.New` / `Plan.Run`. The runner does not reimplement these
stages. The transport wrapper receives the once-expanded post-pre request and
passes it to the underlying sender with nil variables, preserving R1's
single-pass expansion semantics.

## Failure and cancellation policy

Zero-value `Options` continues after request failures. `StopOnFailure: true`
skips every later request after the first failed request. Every request has a
stable result position even when it was not attempted. Fixed codes are:

| Code | Outcome | Meaning |
| --- | --- | --- |
| `ok` | `passed` | Completed with no operational/script/assertion failure |
| `assertion_failed` | `failed` | At least one response assertion failed |
| `pre_script_failed` | `failed` | Pre-script syntax error, exception or timeout; no send |
| `test_script_failed` | `failed` | Response-script syntax error, exception or timeout |
| `auth_failed` | `failed` | Inheritance construction or post-pre auth validation failed |
| `transport_failed` | `failed` | Request construction, network, body read, or per-request timeout failed |
| `unsupported_method` | `failed` | Post-pre method outside the supported native method set |
| `unsupported_protocol` | `failed` | Post-pre URL explicitly selects a non-HTTP(S) `://` protocol |
| `cancelled` | `cancelled` | Parent context interrupted an attempted request |
| `skipped_failure` | `skipped` | Unattempted because stop-on-failure was selected |
| `skipped_cancellation` | `skipped` | Unattempted because the parent context ended |

Supported methods are the native picker set: uppercase `GET`, `POST`, `PUT`,
`PATCH`, `DELETE`, `HEAD`, `OPTIONS`. Custom methods, `TRACE`, `CONNECT`,
WebSocket methods and lowercase method strings are explicitly unsupported in
this first runner. The guard runs after pre scripting and expansion, so scripts
can convert an initial request into a supported request, but cannot bypass the
capability check. Schemeless URLs retain the request service's implicit HTTP
behavior. Malformed URLs (including malformed scheme syntax without `://`)
remain transport/request-construction failures. Request body modes and upload
file access use the existing HTTP sender's rules.

HTTP 4xx/5xx are ordinary successful responses unless a script explicitly asserts
otherwise. A response-script exception takes diagnostic priority over failed
assertions in the fixed code, while assertion counts/details retain both. Auth
and pre-script failures never call the underlying sender. `Sent` means that
sender was invoked, not that network bytes reached a server. Unsupported
requests have `Sent == false`, although their detailed execution engine
`TransportStarted` is true because the engine entered its injected wrapper.

Parent cancellation/deadline has priority over script, auth or transport errors
and over the stop strategy. It always makes the run `cancelled` and sets
`Result.Err = ctx.Err()`. It prevents subsequent sender invocations. As in R1,
an already running script finishes or reaches its existing two-second sandbox
limit before cancellation is observed. Cancellation during transport discards
the transport response and prevents response tests. A per-request timeout or a
custom sender's context error with a still-live parent is a transport failure,
so default continuation still applies.

An empty collection completes `passed`, with zero counts and an empty `requests`
array. An already-cancelled context still makes an empty run `cancelled`.
The runner never turns cancellation into success because nothing was sent.

## Exact report safety contract for CLI output

`runner.ReportVersion == 1` versions this report independently from native
storage schemas. `json.Marshal(result)`, a pointer to the result, each individual
`RequestResult`, and slices of request results use custom JSON projection.
Detailed fields also carry `json:"-"` tags. `result.Report()` returns a fresh
DTO containing **only** the following fields:

| Object | Fields |
| --- | --- |
| Report | `version`, `outcome`, `durationNs`, `counts`, `assertions`, `requests` |
| Counts | `total`, `passed`, `failed`, `cancelled`, `skipped` |
| Assertions | `passed`, `failed` |
| RequestReport | `ordinal`, `outcome`, `code`, `stage`, `sent`, optional `statusCode`, `durationNs`, `assertions` |

All exported string values are fixed allowlisted outcome/code/stage values.
Stages are `pending`, `prepare`, `pre`, `validation`, `transport`, `tests`,
`complete`. `prepare` identifies inheritance-construction failure; `pending`
identifies skipped requests. Unknown mutated enum values in a detailed result
are replaced with fixed values by projection. Report request ordinals are
regenerated from slice order. Durations are nonnegative integer nanoseconds;
HTTP status is included only in the range 100–599. Counts include all requests;
an interrupted attempt counts as `cancelled`, while requests never attempted
count as `skipped`. Assertion counts include results obtained before a later
script exception. No timestamps are exported.

There are **no stored IDs**, even hashed IDs, and **no request/folder/collection
or environment names** in reports. These are all caller-controlled and may
contain credentials. Ordinals are generated numeric positions local to this
run and have no persistent identity. There are no raw requests/responses, URLs,
methods, paths, variable keys/values, auth fields, headers/cookies, bodies,
scripts, script logs, arbitrary error strings, assertion names, or assertion
error messages. No expanded token is emitted. Reports intentionally disclose
request count/order, stage/outcome, assertion counts, whether the sender was
invoked, HTTP status and elapsed timing; they are not designed to conceal those
execution facts or deliberate covert encoding through numeric metadata.

The trust boundary is the projection from detailed engine data to this
allowlist. In-memory `Execution`, item IDs and `Err` are sensitive. A CLI must
never format them with `%v`/`%+v`, serialize `Execution` separately, append a raw
error/collection identifier to a report, or print script logs. Projection protects
the normal JSON path, not arbitrary diagnostic formatting or subsequent caller
mutation of the exported DTO (including deserializing an untrusted DTO and
re-emitting it). CLI input/load errors need their own fixed-code output policy.
This is field omission, not unreliable secret-string search and replacement.

## Behavior coverage and delivery

New tests use local `httptest` endpoints and an isolated real store:

- `TestNestedDepthFirstOrderAuthAndEnvironmentPrecedence`
- `TestAllInputsSnapshotBeforeFirstSend`
- `TestAssertionFailureContinueAndStopStrategy`
- `TestPreScriptFailureNeverReachesEndpoint`
- `TestFailureCategoriesAndPostScriptCapabilityChecks`
- `TestCancellationStopsRemainingRequestsAndIsPrimary`
- `TestRequestLocalScriptVariablesAndUnchangedInputs`
- `TestDeadlineAndTransportFailureRemainDistinct`
- `TestCancellationDuringPreScriptOutranksScriptFailure`
- `TestEmptyCollectionSemantics`
- `TestSafeVersionedReportOmitsAllUntrustedStrings`
- `TestRealStoreBytesUnchangedAcrossRun`

The real-store test compares every data-file byte and file name before/after
the run, covering history, collections, environment and settings. The report
test places a secret canary in supplied IDs, all entity names, URL, credentials,
variables, headers/cookies, response body, scripts/logs, assertion name/error,
and transport error, then verifies both the explicit DTO and default JSON paths.

Branch: `runbrick/research-collection-runner`. Commit hash is supplied in the
worker completion report. Files: `internal/runner/runner.go`,
`internal/runner/report.go`, `internal/runner/runner_test.go`, and this document.
No changes to another owner's files or API are requested. No unresolved
blockers; parent integration remains the coordinator's task.

Actual required check output from this Windows worktree (all exit codes 0):

```text
$ gofmt -l .
[empty output]
$ go vet ./...
[empty output]
$ go tool viva vet ./...
[empty output]
$ go test ./... -count=1
?    github.com/runbrick/reva/cmd/reva [no test files]
ok   github.com/runbrick/reva/internal/auth 1.426s
?    github.com/runbrick/reva/internal/cloudsync [no test files]
ok   github.com/runbrick/reva/internal/execution 4.574s
ok   github.com/runbrick/reva/internal/format 0.955s
ok   github.com/runbrick/reva/internal/gist 2.604s
ok   github.com/runbrick/reva/internal/gui 15.128s
?    github.com/runbrick/reva/internal/gui/icons [no test files]
ok   github.com/runbrick/reva/internal/i18n 1.759s
ok   github.com/runbrick/reva/internal/importer 7.982s
ok   github.com/runbrick/reva/internal/model 2.144s
ok   github.com/runbrick/reva/internal/runner 2.709s
ok   github.com/runbrick/reva/internal/s3 3.582s
ok   github.com/runbrick/reva/internal/script 2.752s
?    github.com/runbrick/reva/internal/service [no test files]
?    github.com/runbrick/reva/internal/service/fake [no test files]
ok   github.com/runbrick/reva/internal/service/request 4.676s
ok   github.com/runbrick/reva/internal/service/store 4.662s
ok   github.com/runbrick/reva/internal/storage 1.416s
ok   github.com/runbrick/reva/internal/syncing 5.192s
ok   github.com/runbrick/reva/internal/update 3.516s
ok   github.com/runbrick/reva/internal/vars 0.976s
ok   github.com/runbrick/reva/internal/websocket 3.529s
```
