# Import expansion: investigation and sequential implementation plan

Status: planning deliverable only, 2026-11-13. No production code is changed by
this phase. Run the remaining phases strictly in order: backend, then UI, then
tests. Start no overlapping worker or helper. Each phase commits locally and
reports its actual checks before the coordinator dispatches the next phase.

## Objective and bounded scope

Make migration into Reqo useful and predictable: preserve requests, folders,
variables and supported authentication; expose unsupported semantics before
saving anything; import into new entities without overwriting existing data.
The existing auth refactor is already present and should be preserved.

This cycle implements:

1. Reqo schema-v1 collection JSON reimport, including shared auth, scripts,
   options, variable descriptions and empty folders.
2. Improved Postman Collection v2.0/v2.1 import with diagnostics and explicit
   failure for authentication/body features that cannot be safely represented.
3. Apifox and ApiPost migration through their documented Postman export options.
   These are Postman-compatible files, not new proprietary-format parsers.
4. Postman environment JSON and standalone Reqo schema-v1 environment JSON.
5. HAR 1.2 request import into one new collection, with a deliberately restricted
   body/cookie mapping and visible omissions.
6. Existing curl import, with diagnostics for transport flags currently ignored.

No new dependencies, service interface, model field, app-shell change, transport
rewrite, remote fetching, archive/bulk importer or encrypted-backup restore is
needed. One input creates exactly one collection, one environment, or one
unsaved curl request. This avoids pretending the current service contracts
provide transactions spanning multiple files.

## Repository findings and missing references

- Read the repository `AGENTS.md` and `README.md`. `TASK.md` and
  `.claude/skills/building-with-viva/SKILL.md` are absent. Searches of tracked and
  hidden repository files and the user's `.claude`, `.agents` and `.codex`
  skill directories found no replacement Viva skill. Existing
  `docs/onboarding-prs.md` independently records the same missing references;
  it is historical guidance, not a substitute task specification.
- The task preamble names branch `runbrick/ribbonworm`; the actual clean checkout
  reports `runbrick/refactor-auth-features`, HEAD `79642cf` (the auth-refactor PR
  merge). Keep this observed feature branch; do not switch branches, touch main,
  push, or merge. The coordinator should use the observed branch in subsequent
  dispatches. This document's commit can be resolved with
  `git log -1 --format=%H -- docs/import-expansion-plan.md`.
- `internal/importer/{importer,postman,curl}.go` currently recognizes only
  collection-shaped Postman JSON (`info` plus `item`) and curl commands. Native
  collection exports cannot be reimported. Detection is only a hint and must
  never replace validation.
- `postman.go` preserves nested/empty folders, repeated query/header rows,
  disabled flags, collection variables, raw/JSON/form/urlencoded/file bodies,
  Bearer/Basic/API-key auth, and explicit request inheritance. It accepts legacy
  object auth and v2.1 array auth. Collection/folder auth is retained, not copied
  into every child. IDs are regenerated.
- Important gaps: unsupported auth becomes no-auth, unsupported body becomes
  no-body, unknown item nodes are skipped, events/scripts and descriptions are
  ignored, structured URL variables are ignored, and file metadata may be lost.
  Current tests explicitly assert some silent downgrades; those expectations
  must be deliberately replaced, not retained as desired behavior.
- `internal/auth` already implements inherited auth with nearest-parent lookup,
  explicit no-auth barriers, legacy empty-request-auth behavior, validation and
  explicit-header precedence. `app.go` resolves auth before variable expansion
  and sending; history retains unexpanded credentials. Request transport has
  cross-origin redirect protections and API-key query behavior. Preserve these
  contracts; OAuth2 lifecycle and additional auth schemes remain separate work.
- `internal/gui/collections.go:exportCollection` writes the raw
  `model.Collection` JSON, including auth. `CollectionService.Import` clones,
  regenerates collection/item/request IDs, and stores one file atomically;
  request IDs are kept equal to their owning item IDs. Store validation alone
  does not validate tree shape, auth enums or incoming schema versions, so
  import validation must happen before calling it.
- `EnvironmentService.Save` writes one environment atomically. It can save a
  fresh generated ID directly; do not call `Create` and then `Save`, which could
  leave an empty environment if the second write fails. Active environment,
  preferences and existing drafts should stay unchanged after an import.
