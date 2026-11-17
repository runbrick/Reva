# R3: standalone headless native collection CLI

Native collections can now run from automation with `go run ./cmd/reva-run` or
a separately built `reva-run` binary. The command loads an explicitly selected
collection, optionally loads one environment, reuses the R2 runner and R1 send
pipeline, and emits the runner's compact version-1 safe JSON report with stable
exit codes. No GUI packages, data mutations, dependencies, or persistent schema
changes are introduced.

## Invocation and behavior

```sh
go run ./cmd/reva-run --data-dir ./reva-data --collection 0123456789abcdef
go build -o bin/reva-run ./cmd/reva-run
bin/reva-run --collection 0123456789abcdef --environment fedcba9876543210 --stop-on-failure
```

`--collection ID` is mandatory. Directory selection is `--data-dir`, then
`REVA_HOME`, then `os.UserConfigDir()/Reva`, following storage's location rules.
An explicitly empty data-dir or environment flag is an argument error. Selected
IDs are filename components, checked for traversal, platform path/stream syntax,
control characters, and Windows reserved device names. IDs and paths never appear
in emitted diagnostics or reports. Help (`-h` / `--help`) succeeds without data.

Only the selected collection, optional explicit environment, and settings file
are read. The GUI's `ActiveEnvID` is deliberately ignored. Unselected malformed
objects and history do not affect the run. Missing settings use safe model
defaults; present malformed or unreadable settings always abort. The loader
decodes into a fresh value so an omitted version cannot inherit version 1 from
initialized defaults.

Runner semantics remain unchanged: native depth-first order, immutable baseline
collection/environment variables, local per-request script mutations, inherited
auth/None barriers, saved settings/request transport options, pre-script abort,
single expansion, HTTP and response tests. Default continuation and optional
stop-on-failure come directly from `runner.Options`. HTTP status alone is not a
failure. Unsupported post-script methods/protocols remain runner failures.
No command-line flags weaken auth or TLS; existing explicit saved settings and
request overrides retain their shared-engine meaning.

`main` uses `signal.NotifyContext` for interrupt and SIGTERM where supported,
then calls the testable `run(context.Context, args, stdout, stderr) int`.
Cancellation produces the existing safe partial report and skips subsequent
requests. Existing script cancellation latency remains bounded by the sandbox's
two-second timeout. Per-request timeouts remain ordinary run failures.

## Read-only input and strict native validation

Neither `storage.Dir()` nor `store.Open()` is called: both create directories.
The small command-local directory resolver reproduces only storage's location
selection, with `storage.EnvHome` as its environment variable contract. There is
no store initialization, directory creation, data-file writer, history service,
or environment mutation in the command.

`storage.ReadJSON` reads the entire file without a bound. The command instead
uses `os.Stat`, regular-file checks before/after open, and `io.LimitReader`
to bound the actual read to 32 MiB even if a file grows. It validates UTF-8 and
uses a typed JSON token walk before decoding. This enforces exact known field
names, required execution fields, no duplicate keys (including escaped keys),
no case aliases, no scalar/struct nulls, one root object and no trailing data.
It accepts native null slices and omitted optional fields. Version 1 is required
for each present settings/collection/environment object; selected ID and stored
ID must match.

Limits are 1,024 JSON nesting levels, 256 item levels, 10,000 tree nodes, and
100,000 rows per collection or environment. Tree validation rejects duplicate or
invalid item IDs, missing request payloads, request children, folder payloads,
unknown item types, blank names, and invalid transport toggle/timeout values.
Timeout bounds prevent overflow when shared options resolve seconds to
milliseconds and transport duration. Normal request/auth/body validation and
script/transport behavior stay with the runner and HTTP service.

Enabled upload references can read and send existing local files through the
shared HTTP service, as in the GUI. Read-only refers to native data persistence;
the command executes trusted collections and is not an import preview or a
filesystem/network sandbox. No new read-only storage API is required; a future
shared noncreating location resolver could remove the small location-rule
duplication, but it is not needed for this delivery.

## Output and failure contract

After execution, stdout contains one compact JSON report and a newline,
including failed scripts and partial cancellation. It is exactly
`json.NewEncoder(stdout).Encode(result.Report())`, with `runner.ReportVersion == 1`.
No caller-supplied strings are added to that projection. Reports contain only
fixed outcome/code/stage values, generated one-based ordinals, send flags,
optional HTTP status codes, nanosecond timings and request/assertion counts.
They omit IDs/names, raw URLs/requests/responses, headers/cookies/bodies, variable
keys/values, scripts/logs, assertion names/errors and arbitrary error messages.

Input/load errors emit no stdout; help emits fixed usage text instead of JSON.
Normal passed/failed/cancelled runs have empty stderr. Error paths emit only
`reva-run: <fixed-code>` on stderr. The flag parser's output is discarded, and
its returned error, file/JSON errors, raw engine errors, and writer errors are
never formatted. Fixed CLI diagnostics are:

