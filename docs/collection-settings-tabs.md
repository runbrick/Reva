# Collection configuration in workbench tabs

Branch: `runbrick/collection-settings-tabs`, based on parent
`runbrick/onboarding-orchestration` at `c460f09`.

The collection context menu starts with **集合设置 / Collection settings**.
Folders offer **文件夹设置 / Folder settings**. Each target opens one configuration
tab alongside request tabs, with a folder icon instead of an HTTP method badge.
The existing Authentication and Collection variables menu shortcuts select
sections in that same tab. Collection roots retain their tree expansion behavior.

Configuration fills the workbench below the tab strip. It has a target heading,
an unsaved indicator, Save, Discard and reload, and General / Authentication /
Collection variables navigation. General edits the name inline. Folder settings
have General and Authentication, including the inherited parent preview; folders
have no variables section. Configurations never show the request URL bar, Send,
response pane or request/response splitter. The existing shared auth editor masks
Bearer tokens, Basic passwords and API-key values, including variable placeholders.
All new copy is in the Collections zh/en string tables and uses existing theme,
spacing and icon assets. No icons or dependencies were added.

## Draft and persistence semantics

- Each open target owns an independent draft in `collectionsState.settings`, keyed
  by its tab ID. Section changes, request tab changes, History/Settings navigation
  and reopening an already open settings entry retain that draft. Opening a tab
  performs no write. The baseline preserves legacy empty auth without normalizing
  it merely by displaying the editor.
- Save and Ctrl/Cmd+S retain the active configuration tab. They fetch the latest
  collection from the service and apply only name, auth or variables that differ
  from this draft's baseline. Folder mutation clones the latest tree first.
  Requests, newly inserted items, other folders and unchanged latest fields survive.
  A successful save reloads the new baseline and clears the dirty/error state.
- Unsaved auth and variables are isolated from requests. Request sends, cURL and
  inheritance continue reading saved service data; the next read sees saved changes.
- Save failures show fixed localized text, never raw backend errors, and retain the
  draft for retry. A deleted collection, deleted folder or changed folder type shows
  an inline missing-target message. Save cannot recreate a missing target. Tab
  headings read current saved names, so external rename is reflected without
  overwriting an unsaved name draft.
- Discard and reload explicitly reads current saved values. Escape and clicking
  outside the editor do not dismiss a configuration or persist its draft.
- Dirty close uses the existing unsaved-discard confirmation; cancelling retains
  the draft, confirming discards it. Clean tabs close directly. Closing removes
  their settings state immediately. The bounded ten-entry reopen history stores
  configuration target/section only, with no configuration credentials or discarded
  draft; reopening loads the latest saved value. Request reopen semantics remain.

## Integration

`Tab.Kind` has a zero-value request kind for compatibility; configuration has only
a collection ID and optional folder target ID, with a zero request. `OpenRequest`
can focus a real request or open a new request while configuration is active; it
cannot reuse/convert the configuration. Request creation, history, curl import and
search therefore keep its draft. `RequestSave` routes configuration saves to its
own handler; `Send` rejects configuration and Ctrl/Cmd+Enter has no request handler
there. No send or history entry is created by configuration shortcuts.

The strip retains reordering, overflow selection, Ctrl+Tab, close others, close
right, close saved, and reopen behavior across mixed tab kinds. Duplicate on a
configuration focuses its existing target tab; Copy URL is disabled. Close saved
includes configurations associated with collections and confirms dirty ones.
Search indexes configuration titles as tab results without a request payload or
auth/variable values. Titles use saved target names, never authentication fields.

Preview requires an active HTTP request. Selecting a configuration clears any
request preview and keeps the configuration visible. Existing keyed sends remain
declared in the shell, and the configuration workbench declares the existing
request-download helper so an already authorized response download completes
while configuration is active. Other pages retain their baseline download behavior.

