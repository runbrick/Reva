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