- `invalid_arguments`
- `collection_required_or_invalid`
- `data_directory_unavailable`
- `settings_invalid_or_unreadable`
- `collection_invalid_or_unreadable`
- `environment_invalid_or_unreadable`
- `output_failed`

| Exit | Meaning |
| --- | --- |
| 0 | All passed, including an empty collection, or help |
| 1 | Run failure/assertion failure, or report/help output failure |
| 2 | Usage or input load/validation failure |
| 130 | Parent context cancelled, including deadline cancellation in embedded tests |

Report write failure takes exit 1; the output may be partial. For reliable
process exit codes use the built binary: `go run` itself wraps nonzero program
exit statuses and may emit its own diagnostic. Counts/order/status/timing remain
intentional report disclosures. There is no raw/debug report flag, parallel
execution, iteration, implicit environment activation, cross-request response
capture, cookie jar, or expanded Postman runtime.

## Behavior tests and evidence

All HTTP tests use loopback `httptest` servers. Native files are written to
`t.TempDir()` with the real storage writer, then CLI runs read them directly.
Byte/name/directory snapshots prove nonmutation on success, request failure,
invalid input, and cancellation. Tests exercise inherited folder auth, variable
precedence and disabled fallback, pre logs, response data, arbitrary test names
and throws, auth failures, malformed URLs and sensitive upload paths; neither
stdout nor stderr may contain the credential canary. Pre failure, invalid inputs,
and cancelled later requests never reach an endpoint.

Added tests:

- `TestCLIInheritedAuthVariablesAndNonmutation`
- `TestCLIDefaultEnvironmentAndDataDirectorySelection`
- `TestCLIFailureContinuationStopAndHTTPStatus`
- `TestCLISafeReportsForScriptAndTransportFailures`
- `TestCLIUsageAndHelpNeverLoadOrCreateData`
- `TestCLIInvalidNativeInputNeverSendsOrMutates`
- `TestCLICancellationProducesPartialSafeReport`
- `TestCLIEmptyCollectionAndReportWriterFailure`
- `TestCLIBinaryExitCodesAndGUIFreeDependencies`
- `TestCLIStrictJSONRejectsAmbiguousMalformedAndFutureData`
- `TestCLIBoundedNativeInput`
- `TestCLIStrictNestedSchemaAndNonregularFiles`

The binary test invokes `go list -deps .` and rejects **all** dependencies under
`github.com/vivaui/viva` or `github.com/runbrick/reva/internal/gui`, including
Viva app/gfx. It then builds the standalone executable and checks actual process
help/usage/failure exits and safe output. Context cancellation is tested during
real HTTP after one completed request, and again before the first request;
this Windows run does not inject an OS Ctrl+C event into the subprocess.

Branch: `runbrick/research-runner-cli`. Commit hash is supplied in the worker
completion report. Files changed only within R3 ownership:

- `cmd/reva-run/main.go`
- `cmd/reva-run/load.go`
- `cmd/reva-run/main_test.go`
- `cmd/reva-run/load_test.go`
- `README.md` (CLI section only)
- `docs/research/pr-r3-cli.md`

No requested cross-owner API changes, new dependencies, icons, persistent schema
changes or unresolved blockers. Parent integration remains the coordinator's
task; no push, remote PR, main merge or parent merge was performed.

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
ok   github.com/runbrick/reva/cmd/reva-run 7.159s
ok   github.com/runbrick/reva/internal/auth 1.314s
?    github.com/runbrick/reva/internal/cloudsync [no test files]
ok   github.com/runbrick/reva/internal/execution 4.614s
ok   github.com/runbrick/reva/internal/format 0.902s
ok   github.com/runbrick/reva/internal/gist 2.169s
ok   github.com/runbrick/reva/internal/gui 12.136s
?    github.com/runbrick/reva/internal/gui/icons [no test files]
ok   github.com/runbrick/reva/internal/i18n 1.522s
ok   github.com/runbrick/reva/internal/importer 6.664s
ok   github.com/runbrick/reva/internal/model 1.737s
ok   github.com/runbrick/reva/internal/runner 2.952s
ok   github.com/runbrick/reva/internal/s3 2.668s
ok   github.com/runbrick/reva/internal/script 2.625s
?    github.com/runbrick/reva/internal/service [no test files]
?    github.com/runbrick/reva/internal/service/fake [no test files]
ok   github.com/runbrick/reva/internal/service/request 4.400s
ok   github.com/runbrick/reva/internal/service/store 4.223s
ok   github.com/runbrick/reva/internal/storage 1.914s
ok   github.com/runbrick/reva/internal/syncing 4.975s
ok   github.com/runbrick/reva/internal/update 3.387s
ok   github.com/runbrick/reva/internal/vars 1.164s
ok   github.com/runbrick/reva/internal/websocket 2.729s
```
