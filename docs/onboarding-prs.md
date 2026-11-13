# Local onboarding PR guide

These are four proposed local PRs. No remote PR has been created, and nothing
has been pushed or merged. The coordinator's integration branch,
`runbrick/feat-onboarding-update`, and `main` remain untouched by this work.

## Branches, commits and dependency order

All work starts from snapshot `731391615c6801078b9529c8c059af3041b27c52`.

| PR | Proposed title | Source branch | Implementation commit | Dependencies |
| --- | --- | --- | --- | --- |
| 1 | Persist onboarding completion in settings | `runbrick/onboarding-state` | `bb688d20f87d1a52247a1202ac3b6dfef2819045` | Base snapshot |
| 2 | Add Chinese and English onboarding guide copy | `runbrick/onboarding-copy` | `0bfe432f1ebdbec60a60387002d5453dc3c1aa87` | Base snapshot; independent of PR 1 |
| 3 | Add optional getting-started guide to Settings | `runbrick/onboarding-settings` | `2346969a597afb3413f19267a8e7c0f86b95d824` | PRs 1 and 2 |
| 4 | Add disk-backed onboarding regression coverage | `runbrick/onboarding-tests` | `758a675323c35e4e7cd410f39d584ab1baa4f9ed` | PR 3, including both prerequisites |

PRs 1 and 2 can be reviewed independently against the base snapshot. PR 3's
worker cherry-picked their original commits into its own branch:

| Original commit | Equivalent commit in the UI/test stack |
| --- | --- |
| State `bb688d20f87d1a52247a1202ac3b6dfef2819045` | `b729698d7d12efa71d7b9a593c7bf45a2283f791` |
| Copy `0bfe432f1ebdbec60a60387002d5453dc3c1aa87` | `acc77133f7b883c5443ed926d7552496aced9479` |

The test child was created from PR 3's HEAD and already inherited that complete
stack. It did not cherry-pick duplicates. Its ancestry is:

```text
7313916 -> b729698 (state) -> acc7713 (copy) -> 2346969 (UI)
        -> 758a675 (regression tests) -> documentation commit
```

The documentation commit is the commit adding this file; resolve its hash with
`git log -1 --format=%H -- docs/onboarding-prs.md`. This avoids a self-referential
commit hash. Both PR 4 commits belong to `runbrick/onboarding-tests`.

For a stacked local review, compare PR 3's implementation with `acc7713`, and
compare PR 4 with `2346969`. Once prerequisites are integrated, each feature's
own commit can be applied to `runbrick/feat-onboarding-update` in the dependency
order above. Apply either the original prerequisite commits or their stack
equivalents, never both. The test child is also a complete local artifact for
reviewing all four changes together.

## PR 1 description

**Title:** Persist onboarding completion in settings

Add an optional `onboardingCompleted` flag to settings so the guide can remember
completion across app restarts. New installations and existing settings files
default to incomplete without changing schema version 1. The existing store
preserves completion when filling defaults and saving unrelated preferences;
false remains omitted from JSON.

Validation adds model JSON compatibility and real temporary-directory store
tests for omitted/false/true values, sparse settings, completion resets and
preference preservation. `gofmt -l .`, `go vet ./...`, `go tool viva vet ./...`
and `go test ./... -count=1` all passed on the original branch.

Files: `internal/model/model.go`, `internal/model/onboarding_test.go`,
`internal/service/store/onboarding_test.go`.

## PR 2 description

**Title:** Add Chinese and English onboarding guide copy

Add 15 Settings strings for a welcome guide that users can close, complete,
skip or replay from Settings. Chinese and English copy explains manually sending
requests, curl/Postman import, `{{name}}` environment variables, credentials in
the user's own configuration, local account-free data, optional encrypted
backup/sync and retrying a failed completion save.

Public `i18n.For` tests verify every field is present and separately localized,
and that both translations explain these choices with matching action labels.
`gofmt -l .`, `go vet ./...`, `go tool viva vet ./...` and
`go test ./... -count=1` all passed on the original branch. This PR is independent
of the state PR.

Files: `internal/i18n/settings.go`, `internal/i18n/onboarding_test.go`.

## PR 3 description

**Title:** Add optional getting-started guide to Settings

Settings now offers a compact, bilingual getting-started guide from its heading,
with replay after completion. The three steps explain requests/imports,
environment variables and local data with optional backups. The guide opens
only on demand and does not send requests or check the network.

Done reads fresh settings, changes only completion, saves and closes after
success. A failed save displays localized feedback and remains retryable. Skip,
Escape and standard modal dismissal close without marking completion. Replay
clears a previous error. Colors and spacing use the existing theme and tokens.

Two headless smoke tests cover zh/en, light/dark, minimum window size 960x600,
dismissal, completion/replay, save failure, latest preference preservation and
unchanged requests/history. All four required checks passed. Depends on PRs 1
and 2; no production files outside its ownership changed.

Files: `internal/gui/settings.go`, `internal/gui/onboarding.go`,
`internal/gui/onboarding_smoke_test.go`.

## PR 4 description

**Title:** Add disk-backed onboarding regression coverage

Add headless GUI regression tests using the real Settings controls and disk
store. Pending guides open from Settings; Skip and Escape leave settings
byte-for-byte unchanged. Done persists across App recreation and Store reload,
preserves preferences changed while the guide is open, and makes replay
available. Skipping a completed replay preserves completion.