Coordinator-approved ownership expansion: only
`internal/gui/import_expansion_regression_test.go`, in
`TestImportedAuthSendPreservesHierarchy` (the dispatch called this the real-store
HTTP test), was added to the original owned list. Its `editParent` helper and
later query-auth save now assert a clean configuration tab and explicitly
reactivate the retained live request before sending. This reflects the intentional
active-tab behavior after Save. All existing HTTP/auth/store assertions remain;
no importer production code changed.

## Files

- `internal/gui/collection_settings.go`: target drafts, sections, save/reload and UI.
- `internal/gui/app.go`: minimal tab kind/target, settings workbench, send guard,
  state cleanup and continued authorized downloads.
- `internal/gui/collections.go`: settings/section menu entries, unified save route,
  removal of auth and variables editing modals.
- `internal/gui/tabs.go`: mixed icon/title, open/duplicate/close/reopen behavior.
- `internal/gui/search.go`, `internal/gui/preview.go`: mixed-tab indexing and preview
  privacy guard.
- `internal/i18n/collections.go`: Chinese and English configuration copy.
- `internal/gui/collection_settings_test.go` and
  `internal/i18n/collection_settings_test.go`: new behavior and localization coverage.
- `internal/gui/auth_test.go`, `internal/gui/auth_benchmark_test.go`,
  `internal/gui/collections_test.go`,
  `internal/gui/import_expansion_regression_test.go`: necessary adaptations from
  modal behavior to persistent configuration tabs.
- `docs/collection-settings-tabs.md`: behavior, verification and integration notes.

## Tests and verification

New tests:

- `TestCollectionSettingsMenuSectionsReuseTabAndRetainDraft`
- `TestCollectionSettingsUnifiedSaveFreshTreeAndInheritance`
- `TestCollectionSettingsDiscardReloadAndDirtyCloseReopen`
- `TestCollectionSettingsFolderIndependentDraftAndLatestFields`
- `TestCollectionSettingsNoSendAndRequestOpenPathsKeepDraft`
- `TestCollectionSettingsMixedTabsSearchDuplicateReorderAndClose`
- `TestCollectionSettingsMixedOverflowCloseRightAndReopen`
- `TestCollectionSettingsDeletedTargetErrorAndSafeTitles`
- `TestCollectionSettingsClearsPreviewAndCompletesInflightDownload`
- `TestCollectionSettingsLanguagesThemesAndMinimumLayout`
- `TestCollectionSettingsTranslationsComplete`

Adapted auth tests retain their masking, inheritance, latest-tree preservation,
failure/retry, changed-target and layout assertions as
`TestAuthSettingsDiscardEscapeOutsideAndLatestTreeSave`,
`TestAuthSettingsFailureRetryAndChangedTarget`, and
`TestAuthSettingsAndEditorLanguagesThemesLayout`. The shared configuration editor
benchmark is now `BenchmarkAuthSettingsPaint`; request auth benchmarks remain.
The existing collection variables save/discard/export test still checks persisted
values and exported content. All other request, auth, preview, import, search and
tab tests ran in the complete suite.

Tests use the existing headless harness, in-memory services and a temporary real
store; existing HTTP regressions use local httptest servers. The layout test covers
zh/en, light/dark and 960×600 / 1440×900 with accessible configuration controls and
layout checks. No real window or external network is used by tests.

Required checks run on 2026-11-17, all exit code 0. Actual output tails:

```text
gofmt -l .
(empty)

go vet ./...
(empty)

go tool viva vet ./...
(empty)

go test ./... -count=1
ok   github.com/runbrick/reva/internal/gui              18.453s
ok   github.com/runbrick/reva/internal/i18n              1.768s
ok   github.com/runbrick/reva/internal/importer          6.063s
ok   github.com/runbrick/reva/internal/model             1.478s
ok   github.com/runbrick/reva/internal/preview           2.904s
ok   github.com/runbrick/reva/internal/runner            2.985s
ok   github.com/runbrick/reva/internal/s3                2.583s
ok   github.com/runbrick/reva/internal/script            2.574s
?    github.com/runbrick/reva/internal/service           [no test files]
?    github.com/runbrick/reva/internal/service/fake      [no test files]
ok   github.com/runbrick/reva/internal/service/request   3.943s
ok   github.com/runbrick/reva/internal/service/store     3.732s
ok   github.com/runbrick/reva/internal/storage           1.198s
ok   github.com/runbrick/reva/internal/syncing           4.194s
ok   github.com/runbrick/reva/internal/update            2.711s
ok   github.com/runbrick/reva/internal/vars              1.628s
ok   github.com/runbrick/reva/internal/websocket         2.081s
```

