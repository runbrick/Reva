# Reqo

A local, account-free API client. Written in Go with [Viva](https://github.com/vivaui/viva):
no WebView, no Electron, a single native binary (about 5 MB stripped on Windows).

## Build and run

```sh
go run ./cmd/reqo                                    # development
go build -ldflags="-s -w" -o bin/reqo ./cmd/reqo     # release binary
go tool viva build                                   # packaged app (see `go tool viva help build`)
```

Data lives in `<user config dir>/Reqo` (`%AppData%\Reqo` on Windows,
`~/Library/Application Support/Reqo` on macOS). Set `REQO_HOME` to use another
directory. Every file is indented JSON, so the directory can be kept in Git:

```
settings.json
history.json
collections/<id>.json
environments/<id>.json
```

## Layout

```
cmd/reqo/              main: builds the services and opens the window
internal/model/        plain data types (Request, Response, Collection, Environment, ...)
internal/storage/      data directory, atomic JSON read/write, IDs
internal/vars/         {{variable}} substitution
internal/service/      service interfaces (the contract between UI and back end)
  fake/                in-memory services for tests
  request/             HTTP sending                       (PR: svc-http)
  store/               history, collections, envs, settings on disk (PR: svc-storage)
internal/importer/     Postman collection and curl import (PR: import)
internal/script/       pre-request / test JavaScript      (PR: scripts)
internal/i18n/         every user-visible string, zh + en, one file per feature
internal/gui/          Viva UI: app.go is the shell, one file per feature
```

The UI depends only on the interfaces in `internal/service`; Viva is pure Go, so
the UI calls Go methods directly and types are checked by the compiler. Network
calls run in a `ui.Effect` off the UI thread and can be cancelled.

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
| Windows | `./scripts/build.ps1 -Arch amd64` in PowerShell | `dist/windows-amd64/reqo.exe` and `reqo-<version>-windows-amd64.zip` |
| macOS | `sh ./scripts/build.sh` | `dist/darwin-<host-arch>/reqo`, `Reqo.app`, and `reqo-<version>-darwin-<host-arch>.dmg` |

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
`reqo-stripped.exe` as a bare stripped baseline and packages the resource-bearing
`reqo.exe` into a portable ZIP. The macOS package contains `Info.plist`, an
`.icns` generated from the PNG, and the executable; its DMG includes an
Applications shortcut for drag-to-install. These commands are equivalent to
the main packaging steps (replace the architecture as needed):

```powershell
# Windows host
go tool viva build --target windows --arch amd64 -o dist/reqo.exe
go tool viva package --target windows --arch amd64 --format zip --no-sign --program dist/reqo.exe -o dist
```

```sh
# macOS host
go tool viva build --target darwin --arch arm64 -o dist/reqo
go tool viva package --target darwin --arch arm64 --format app,dmg --no-sign --program dist/reqo -o dist/Reqo.app
```

The scripts and CI create unsigned artifacts. Distribution signing is separate:
Viva supports Windows certificates and Apple signing/notarization credentials
(see `go tool viva help package`); omit `--no-sign` when using those credentials.
Downloaded unsigned apps may be blocked by macOS Gatekeeper or flagged by
Windows SmartScreen. A Windows NSIS installer is also available with
`--format nsis` when `makensis` is installed; the portable ZIP needs no installer
tool.

On every push and pull request, [CI](.github/workflows/ci.yml) runs Go 1.26 on
`windows-latest` and `macos-latest`: formatting, Go vet, Viva vet, all tests,
the stripped release build, and packaging. CI disables Git's automatic CRLF
conversion before checkout so `gofmt` sees LF on both hosts. Each build script
prints artifact sizes without enforcing a size limit; the macOS script also
reports the sum of all files in `Reqo.app`.
CI uploads the ZIP or DMG as a workflow artifact.

To publish a release, set `[app] version` in `viva.toml` and the matching
`internal/update/update.go` `Current` constant, then push a matching `vX.Y.Z`
tag. The [Release workflow](.github/workflows/release.yml) verifies the tag,
runs the full test suite on Windows and macOS, packages their native artifacts,
and publishes the ZIP, DMG, and `SHA256SUMS` to GitHub Releases. A release
already published under that tag is never overwritten. The Settings page checks
the latest published release only when you click **Check for updates**; it opens
the GitHub download page for a newer version and does not replace the running app.

Measured on Windows/amd64 with Go 1.26.3 and Viva 0.6.0 using
`./scripts/build.ps1` on this branch:

| Artifact | Bytes | Decimal MB |
| --- | ---: | ---: |
| Stripped `cmd/reqo`, before app resources (`reqo-stripped.exe`) | 14,998,016 | 15.00 |
| Packaged Windows executable, with icon and version resources (`reqo.exe`) | 15,032,320 | 15.03 |
| Portable Windows package (`reqo-0.2.0-windows-amd64.zip`) | 6,369,720 | 6.37 |

The Windows app resources add 34,304 bytes. Sizes change as features are added;
rerun the scripts for current measurements. macOS packaging is configured for
its native CI runner and has not been executed on this Windows development
host.
