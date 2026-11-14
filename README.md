# Reva

A local, account-free API client. Written in Go with [Viva](https://github.com/vivaui/viva):
no WebView, no Electron, a single native binary (about 5 MB stripped on Windows).

## Build and run

```sh
go run ./cmd/reva                                    # development
go build -ldflags="-s -w" -o bin/reva ./cmd/reva     # release binary
go tool viva build                                   # packaged app (see `go tool viva help build`)
```

Data lives in `<user config dir>/Reva` (`%AppData%\Reva` on Windows,
`~/Library/Application Support/Reva` on macOS). Set `REVA_HOME` to use another
directory. Every file is indented JSON, so the directory can be kept in Git:

```
settings.json
history.json
collections/<id>.json
environments/<id>.json
```

For data created before the naming correction, copy the existing data directory
to the new location or set `REVA_HOME` to its current location. This rename does
not move existing user data. Backups now use `.revabak` and the
`reva-backup-v1` envelope; backups from earlier builds must be re-exported from
their local data with this build.

## Layout

```
cmd/reva/              main: builds the services and opens the window
internal/model/        plain data types (Request, Response, Collection, Environment, ...)
internal/storage/      data directory, atomic JSON read/write, IDs
internal/vars/         {{variable}} substitution
internal/service/      service interfaces (the contract between UI and back end)
  fake/                in-memory services for tests
  request/             HTTP sending                       (PR: svc-http)
  store/               history, collections, envs, settings on disk (PR: svc-storage)
internal/importer/     bounded collection/environment, HAR and curl import
internal/script/       pre-request / test JavaScript      (PR: scripts)
internal/i18n/         every user-visible string, zh + en, one file per feature
internal/gui/          Viva UI: app.go is the shell, one file per feature
```

The UI depends only on the interfaces in `internal/service`; Viva is pure Go, so
the UI calls Go methods directly and types are checked by the compiler. Network
calls run in a `ui.Effect` off the UI thread and can be cancelled.

## HTTP authentication

Collections and folders can store shared Bearer, Basic, or API-key credentials.
Requests explicitly set to inherit use their nearest configured folder, then
the collection; folder **None** stops inheritance. Existing requests with empty
auth or **None** continue to send without generated authentication. Enabled
explicit headers, including empty values, take precedence over generated auth.
Cross-origin redirects strip Authorization and the configured API-key header
for the remainder of that send; query API keys are applied only to the initial URL.

Postman v2 imports retain collection/folder auth and request inheritance.
These fields are additive within native schema version 1; older binaries cannot
preserve their semantics. Native exports include configured shared credentials.
Secure input masking is presentation, not a credential vault. History keeps
unexpanded variable placeholders. OAuth2 lifecycle and WebSocket Auth (manual
headers remain available) are deferred.

## Import and migration

Paste content or open a `.json`, `.har`, or other text file, then choose **Review
import**. Review the detected format, destination, request/folder/variable counts,
and compatibility notices before **Confirm import**. Every notice requires an
explicit acknowledgement. Nothing is saved or opened during review. Errors keep
the input available; save failures preserve the review for retry.

Supported inputs create one new entity, without replacing existing data:

- Swagger 2.0 and OpenAPI 3.0/3.1 **JSON**, through **Swagger** (file/paste),
  **Swagger URL**, or automatic format detection. URL import accepts a directly
  accessible HTTP(S) JSON document, rather than a Swagger UI HTML page. Only
  clicking **Review import** fetches it, with cancellation, a 30-second timeout,
  a 32 MiB response limit, and at most nine followed redirects. Confirmation
  saves the reviewed result without another fetch. Relative OpenAPI servers
  resolve against the final document URL after redirects.
  Operations become request templates, grouped by their first tag. Summary,
  operation ID, method/path, simple path/query/header parameters, examples and
  defaults, JSON/text bodies and supported text forms are converted. Query objects
  using `form` style with `explode: true` become individual query rows; nested
  objects and custom object serialization remain unsupported. Local JSON
  Pointer references are resolved; external references are never fetched.
  Basic, Bearer and header/query API-key security become configurable auth with
  credential placeholders; explicit empty operation security disables auth.
  Path parameters become collection variables. Optional parameters without
  examples/defaults start disabled. **Add basePath** and **Add host** independently
  include the base path and scheme/host (OpenAPI uses the selected server URL);
  both initially start off in the dialog. Other formats retain their URLs.
  Multiple servers, media types, examples or security alternatives produce a
  selection notice: the first array entry is used; maps use sorted key order,
  and request-body media types prefer `application/json`. An empty selected server
  URL keeps paths relative (URL imports resolve against the document origin) and
  warns; another environment is never chosen automatically. Full URLs, base URL
  variables and host:port addresses in vendor path fields are retained verbatim
  with a review notice, without prepending a server. JSON examples stored as text
  are decoded when the schema is not a string; empty JSON-body placeholders with
  empty schemas become no body. Multipart binary fields are retained as disabled
  file references: import never opens them; reselect and enable before sending.
  `oneOf`/`anyOf` use the first branch with a notice. Missing local request-body
  schema definitions use `null` placeholders with a notice; the operation is kept
  for editing. Generated templates
  always require acknowledgement: review values, path variables and credentials
  before sending. Responses, documentation, schema constraints, callbacks and
  extensions are omitted. Unsupported auth, combined security requirements,
  cookie/custom-serialized parameters, binary bodies outside supported multipart
  file references, form encodings, `allOf` without explicit examples, recursive
  refs, external refs and missing parameter or requestBody object references block import.
  YAML is not supported. Generated output and reference expansion
  are bounded as well as the source document.
- Reva schema-v1 collection and standalone environment JSON. Current fields,
  shared auth, request options, disabled rows, descriptions, and empty folders
  are retained. Native scripts are retained but never executed by import.
  Unknown native fields and future versions are rejected to avoid silent loss.
- Postman Collection v2.0/v2.1 JSON and single Postman environment exports.
  Nested requests/folders, repeated rows, variables, Bearer/Basic/API-key auth,
  inheritance and explicit no-auth barriers are preserved. Supported bodies are
  raw/JSON, URL-encoded, multipart text/file references, and binary file references.
  GraphQL bodies and OAuth1/OAuth2/Digest/AWS/NTLM or unknown auth block import.
  Postman scripts, saved responses/examples, and unsupported metadata are omitted
  with notices. Dynamic/vault and request-local path variables remain literal
  and need attention before sending. Only values present in an export can be
  imported; secret fields do not gain vault protection.
- HAR 1.2 request extraction into a new collection. Requests retain order,
  repeated headers/query rows and supported textual, URL-encoded or multipart
  text bodies. Captured explicit auth/Cookie headers remain headers; auth is not
  inferred. Responses, timings and pages are omitted, not restored as history.
  Cookie lists without a Cookie header warn. Missing file payloads, encoded or
  binary bodies, and conflicting body representations block import.
- A single supported curl command opens a new unsaved request after confirmation.
  TLS, redirect and compression flags that are not applied generate notices.
  Commands are never executed. Local upload paths remain references: import
  never reads them, and files must be checked/reselected before sending.

For **Apifox**, use **Settings > Export data > Postman**. For **ApiPost**, use
**Project settings > Export project > Postman 2.1** (or copy one request as curl).
Import the resulting Postman file; Swagger/OpenAPI JSON exports can also use the
Swagger entry. Proprietary Apifox/ApiPost project exports are
unsupported. This is a compatibility route, not a lossless migration or vendor
version certification. Inspect all notices and review requests before sending.

File and pasted input share a **32 MiB** UTF-8 limit. One JSON object is accepted;
invalid JSON, duplicate object keys and ambiguous formats are rejected. Limits
are 256 nested item levels, 10,000 aggregate tree nodes/requests, 100,000 aggregate
key/value/form rows, and a defensive 1,024 JSON-container nesting cap. Split
larger exports. JSON/BOM whitespace is supported; Insomnia, YAML,
bulk dumps and backup restore are outside this importer. Encrypted backup/sync
restore remains a separate feature.

Imported collections and environments get fresh IDs. Environments are saved
without automatic activation; existing drafts and preferences remain unchanged.
Imported credentials remain configured credentials in local JSON, not a vault.
Import does not resolve Reva variables, send imported requests, or execute scripts.
Only the explicitly selected Swagger URL flow fetches a schema; file/paste parsing
stays offline. Native scripts may execute later through the normal explicit send flow.

Choose **Existing collection** to append imported requests in a separate folder,
preserving existing items and the imported auth scope. Imported variables are
shared with the destination; conflicting variable values block append and can
instead be imported as a new project.

## Checks

```sh
gofmt -l . && go vet ./... && go tool viva vet ./... && go test ./... -count=1
```

## Packaging

Use Go 1.26 and the project-pinned `go tool viva` (Viva 0.6.0). Application
identity, version, icon, output directory, and minimum macOS version are in
[`viva.toml`](viva.toml); update `[app] version` for a release. The original
[`assets/icon.svg`](assets/icon.svg) depicts a request arrow inside a Q; its
1024 x 1024 PNG rendition supplies both platforms' icons. No additional Go
dependencies or icon/resource compiler are needed.

Run the following from the repository root on the corresponding host:

| Host | Release command | Outputs |
| --- | --- | --- |
| Windows | `./scripts/build.ps1 -Arch amd64` in PowerShell | `dist/windows-amd64/reva-<version>-windows-amd64-setup.exe` (installer), `reva.exe`, and a portable ZIP |
| macOS | `sh ./scripts/build.sh` | `dist/darwin-<host-arch>/reva`, `Reva.app`, and `reva-<version>-darwin-<host-arch>.dmg` |

Windows packaging requires NSIS (`makensis`). Install it with
`winget install --id NSIS.NSIS --exact`, then run the Windows build command.
If NSIS is unpacked elsewhere, set `NSIS_HOME` to its `Bin` directory.
The installer installs for the current user without administrator access and
provides Start Menu shortcuts and an uninstaller. User data in `%AppData%\Reva`
is stored separately from the program.

Both scripts accept `arm64` or `amd64` (on macOS, for example,
`sh ./scripts/build.sh amd64`). macOS requires Xcode command-line tools
(`xcode-select --install`) for Viva's Objective-C Metal bridge. **Cross-building
macOS from Windows does not work:** Viva explicitly requires a macOS host for
both `build --target darwin` and `package --target darwin`. A Mac can build
either macOS architecture with its Apple toolchain; CI builds its runner's
native architecture, and Windows CI builds amd64.

Viva release builds use `-trimpath`, `-s -w`, and an empty build ID. Windows
builds also use the GUI subsystem and embed the PNG as an icon plus name,
publisher, and version resources in the executable. The Windows script keeps
`reva-stripped.exe` as a bare stripped baseline and packages the resource-bearing
`reva.exe` into an NSIS installer and a portable ZIP. The macOS package contains
`Info.plist`, an `.icns` generated from the PNG, and the executable; its DMG includes an
Applications shortcut for drag-to-install. These commands are equivalent to
the main packaging steps (replace the architecture as needed):

```powershell
# Windows host
go tool viva build --target windows --arch amd64 -o dist/reva.exe
go tool viva package --target windows --arch amd64 --format nsis,zip --no-sign --program dist/reva.exe -o dist
```

```sh
# macOS host
go tool viva build --target darwin --arch arm64 -o dist/reva
go tool viva package --target darwin --arch arm64 --format app,dmg --no-sign --program dist/reva -o dist/Reva.app
```

The scripts and CI create unsigned artifacts. Distribution signing is separate:
Viva supports Windows certificates and Apple signing/notarization credentials
(see `go tool viva help package`); omit `--no-sign` when using those credentials.
Downloaded unsigned apps may be blocked by macOS Gatekeeper or flagged by
Windows SmartScreen. The portable Windows ZIP remains available without running
an installer.

On every push and pull request, [CI](.github/workflows/ci.yml) runs Go 1.26 on
`windows-latest` and `macos-latest`: formatting, Go vet, Viva vet, all tests,
the stripped release build, and packaging. CI disables Git's automatic CRLF
conversion before checkout so `gofmt` sees LF on both hosts. Each build script
prints artifact sizes without enforcing a size limit; the macOS script also
reports the sum of all files in `Reva.app`.
CI installs NSIS on Windows and uploads the installer, ZIP, or DMG as workflow
artifacts.

To publish a release, set `[app] version` in `viva.toml` and the matching
`internal/update/update.go` `Current` constant, then push a matching `vX.Y.Z`
tag. The [Release workflow](.github/workflows/release.yml) verifies the tag,
runs the full test suite on Windows and macOS, packages their native artifacts,
and publishes the Windows installer, ZIP, DMG, and `SHA256SUMS` to GitHub Releases. A release
already published under that tag is never overwritten. The Settings page checks
the latest published release only when you click **Check for updates**; it opens
the GitHub download page for a newer version and does not replace the running app.

Measured on Windows/amd64 with Go 1.26.3 and Viva 0.6.0 using
`./scripts/build.ps1` on this branch:

| Artifact | Bytes | Decimal MB |
| --- | ---: | ---: |
| Stripped `cmd/reva`, before app resources (`reva-stripped.exe`) | 14,998,016 | 15.00 |
| Packaged Windows executable, with icon and version resources (`reva.exe`) | 15,032,320 | 15.03 |
| Portable Windows package (`reva-0.2.0-windows-amd64.zip`) | 6,369,720 | 6.37 |

The Windows app resources add 34,304 bytes. Sizes change as features are added;
rerun the scripts for current measurements. macOS packaging is configured for
its native CI runner and has not been executed on this Windows development
host.