Use an actual filesystem write failure to verify repeated Done attempts show
localized feedback without publishing completion. After repairing the test
filesystem, retry saves the latest preferences and survives reload. A separate
store regression verifies completion and settings roll back if history
persistence fails during a settings save, then verifies retry and reload.
Additional i18n coverage verifies Chinese fallback for missing or unsupported
saved language codes.

Snapshots check zh/en guide copy and modal layout in light/dark at 960x600 and
the default 1280x800 viewport. Tests use the shared real GUI harness and fresh
temporary directories, with real settings/history/collection/environment
services. Only the request sender is a fake that fails the test if called.
No real window, internet access, production changes, shared harness edits,
dependencies or icons were added.

Validation: all four required checks pass on the complete stack. Depends on PR
3. This PR contains the three regression test files and this guide only.

Files:

- `internal/gui/onboarding_regression_test.go`
- `internal/service/store/onboarding_regression_test.go`
- `internal/i18n/onboarding_regression_test.go`
- `docs/onboarding-prs.md`

## Review and test evidence

Existing prerequisite tests were read before adding coverage. PR 4 exercises
public actions and persisted results, without calling onboarding implementation
helpers or inspecting private guide state. Its new test names are:

- `TestOnboardingDiskLifecycle` — zh/en × light/dark, real disk, dismissals,
  latest preferences, successful completion, recreation/reload and replay.
- `TestOnboardingDiskWriteFailureCanRetry` — zh/en, repeated real write failure,
  localized error, unchanged last durable preferences, retry, reload and replay.
- `TestOnboardingCompletionRollsBackWhenHistoryPersistenceFails` — real store,
  rollback of completion/settings/history, reload, retry and history limit.
- `TestOnboardingCopyFallsBackToChinese` — all 15 guide fields for unset, `fr`
  and `zh-CN` language values.

Original branch check evidence below was supplied by each worker report; the
final stack checks were run again in the test child. Every command listed
exited 0. Empty output from the first three checks is recorded explicitly.

### PR 1 original branch check tails

```text
$ gofmt -l .
(empty)
$ go vet ./...
(empty)
$ go tool viva vet ./...
(empty)
$ go test ./... -count=1
ok   github.com/runbrick/reqo/internal/storage         1.989s
ok   github.com/runbrick/reqo/internal/syncing          9.031s
ok   github.com/runbrick/reqo/internal/update           6.806s
ok   github.com/runbrick/reqo/internal/vars             1.134s
ok   github.com/runbrick/reqo/internal/websocket        4.528s
```

### PR 2 original branch check tails

```text
$ gofmt -l .
(empty)
$ go vet ./...
(empty)
$ go tool viva vet ./...
(empty)
$ go test ./... -count=1
ok   github.com/runbrick/reqo/internal/storage         2.984s
ok   github.com/runbrick/reqo/internal/syncing          6.636s
ok   github.com/runbrick/reqo/internal/update           6.084s
ok   github.com/runbrick/reqo/internal/vars             0.045s
ok   github.com/runbrick/reqo/internal/websocket        3.710s
```

### PR 3 stack check tails

```text
$ gofmt -l .
(empty)
$ go vet ./...
(empty)
$ go tool viva vet ./...
(empty)
$ go test ./... -count=1
ok   github.com/runbrick/reqo/internal/storage         3.607s
ok   github.com/runbrick/reqo/internal/syncing          8.857s
ok   github.com/runbrick/reqo/internal/update           8.295s
ok   github.com/runbrick/reqo/internal/vars             0.043s
ok   github.com/runbrick/reqo/internal/websocket        6.901s
```

### PR 4 final stack check tails

```text
$ gofmt -l .
(empty)
Exit: 0
$ go vet ./...
(empty)
Exit: 0
$ go tool viva vet ./...
(empty)
Exit: 0
$ go test ./... -count=1
ok   github.com/runbrick/reqo/internal/service/request  3.035s
ok   github.com/runbrick/reqo/internal/service/store    0.350s
ok   github.com/runbrick/reqo/internal/storage          2.216s
ok   github.com/runbrick/reqo/internal/syncing           2.845s
ok   github.com/runbrick/reqo/internal/update            2.300s
ok   github.com/runbrick/reqo/internal/vars              0.388s
ok   github.com/runbrick/reqo/internal/websocket         2.278s
Exit: 0
```

Other relevant final full-suite lines:

```text
ok   github.com/runbrick/reqo/internal/gui              3.272s
ok   github.com/runbrick/reqo/internal/i18n             0.042s
```

Focused regression run, also exit 0:

```text
$ go test ./internal/gui ./internal/service/store ./internal/i18n -run 'TestOnboarding(Disk|CompletionRollsBack|CopyFalls)' -count=1
ok   github.com/runbrick/reqo/internal/gui              1.400s
ok   github.com/runbrick/reqo/internal/service/store    1.191s
ok   github.com/runbrick/reqo/internal/i18n             0.022s
```

`git diff --check` also passed. No production defects or changes outside owned
files were identified. Snapshot validation inspects the real painter's text,
hit regions and layout; it does not claim a native-window or pixel screenshot
review. README.md and AGENTS.md were read; TASK.md and the referenced local Viva
skill are absent. The actual Viva v0.6.0 Driver, Snapshot, ModalWith and headless
layout APIs and overlay examples were read from the Go module cache instead.