- `syncing` has a separate encrypted `reqo-backup-v1` envelope and snapshot
  restore path that can replace existing data. Neither its decrypted snapshot
  nor its envelope is a collection import. Do not invoke `syncing.Apply` here.
- Import UI currently loads one `.json` file through a keyed effect, limits file
  reads to 32 MiB, parses synchronously after one Submit, and either imports a
  collection or opens an unsaved curl tab. It shows generic localized errors but
  no preview or fidelity report. Pasted text lacks the same input-size guard.
- Available tests include importer v2.0/v2.1/curl fixtures and fuzz seeds, GUI
  file-picker/cancel/error tests, store import-ID/copy-isolation tests, and
  extensive auth resolution, GUI, transport, persistence and sync regressions.
  The shared GUI harness uses `app.Driver`; do not modify it or open a window.

## Source-format evidence and support claims

Public sources were read with HTTP on 2026-11-13. Tests must use checked-in
sanitized fixtures and must never fetch these URLs.

| Source | Evidence and resulting decision |
| --- | --- |
| [Postman v2.1 schema](https://schema.getpostman.com/json/collection/v2.1.0/collection.json) | Actual schema retrieved, redirecting to `schema.postman.com`. It defines auth, folders, events, bodies, descriptions, variables and structured URLs; recognize v2.0/v2.1 schema identities rather than arbitrary substring matches. |
| [Postman export guide](https://learning.postman.com/docs/getting-started/importing-and-exporting/exporting-data.md) | Collections export as v2.1; environments and globals export separately as JSON; bulk dumps are distinct. Support a single environment export, not bulk dumps or implicit global-scope promotion. |
| [Apifox export guide](https://docs.apifox.com/export-data.md) | Explicitly lists Postman, Apifox, OpenAPI, HTML and Markdown export. UI guidance: Apifox Settings → Export data → Postman; inspect the resulting file as Postman, without claiming native Apifox project support. |
| [ApiPost export guide](https://wiki.apipost.cn/docs/project_set/import_project/export_project) | Official current wiki explicitly says standard Swagger 2.0/3.0 and Postman 2.1 exports are supported. UI guidance: project settings → export project → Postman 2.1. This is affirmative evidence for a compatibility route, not evidence of proprietary schema equivalence. |
| [ApiPost code generation](https://wiki.apipost.cn/docs/gencode) | Documents Copy as cURL and cURL code generation for HTTP/SSE/GraphQL. Existing curl parsing offers a per-request alternative, without adding SSE streaming behavior. |
| [Apifox importing ApiPost](https://docs.apifox.com/import-apipost.md) | Mentions ApiPost-native exports after v7. It provides no versioned field-level schema sufficient for a reliable native parser. Defer direct ApiPost formats. |
| [HAR historical 1.2 specification](https://w3c.github.io/web-performance/specs/HAR/Overview.html) | Documents `log.entries[].request`, headers/query/cookies and `postData`. The draft itself is abandoned; support the widely used 1.2 request interchange subset explicitly, not an active W3C-standard claim. |

The first attempted Apifox guessed URL returned 404; the actual export page was
found through `https://docs.apifox.com/llms.txt`. `docs.apipost.net` and its
`llms.txt` returned a client-rendered app shell, not format documentation; the
current `wiki.apipost.cn` source above supplied the usable evidence. No real
customer or authentic vendor-export fixture was supplied. New compatibility
fixtures must be labeled synthetic, shaped to the documented Postman contract;
do not report them as end-to-end validation of particular vendor versions.

UI/README wording must distinguish “Apifox / ApiPost: export as Postman first”
from direct format support. Neither the importer nor its UI can recover data
already omitted by the exporting tool. Both vendors document OpenAPI export
limitations for duplicate method/path operations, strengthening the choice of
Postman as the first migration route.

## Shared backend-to-UI contract

Keep `internal/service/service.go`, `internal/model/*`, `internal/gui/app.go`,
the fake services and existing auth package unchanged. Add the following public
importer contract in `internal/importer/result.go`; freeze its exact names in
the backend report before UI work starts:

```go
// Keep existing KindUnknown, KindPostman and KindCurl values.
const (
    KindNativeCollection Kind = "reqo-collection"
    KindNativeEnvironment Kind = "reqo-environment"
    KindPostmanEnvironment Kind = "postman-environment"
    KindHAR Kind = "har"
    MaxInputBytes = 32 << 20
)

type Diagnostic struct {
    Code string // stable machine code; no English UI prose
    Path string // structural path with array indexes, never payload values
}

type Result struct {
    Kind Kind
    Collection *model.Collection
    Environment *model.Environment
    Request *model.Request
    Warnings []Diagnostic
}

type ParseError struct {
    Code string
    Path string
}

func (e *ParseError) Error() string // code/path only; no source data
func Parse(input string) (Result, error)
```

Successful Parse returns exactly one non-nil payload and zero or more warnings.
Failure returns no usable payload and a typed error, with a zero result.
`Parse` never reads local files, invokes commands/scripts, changes services,
resolves variables or accesses the network. Output does not alias mutable
caller data. UI derives counts by walking the returned tree and counting rows;
no additional service is necessary. Existing `Postman` and `Curl` signatures
remain callable; `Postman` uses the same stricter conversion for blocking cases,
and `Parse` is the supported API for fidelity diagnostics. Document that legacy
`Postman` cannot expose recoverable warnings; new UI must call `Parse`.

Required error codes: `invalid_json`, `unknown_format`, `unsupported_format`,
`unsupported_version`, `ambiguous_format`, `input_too_large`, `limit_exceeded`,
`invalid_structure`, `invalid_auth`, `unsupported_auth`, `unsupported_body`,
`invalid_curl`. Required warning codes: `scripts_omitted`, `metadata_omitted`,
`examples_omitted`, `path_variables_unresolved`, `dynamic_variables_unsupported`,
`local_file_references`, `form_metadata_omitted`, `transport_flags_ignored`,
`har_response_omitted`, `har_cookies_omitted`. Backend may consolidate repeated
warnings by code/path, but cannot drop warning categories to fit the UI.
Any additional code requires a matching zh/en UI mapping before completion.

Detection checks mutually exclusive top-level discriminators, not filename or
vendor-name substrings: Postman `info`/`item`, native `version`/`items` or
`variables`, Postman environment `_postman_variable_scope`/`values`, and HAR
`log`/`entries`. Require a JSON object, one complete JSON document, optional
leading BOM/whitespace, valid types and structural agreement. Reject ambiguous
shapes and recognizable unsupported OpenAPI/Swagger, Insomnia, bulk dump or
backup shapes with an actionable supported-route message. Detect may identify
a malformed candidate; Parse must validate it without trying a different format
after that validation fails. Do not classify any arbitrary `name` object as an
environment. Native environment requires schema version 1, name and variables,
with no collection/backup markers.

Apply the 32 MiB UTF-8 input limit to paste and file entry alike. Bound tree depth
at the existing 256 levels, aggregate nodes/requests at 10,000, and aggregate
KV/form rows at 100,000; exceedance rejects the whole input. Check limits before
recursive conversion and ID allocation where practical. Validate names, item
shapes, request presence, known body/auth/options enums, sensible timeout values,
and contradictory fields. JSON decoding must not silently accept wrong types or
duplicate object keys: reject duplicate keys, while retaining duplicate entries
in intentional arrays. Never print raw decoder errors containing source values
into the UI. Structural paths use indexes, such as `$.item[2].request.auth`.

## Mapping and data-integrity rules

| Concern | Required behavior |
| --- | --- |
| Names/folders/order | Preserve Unicode names, sibling order, duplicate names, nested and empty folders. Reject a node containing both a folder tree and a request, missing/blank required names, unknown executable item kinds or malformed request nodes; never skip an entire request silently. HAR has no request names: use method plus URL as the request name and stable technical collection name `HAR`; explain source in preview. |
| IDs/import destination | All successful imports create new identities. `Collections.Import` owns final collection/tree re-ID; preserve item/request identity equality. Native imported IDs are untrusted and never used as filenames. Environment save uses `storage.NewID()` and version 1. Same-name imports coexist; no merge/replace or automatic environment activation. |
| Native schema | Reimport raw exported collection or environment version 1; preserve all current model fields. Strictly reject unknown fields/versions and invalid or contradictory tree shapes so a newer export is not silently truncated. Allow missing additive legacy auth/options fields with their existing zero-value semantics. Existing empty request auth remains no-auth, not inheritance. |
| Auth hierarchy | Postman omitted/null request/folder auth → `inherit`; collection omitted/null → `none`; explicit `noauth` → `none` barrier. Preserve Bearer/Basic/API-key values and placeholders at their original scope; accept documented array auth and existing legacy object auth. Do not flatten parents. Native retains its stored auth verbatim after structural validation. |
| Unsupported auth | OAuth1/OAuth2/Digest/AWS/NTLM and unknown schemes block the whole import with a structural diagnostic; never become None or inherit. Invalid API-key location blocks import; empty/malformed auth types block too. Do not validate templated credentials as expanded header values or invent token exchange. Explicit Authorization/API-key headers remain explicit headers. |
| Variables | Keep `{{name}}` literal, order, disabled state and descriptions. Postman collection variables stay collection-scoped; environment entries stay separate. Postman `enabled:false` maps to Disabled; missing enabled means enabled; `type:secret` does not create vault protection. Preserve strings and compact JSON lexical values for numeric/boolean/object values as current importer does. Native copies exact strings. Environment precedence over collection and last-enabled-row precedence remain existing behavior. |
| Postman environments | Require `_postman_variable_scope:"environment"`, nonblank name and `values` array; map `key,value,enabled` and descriptions. Reject global scope instead of quietly changing scope. An export contains only exported values; never fabricate missing local/current/vault values. Detect vault/dynamic placeholders and warn they are not resolved by Reqo; retain their text. |
| URLs/query/headers | Preserve templates, repeated rows, disabled flags and descriptions. Structured Postman query is authoritative even when explicitly empty; do not double-import raw URL query. Keep fragments. Preserve unsupported request-local path variables literally with `path_variables_unresolved` warning; user must edit before sending. Never promote these into collection variables and accidentally change sibling requests. Disabled headers do not override generated auth; enabled explicit empty headers do. |
| Body | Preserve none/raw/JSON/urlencoded/text-and-file multipart/binary references. Preserve raw language-derived Content-Type where representable without overriding an explicit header; unsupported presentation-only raw languages are raw text. Postman GraphQL body mode and unknown body modes block import this cycle. File references remain literal paths; never open them during parsing or preview, and require acknowledgement that files must be reselected/checked before sending. Warn on per-part descriptions/content types or other metadata absent from `FormField`. Respect a disabled Postman body by mapping no active body with an omission warning when content was supplied. |
| Scripts/examples/descriptions | Native scripts are retained exactly but never executed by import. Postman request/folder/collection events are omitted with `scripts_omitted`, because Reqo's Otto/pm subset is not full Postman and has no shared lifecycle model. Saved examples/responses warn `examples_omitted`. Preserve descriptions where `model.KV.Description` exists (string or documented description content); collection/request/folder notes and nonrepresentable metadata warn `metadata_omitted`. No claim of lossless Postman import. |
| Postman options/extensions | Nonempty protocol-profile/certificate/proxy/other behavior extensions absent from Reqo warn `metadata_omitted`; if an unknown feature changes auth or body encoding, block instead. Allow inert source IDs/version/export timestamps without a warning. Nonempty unknown extension fields receive a generic structural omission warning rather than disappearing silently. |
| HAR requests | Require `log.version:"1.2"` and entries array, preserving entry order and repeated request headers/query values. Prefer explicit queryString rows without duplicating URL query; if missing, split the URL query. Map auth only through captured explicit headers/query, with `AuthNone`; never infer credentials or shared auth. Omit responses/timings/pages with an explicit aggregate `har_response_omitted` notice; create no history. |
| HAR bodies/cookies | Text `postData` maps to raw with its MIME type (JSON may use JSON mode when equivalent); urlencoded params map to ordered rows; multipart text params map to text form fields. Missing multipart file payloads/filename-only params and binary/encoded body extensions block, not imaginary upload paths or empty bodies. If both params and text have conflicting representations, block. Captured Cookie headers are preserved; `request.cookies` without a Cookie header is omitted with `har_cookies_omitted` so the user can supply one. Responses never become a cookie jar. |
| curl | Keep current command grammar and no-execution/no-file-read behavior. Preserve existing URL/auth/header/body conversion. Detect `-k/--insecure`, `-L/--location` (including combined switches) and `--compressed` with a transport warning since current conversion ignores these. Do not silently change TLS defaults. Unknown flags and multiple URLs continue to fail. |

Recoverable omissions require an explicit preview acknowledgement, not a toast
after data has already been written. Blocking errors never offer “import anyway”.
The fidelity guarantee is preservation of the supported subset plus disclosure,
not a promise that every imported request immediately succeeds on its server.

## UI behavior and actual Viva API constraints

Read the installed Viva v0.6.0 source, not hypothetical APIs:
`ui/effects.go`, `ui/elements.go` (`ModalSpec`, `ModalWith`, `Scroll`, `Wrap`,
`Checkbox`), `ui/page.go` (`Ctx.Key`/`KeyInt`), `app/system.go` (`FileFilter`,
`OpenFileOptions`, `OpenFiles`), and `examples/guides/overlay/main.go`. Also
inspected the background guide; its raw goroutine example does not override
Reqo's stricter effect-only rule. Use `ui.NewEffectKeyed`/`ui.Effect`/`Take`;
changed keys cancel old work and late results are dropped. Effects must never
capture UI context/boxes or mutate App state off the UI thread.

- Keep all state inside `importerState` in `internal/gui/importer.go`. The shell's
  existing `importButton`/`importDialog` entry points stay unchanged.
- Input stage has paste field, file action, supported-format/help copy, Cancel
  and a localized Review action. Offer JSON, HAR and all-files filters via the
  real `[]app.FileFilter` API; choose only one file. No YAML support implied.
- Parse potentially large text in a keyed effect using an immutable captured
  input/job; check cancellation before/after parsing. On file open, existing
  bounded read effect remains. Disable competing actions while busy. Dismissal
  must stop declaring the effect; stale file/parse results cannot reopen a dialog
  or import anything. Reopening and loading the same file must create a fresh
  effect key. No goroutines, timers, polling or work during paint.
- Successful review shows localized detected format, destination name,
  request/folder/variable counts and structural warning rows inside bounded
  scrollable content. Show no credential values or whole request URLs in
  diagnostic messages; paths are structural only. A help note names the verified
  Apifox/ApiPost Postman export routes and says native formats are unsupported.
- Warnings require one explicit unchecked acknowledgement before final Submit
  is enabled. Clean imports still have review and final Submit. Edit/Back returns
  to input; any input/file change invalidates review and acknowledgement.
- Final Submit uses only the reviewed result, once, via `c.After`. Persist a
  collection with `Collections.Import`, an environment with fresh-ID
  `Environments.Save`, or open a curl request through `a.OpenRequest(req, "")`.
  No input reparsing or service mutations before final Submit. Successful saves
  close, clear state and toast the correct entity type. Existing collection and
  environment list refresh mechanisms observe their services.
- On save failure retain input and reviewed result, show localized inline
  feedback and allow retry. A repeated click after success cannot create another
  entity. Cancel/Escape/outside dismissal never writes or opens tabs; source
  input may be retained for retry but acknowledgement must reset on reopen.
- Use theme/palette and Space/metrics tokens for all new/updated UI. Ensure
  960×600 and 1280×800, zh/en and light/dark layouts are usable. Warning paths
  wrap, lists scroll, final controls remain reachable; no new icons are required.
- All human-facing strings, format names/help, diagnostic mappings, size errors,
  preview summaries and success/failure copy belong in `internal/i18n/import.go`
  with zh and en fields. Keep existing field names usable for regression tests;
  add fields for Review/Back/acknowledgement/diagnostics instead of repurposing
  unrelated sections. Do not expose parser `err.Error()` as UI text.

## Exact sequential phase ownership and dispatch specifications

Phase 1 owns only this document. The following ownership is a proposed dispatch
contract for the coordinator to include explicitly in later tasks; no code edits
are authorized by this planning phase.

| Phase | Exact files allowed | Deliverable and gate |
| --- | --- | --- |
| 2: backend | Existing `internal/importer/importer.go`, `internal/importer/postman.go`; new `internal/importer/result.go`, `internal/importer/native.go`, `internal/importer/environment.go`, `internal/importer/har.go`, `internal/importer/diagnostics.go`, `internal/importer/json.go`; existing `internal/importer/importer_test.go` only to update superseded silent-loss expectations; new `internal/importer/expansion_test.go` and `internal/importer/testdata/expansion.json` | Implement/freeze Parse, format detection, limits/validation, typed diagnostics, strict supported mappings and focused behavior coverage. Keep curl.go unchanged; Parse can tokenize/inspect curl flags with same-package helpers. Keep old public signatures, explain intentional stricter behavior. Run all four required checks. Report supported/blocked cases and actual diagnostic codes before UI dispatch. |
| 3: UI | `internal/gui/importer.go`, `internal/i18n/import.go`, existing `internal/gui/importer_test.go`, new `internal/gui/import_expansion_smoke_test.go`, `README.md` | Wire shared Parse contract, input/review/acknowledge/commit flow, effects/cancellation, one-entity persistence and bilingual feedback. Adapt existing one-click import and exact-picker-filter tests to the deliberate review flow; retain their error/cancel coverage. Add focused smoke coverage. Update README import/support limits and remove the now-obsolete native-reimport deferral, retaining the auth/backup caveats. Run all four checks. No new service or shell dependency. |
| 4: tests | Existing `internal/importer/expansion_test.go`, `internal/importer/testdata/expansion.json`; new `internal/importer/import_regression_test.go`, `internal/gui/import_expansion_regression_test.go`, `internal/service/store/import_expansion_test.go`, `internal/i18n/import_expansion_test.go`; existing `docs/import-expansion-plan.md` only for final actual support/check evidence | Add independent behavior, real-store, auth-send, limits, malformed/adversarial and headless UX regressions below. Production files are read-only. Report implementation defects to coordinator for a narrowly owned fix before rerunning; do not silently expand ownership. Run all four checks and diff hygiene. |

No worker owns `service.go`, model, app.go, harness, fake service, auth,
transport, syncing, environment UI or collection UI in this cycle. The existing
service APIs suffice. If an unexpected integration defect requires one of those
files, report the exact smallest change and request explicit ownership from the
coordinator before editing; do not solve it by broad refactoring.

## Validation and user-observable acceptance criteria

Backend phase adds focused tests in `expansion_test.go`; test phase extends them
without mirroring helper implementation. Suggested test names describe behavior,
not a requirement to split every row into a separate test function:

| Tests | Observable proof |
| --- | --- |
| `TestImportNativeCollectionRoundTrip`, `TestImportNativeEnvironmentRoundTrip` | Export-shaped JSON retains all current fields, nested shared auth/None barriers, options, scripts, disabled rows, descriptions, Unicode and empty folders. Import twice with colliding/source IDs; new identities coexist, original entities and input remain unchanged. |
| `TestImportPostmanCompatibleExports` | Synthetic v2.0/v2.1, Apifox-compatible and ApiPost-compatible Postman fixtures retain nesting, duplicate request names, repeated query/headers, templates and supported bodies/auth. Explicitly label these as compatibility-shape tests, not real vendor exports. |
| `TestImportUnsupportedSemanticsAreVisible` | Unknown auth, malformed auth/API-key placement, unknown/GraphQL bodies and malformed item nodes fail before persistence. Events/examples/notes/path variables/dynamic placeholders/nonrepresentable form metadata yield correct structural warning codes; no credential marker appears in diagnostics. Update the existing unknown-parts/auth-barrier tests for deliberate rejection. |
| `TestImportEnvironmentScopesAndDisabledRows` | Environment `enabled:false`, missing enabled, duplicate keys, JSON scalar values, descriptions and placeholders survive. Globals/bulk/unsupported scope fail; disabled environment rows do not shadow collection values and active environment is unchanged. |
| `TestImportHARRequestsAndOmissions` | Repeated rows, request order, text/JSON/urlencoded/multipart-text bodies and captured headers survive. Response/history omission is disclosed; cookies lacking a header warn; filename-only multipart, encoded/binary bodies and contradictory body representations block. |
| `TestImportValidationIsBounded`, `FuzzParseNeverPanics` | BOM/whitespace, empty/trailing/invalid JSON, duplicate object keys, ambiguous discriminators, future native versions/fields, wrong types, nulls, contradictory shapes and each limit boundary fail safely. Malformed known formats do not fall through. Include seeds for every new kind. |
| `TestImportReviewAcknowledgementAndEdit`, `TestImportCancelAndStaleEffects` | Driver clicks prove no collection/env/tab change during paste/load/review; warnings prevent Submit until acknowledged; edit resets acceptance; Cancel/Escape/outside dismissal and late effects cannot import; same-file retry and duplicate-submit behavior are correct. |
| `TestImportDiskFailureRetryAndReload` | Real temporary store and actual write failure leave existing bytes/settings/history intact with no empty/orphan environment. Repair and retry once; successful result survives Store/App recreation. Test both collection and environment failures. |
| `TestImportedAuthSendPreservesHierarchy` | GUI opens an imported saved request, edits parent auth, chooses imported environment explicitly, and sends through real RequestService to `httptest.Server`. Verify live nearest parent, None barrier, environment-over-collection variables, disabled rows, explicit empty/repeated header overrides and query API key without damaging unrelated repeated query values. No external network. |
| `TestImportLanguagesThemesAndLayout`, `TestImportCopyCoversDiagnostics` | Headless snapshots/hit bounds at zh/en × light/dark × two viewport sizes show complete actionable review/errors; acknowledgement and final controls remain reachable with long warnings. Every public backend code has zh/en copy; format help distinguishes compatibility route from native support. Existing auth and import tests continue passing. |

Use `t.TempDir()` and `t.Setenv("REQO_HOME", ...)`, real store APIs and the shared
headless harness. Wrap existing service interfaces in test-local failure spies
where useful; do not change the production fake or shared harness. Assert no
request sender invocation until the user explicitly sends. Do not fetch schemas,
read imported upload paths, or run vendor scripts in tests. If a test package
needs fixture data, create it locally or read an explicitly owned checked-in
fixture, not user configuration.

Every implementation/test phase must finish with these commands and report
their real exit statuses and tails:

```text
gofmt -l .                  # must be empty
go vet ./...
go tool viva vet ./...
go test ./... -count=1
git diff --check
```

The phase is not complete with a failing check unless the coordinator accepts a
documented outside-ownership cause. Phase 4 must not describe snapshot/hit-bound
validation as native-window pixel verification. No tests are added in this
documentation-only phase, and full code checks are intentionally deferred to
the phases that change code. Planning validation is the source/API inspection,
only-owned-file diff review, `git diff --check`, and local documentation commit.
Read-only discovery produced expected missing-path/404 errors noted above; these
are investigation limitations, not hidden passing code checks.

## Deferred priorities after this bounded cycle

1. OpenAPI/Swagger JSON, then YAML: strong migration demand, but server/template
   selection, parameter serialization, refs, composed schemas, examples and
   security alternatives need their own explicit mapping and fixtures. No YAML
   dependency or remote `$ref` fetching in this cycle.
2. Direct Apifox/ApiPost-native projects: obtain sanitized real exports and
   version identifiers, document native auth/environment/test semantics, then
   add version-scoped support. A vendor name alone is not a format contract.
3. Insomnia: resource graph/workspace/env inheritance and expression-template
   syntax need dedicated handling. Do not accept it as Postman by heuristic.
4. OAuth2 lifecycle and other auth schemes: model token acquisition/refresh and
   storage explicitly; current inheritance refactor is already implemented.
5. Postman scripts with fidelity reporting, collection runner/data-driven
   assertions, cookie jar, richer documentation/examples, SSE/other protocols,
   and request-local variables. Existing limited scripts, GraphQL editor,
   WebSocket support, history, collection/environment CRUD, sync and updates
   should be considered before calling those entire app areas “missing”.

Multi-file transactions, drag/drop/bulk/zip import, destructive restore/merge
and vendor cloud APIs are separate features. This cycle ends when the supported
inputs are reviewable, preserve their documented subset, disclose omissions,
create new durable entities safely, and pass the checks above.
