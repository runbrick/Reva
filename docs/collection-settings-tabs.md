# Collection configuration in workbench tabs

Current layout branch: `runbrick/collection-reference-layout`, based on parent
`runbrick/onboarding-orchestration` at `1e8d4c4`. The earlier implementation and
review checks below describe the original collection settings feature.

Each collection/folder sidebar row has a directly visible gear at its right edge,
and a separate descendant request count. Clicking the gear opens **概览 / Overview**
without a menu or modal, reusing the target's existing settings tab and its draft.
The active configuration target is highlighted; disclosure chevrons still expand
and collapse the tree independently. Counts never modify a saved display name,
including names containing parentheses. Duplicate names get distinct accessible
gear labels containing the display name and target identity.

The collection context menu still starts with **集合设置 / Collection settings**.
Folders offer **文件夹设置 / Folder settings**. Each target opens one configuration
tab alongside request tabs, with a folder icon instead of an HTTP method badge.
The existing Authentication and Collection variables menu shortcuts select
sections in that same tab. Collection roots retain their tree expansion behavior.

Configuration fills the workbench below the tab strip. The strip paints the bare
saved target name beside its folder icon; its accessible name and overflow entry
retain the collection/folder settings distinction. A compact heading holds the
editable name in Overview, an unsaved indicator, Save, and Discard (whose tooltip
explains that it reloads saved data). The plain horizontal Overview / Collection
variables / Authentication labels use `ui.TabBase` with an active blue underline.
Folder settings have Overview and Authentication, including the parent preview; folders
have no variables section. Configurations never show the request URL bar, Send,
response pane or request/response splitter. The existing shared auth editor masks
Bearer tokens, Basic passwords and API-key values, including variable placeholders.
All new copy is in the Collections zh/en string tables and uses existing theme,
spacing and icon assets. No icons or dependencies were added.

## Reference layout

The supplied Reqable screenshot informs the hierarchy and whitespace, with Reva's
existing blue theme retained in light and dark modes. Direct gears replace branch
ellipsis buttons; right-click context menus retain all existing shortcuts. Request
rows retain their ellipsis actions. The sidebar's shell width/resizer is unchanged.

Overview has two equal columns separated by whitespace: API/actions on the left,
and a localized collection guide on the right. Both section headings begin directly
below navigation. Thin rules divide API, organization, and export; no card surfaces
or extra settings toolbar are introduced. The name field has a compact maximum
width and trailing Save/Discard controls remain in the same heading row.

- HTTP counts all saved non-WebSocket request nodes, including GraphQL bodies.
  WebSocket is counted separately. Empty folders never count as requests.
- Collection counts cover the complete persisted tree; folder counts and creation
  actions cover only that subtree. Plus actions create real HTTP/WebSocket requests
  through the existing service and open their editors without sending. New folder
  uses the existing name dialog with the same target.
- Collection export is the existing native Reva JSON save-file flow using current
  saved data, including its credentials. Unsaved settings do not enter exports.
  Folder overview does not offer whole-collection export.
- The guide explains actual collection/folder scope, shared variables, auth
  inheritance, draft editing, save/discard, and export behavior. It is static help,
  never an editable or persisted document. There are no unsupported document
  generators, export formats, request headers, or script sections.
- Wide overviews give each column its own scroll area. When the available content
  width falls below 720 logical pixels, including after resizing the sidebar, one
  scroll area stacks actions before the guide. Heading/navigation remain outside
  that body scroll; 960×600 remains usable. At 1440 and 1800 default widths, API
  actions and the guide appear simultaneously.

New headless behavior/layout tests in `collection_layout_test.go`:

- `TestCollectionOverviewGearCountsReuseAndDisclosure`
- `TestCollectionOverviewFolderActionsAndSavedExport`
- `TestCollectionOverviewDuplicateNamesHaveDistinctGears`
- `TestCollectionOverviewResponsiveGeometry`
- `TestCollectionOverviewSidebarResizeAndIndependentScroll`
- `TestCollectionOverviewHeadlessCapture` (opt-in review PNGs)

The geometry matrix covers zh/en, light/dark, 960×600, 1440×900, and 1800×1000;
it checks compact controls, active underline, columns/stack, and reachable guide
content. Existing settings save/draft/error/close/reopen, masking/preview,
no-send, pending-download, auth, variables and collection behaviors remain covered
by their original tests without assertion removals or test rewrites.

Actual native offscreen PNGs can be reproduced without a window or production
screenshot mode by setting `REVA_COLLECTION_CAPTURE_DIR` to a scratch directory
outside the repository and running:

```text
go test ./internal/gui -run '^TestCollectionOverviewHeadlessCapture$' -count=1 -v
```

This uses Viva 0.6.0 `headless.Open` / `Session.Screenshot` with the actual App
page and platform text engine at scale 1. The rendered zh/light 1800×1000,
en/dark 1440×900, and en/light 960×600 images were inspected during this revision.
No real native window, browser mockup, or external network is used.

Revision verification: all four required checks passed on this child branch.
The existing behavior tests needed no caption or assertion rewrites. Output tails:

```text
gofmt -l .
(empty)

go vet ./...
(empty)

go tool viva vet ./...
(empty)

go test ./... -count=1
ok   github.com/runbrick/reva/internal/gui              20.884s
ok   github.com/runbrick/reva/internal/i18n              1.338s
ok   github.com/runbrick/reva/internal/service/request   5.074s
ok   github.com/runbrick/reva/internal/service/store     4.363s
ok   github.com/runbrick/reva/internal/storage           1.975s
ok   github.com/runbrick/reva/internal/syncing            5.510s
ok   github.com/runbrick/reva/internal/update            3.382s
ok   github.com/runbrick/reva/internal/vars              1.972s
ok   github.com/runbrick/reva/internal/websocket         2.933s
```

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
