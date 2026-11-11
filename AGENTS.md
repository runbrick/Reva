# Agent instructions for Reqo

## Project context (read first)

Repo: Reqo, a local Postman-like API client in Go 1.26 using the Viva GUI framework
(github.com/vivaui/viva v0.6.0, pure Go, self-drawn UI, no WebView). Module path
`github.com/runbrick/reqo`. Read `README.md`, `TASK.md`, and
`.claude/skills/building-with-viva/SKILL.md` (the Viva rules; follow them strictly) before coding.
Viva source with examples is in the Go module cache:
`$(go env GOMODCACHE)/github.com/vivaui/viva@v0.6.0` (see `examples/guides/*`, `examples/recipes/*`,
and `ui/*.go` doc comments). Read the real API there instead of guessing.

Foundation already on main (do not redesign it):
- `internal/model` data types (Request, Response, KV, Body, Auth, Collection, Item, Environment, Settings, HistoryEntry) + tree helpers.
- `internal/storage` (Dir(), ReadJSON, WriteJSON atomic, NewID; `REQO_HOME` env overrides data dir).
- `internal/vars` ({{var}} Expand / Apply / Unresolved).
- `internal/service/service.go` interfaces; `internal/service/fake` in-memory implementations.
- `internal/i18n`: typed string tables, zh (default) + en. Each feature owns ONE section file.
- `internal/gui/app.go`: the App shell (layout, tabs slice, send pipeline via ui.Effect, toasts). One file per feature with stub functions whose signatures the shell calls.
- `internal/gui/harness_test.go`: headless app.Driver test harness (newHarness, click, typeInto, shows, waitFor, pump).

## Visual design (round 2)

The target look is the screenshot `docs/design/target.png`: a clean, light, blue-accented
desktop client. Layout, owned by `app.go`: custom titlebar strip on top (`titlebar.go`), a
72 px icon rail on the left (APIs / Collections / Environments / Settings), a list sidebar,
and the workbench (request tabs, request editor, response) or a full-width History /
Settings page (`App.Main`).

- Colors: ONLY from `c.Theme()` and `a.palette` (`internal/gui/theme.go`: Rail, Sidebar,
  Surface, RailActive, TableHeader, method colors, Warning, Code* colors). Light and dark
  must both look finished. No RGB literals in feature files.
- Lengths: the `Space1..Space6` tokens (4/8/12/16/24) and `theme.Metrics` (radii, font sizes,
  control height). Method badges: `a.MethodBadge(c, method)`; status colors:
  `a.palette.StatusColor(theme, code)`.
- Icons: `internal/gui/icons` (Lucide, ISC license, compiled with `go tool viva icons`):
  `ui.Icon(c, icons.Send).Size(16, 16).Color(...)`, `ui.Button(c, label).Icon(icons.Plus)`.
  To add an icon, download its SVG from
  `https://cdn.jsdelivr.net/npm/lucide-static@0.460.0/icons/<name>.svg` into
  `internal/gui/icons/svg/`, run
  `go tool viva icons -o icons.go -pkg icons -license LICENSE-lucide.txt svg` inside
  `internal/gui/icons`, and commit the SVG and `icons.go`. List added icons in your report:
  `icons.go` is generated, so on a merge conflict the coordinator regenerates it from all SVGs.
- Model fields added for round 2: `KV.Description`, `Response.Cookies`, `Response.Timeline`,
  `Request.Options` (`RequestOptions`, resolved with `Options.Resolve(settings)`),
  `Settings.RequestTimeoutSec / NoFollowRedirects / InsecureTLS`, `Settings.Theme` may be
  `"system"`.

## Rules for every feature PR

1. Work only in your own worktree/branch. Commit your work on that branch with clear messages. Do NOT push, do NOT merge into main, do NOT touch main.
2. Edit ONLY the files listed under "You own". If you truly need a change in a file you do not own (app.go, model, service.go, harness_test.go, another feature's file), do not make it: make the smallest workaround inside your files and describe the needed change in your final report. Exception: you may add a new field/helper to `internal/model` ONLY if your task spec explicitly allows it.
3. All user-visible text goes in your i18n section file, as struct fields set in BOTH zh and en values (Chinese first-class). No hard-coded UI strings in gui files except technical tokens (HTTP method names, "JSON", header names).
4. State: keep your feature's UI state in your `xxxState` struct in your gui file (already declared, empty). Never keep a `ui.C` or `ui.Box` in a struct. No goroutines/timers/polling in UI code; async work uses ui.Effect. Rows built in loops use `c.Key`/`c.KeyInt`.
5. Tests: write behavior tests a user depends on. GUI tests use the harness in `internal/gui` (headless app.Driver), never a real window. Service/library tests use `t.TempDir()`, `t.Setenv("REQO_HOME", ...)`, `net/http/httptest`. No network access to the internet in tests.
6. Do not add third-party dependencies unless your task spec explicitly allows it.
7. Before finishing, ALL of these must pass in your worktree, and paste their tails in your report:
   `gofmt -l .` (empty), `go vet ./...`, `go tool viva vet ./...`, `go test ./... -count=1`.
   A failing check means the Task failed unless you explain why it is outside your files.
8. Final report (worker_done): branch name, commit hash(es), files changed, what you built, test names added, check output, and any requested changes to files you do not own.