The full suite initially exposed a remaining query-auth save in the import HTTP
regression that still assumed automatic return to the request. It was adapted
within the approved test scope and the full suite then passed. `git diff --check`
also produced no output.

The adapted benchmark setup was also executed with
`go test ./internal/gui -run '^$' -bench '^BenchmarkAuthSettingsPaint$' -benchtime=1x`;
all nine root/folder auth cases completed, ending with:

```text
PASS
ok   github.com/runbrick/reva/internal/gui 0.344s
```

Scope limits: drafts are session UI state and do not survive application restart.
The service has no atomic compare-and-swap API, so saves merge a fresh service
snapshot but do not implement distributed conflict resolution. Configuration
layout was verified headlessly rather than by opening a native window. Existing
creation, deletion confirmation and request-save destination dialogs remain.
No model/service/shared harness or other unowned production files need changes.

## Review follow-up: preview privacy across mixed tab kinds

The initial implementation commit is
`74743f6fb395df18c0421d525e327b567cd715cf`. Review found that
`requestTabTitle` returned configuration titles before checking request-preview
privacy. An inactive collection/folder tab could therefore expose a saved target
name containing a current secret while an HTTP preview masked the rest of the
window. The follow-up moves the preview guard before kind-specific title
selection. Existing request-kind guards remain unchanged.

`TestCollectionSettingsPreviewMasksRootFolderAndOverflowTitles` uses root and
folder names containing a short current collection variable/token canary and
independent unsaved drafts. With the request preview active it checks every
painted label, accessible hit name/value, all tab title results, and actual
overflow menu entries. An already-open search palette remains suppressed by the
shell's existing preview gate. Closing preview restores the original configuration
titles and retains both draft objects and their unsaved values.

The new regression first failed against the original implementation:

```text
--- FAIL: TestCollectionSettingsPreviewMasksRootFolderAndOverflowTitles (0.01s)
    collection_settings_test.go:485: preview painted a configuration-name secret
FAIL
FAIL github.com/runbrick/reva/internal/gui 0.345s
```

It passed after the fix. All four required checks were then rerun, each exiting 0:

```text
gofmt -l .
(empty)

go vet ./...
(empty)

go tool viva vet ./...
(empty)

go test ./... -count=1
ok   github.com/runbrick/reva/internal/gui             18.075s
?    github.com/runbrick/reva/internal/gui/icons       [no test files]
ok   github.com/runbrick/reva/internal/i18n            0.613s
ok   github.com/runbrick/reva/internal/importer        6.026s
ok   github.com/runbrick/reva/internal/model           0.561s
ok   github.com/runbrick/reva/internal/preview         2.364s
ok   github.com/runbrick/reva/internal/runner          2.483s
ok   github.com/runbrick/reva/internal/s3              2.922s
ok   github.com/runbrick/reva/internal/script          1.075s
?    github.com/runbrick/reva/internal/service         [no test files]
?    github.com/runbrick/reva/internal/service/fake    [no test files]
ok   github.com/runbrick/reva/internal/service/request 3.879s
ok   github.com/runbrick/reva/internal/service/store   2.208s
ok   github.com/runbrick/reva/internal/storage         0.595s
ok   github.com/runbrick/reva/internal/syncing         4.410s
ok   github.com/runbrick/reva/internal/update          2.963s
ok   github.com/runbrick/reva/internal/vars            1.519s
ok   github.com/runbrick/reva/internal/websocket       2.623s
```

Follow-up changes are confined to `internal/gui/tabs.go`,
`internal/gui/collection_settings_test.go`, and this report. No remaining blocker
or ownership expansion is required. `git diff --check` also passed.
