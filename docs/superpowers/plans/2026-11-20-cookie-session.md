# Cookie Session Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让桌面用户在登录后连续发送请求，或在一次集合运行中复用登录态；两个入口都默认关闭，可清除会话，Cookie 仅保存在内存。

**Architecture:** 由 GUI 显式选用内存会话，通过请求上下文传给现有 HTTP 服务。HTTP 服务在每次发送的 `http.Client` 副本上挂载标准库 Cookie jar，继续复用现有连接池。手动发送按集合和环境隔离，会话由工作台持有；每次桌面 Runner 单独创建并丢弃会话，复用原有执行与报告流程。

**Tech Stack:** Go 1.26.0、`net/http/cookiejar`、Viva v0.6.0、`net/http/httptest`、现有 GUI headless harness。

**Spec:** 本文的 [首版行为](#首版行为)；背景见 [R5b 调研](../../research/implementation-plan.zh-CN.md)。接线核对基于 `481b84b`，亦检查本地 `v0.6.0` 标签（`43690f0`）；开工时从计划执行者获准的基线建立独立工作树。

## Global Constraints

- 首版覆盖桌面手动调试和单次集合运行，默认关闭、可清空、不落盘。
- 不增加第三方依赖，不改持久化 schema，不改 `service.RequestService`、`runner.Options`、报告 v1 或 `reva-run` 参数。WebSocket、Cookie 编辑器和跨子域共享留待以后。
- GUI 使用 Viva `ui.Effect`、现有主题与间距；所有新增文案写入对应中英文 i18n 段。网络测试只用本地 `httptest`，GUI 测试用现有 headless harness。
- 任务所列文件即建议的实施所有权；`app.go` 是共享文件，Task 2 的发送钩子需在实施前纳入明确所有权。其余共享接口、模型和 harness 不需扩权。实施者在独立工作树分步提交，不推送或合并 main。

## Review Focus

- 同一主机不同端口、HTTPS 降级、跨源后返回：自动 Cookie 不随跳转越界；返回起点也不恢复。Task 1：`TestCookieSessionRedirectIsolation`。
- 手写空 `Cookie`、鉴权生成的 `Cookie` 头、自定义 `Host`：不能叠加自动会话，也不能拿这些响应更新会话。Task 1：`TestCookieSessionExplicitHeadersBypass`。
- 登录响应晚于清空或关闭：旧响应不得给之后的发送重新登录。Task 2：`TestManualCookieSessionClearDuringSend`。
- 服务端在坏重定向的 `Location` 回显 Cookie：实时界面仍可显示错误，磁盘历史只留固定文案。Task 2：`TestManualCookieSessionNeverPersists`。
- 运行取消时请求正在进行：不再发送后续请求；下一次勾选运行也从空会话开始。Task 3：`TestCollectionCookieSessionCancellation`。

## 首版行为

1. **手动调试**：在工作台「请求设置」最上方提供「使用临时 Cookie 会话」和「清空手动 Cookie 会话」。开关适用于当前应用窗口；各会话以 `(集合 ID, 有效环境 ID)` 隔离，独立草稿使用空集合 ID，未选环境使用空环境 ID。同一作用域的标签共享登录态；切回原作用域可继续使用。清空清除该窗口所有手动会话，保留开启状态；关闭也清空，下次开启从空状态开始。已开始的发送及其重定向可使用原会话完成，但从清空后新发起的请求不会拿到旧会话。
2. **单次集合运行**：桌面 Runner 设置中单独勾选「在本次运行中使用 Cookie 会话」。每次打开默认未勾选；仅在开始后创建新会话，贯穿本次串行运行，成功、失败或取消后都丢弃。清空手动会话不影响正在运行的集合，运行也不会取得手动 Cookie。运行期间用现有「取消运行」结束会话；下次从空会话开始。首版不新增 CLI 开关。
3. **HTTP 规则**：会话以 `scheme + hostname + effective port` 隔离。匹配路径、Secure、过期和删除由标准库处理；不跨源共享 `Domain` Cookie，也不模拟浏览器页面上下文的 SameSite 规则。同源重定向可沿用会话；一旦跳到其他源，该链余下跳转暂停 Cookie 读取和保存，即使回到起点也一样。发送启用会话时，最终报文存在 `Cookie` 头（包括空值）或显式 `Host` 头，该次发送及其重定向绕过 jar；跨源后也移除手写 `Cookie`。会话关闭时，现有显式请求头及重定向行为保持原样。
4. **保存规则**：开关与 jar 不写入设置、请求、历史、导出或备份；自动 Cookie 不加入草稿、复制 cURL 或发送前预览。响应的 `Set-Cookie` 仍可在响应面板查看，应用不保存它；会话启用时的传输错误在历史只存固定文案。用户手写或主动复制的 Cookie、显式下载的响应正文，仍沿用现有行为。

---

### Task 1: 给共享 HTTP 发送路径接入会话

**Files:** 新建 `internal/service/request/cookie_session.go`、`internal/service/request/cookie_session_test.go`；修改 `internal/service/request/request.go` 中的 `send`、`protectRedirects` 和 `sameOrigin`。

**Interfaces:** 产出 `type CookieSession struct`、`func NewCookieSession() *CookieSession`、`func WithCookieSession(ctx context.Context, session *CookieSession) context.Context`（`nil` 返回原上下文）。内部 `func (s *CookieSession) jarFor(u *url.URL) *cookiejar.Jar`；`func originOf(u *url.URL) httpOrigin` 返回可比较的 `httpOrigin`，以现有 `sameOrigin` 的大小写与默认端口规则规范化。

- [ ] **写失败测试。** `TestCookieSessionCarryAndExpiry` 用 `httptest.NewTLSServer` 在 `/login` 设置 `sid=session-1; Path=/api; Secure`，通过同一个带会话的上下文和 `Service` 调用 `Send`；`/api/me` 回显收到的 Cookie，`/other` 回显空值。再由 `/logout` 发送同名同路径 `Max-Age=0` 并确认清除。验证无会话、另一新会话和切换 `FollowRedirects` 的行为：前两者在 `/api/me` 无 Cookie，切策略仍可用同一会话（在登出前验证）。核心断言：

  ```go
  assertCookieBody(t, send(ctx, "/api/me"), "sid=session-1")
  assertCookieBody(t, send(ctx, "/other"), "")
  assertCookieBody(t, send(context.Background(), "/api/me"), "")
  assertCookieBody(t, send(WithCookieSession(context.Background(), NewCookieSession()), "/api/me"), "")
  assertCookieBody(t, sendWithNoRedirects(ctx, "/api/me"), "sid=session-1")
  send(ctx, "/logout")
  assertCookieBody(t, send(ctx, "/api/me"), "")
  ```

  `TestCookieSessionRedirectIsolation` 设置两个同主机不同端口的本地服务 A/B；A 302 加 `Set-Cookie: sid=from-a; Path=/` 到同源目标时收到 `sid=from-a`，禁用跳转时返回 `302` 但保留该响应的 Cookie。先在 B 单独写入 `sid=from-b`，再测 A→B→A，记录每跳收到的请求头；另测 TLS→明文和直接访问 B。以相同表驱动测试覆盖手写 Cookie 在启用会话时的跨源剥离。

  ```go
  assertHeaderSequence(t, sameOriginHops, []string{"", "sid=from-a"})
  assertHeaderSequence(t, crossOriginHops, []string{"sid=from-a", "", ""})
  assertHeaderSequence(t, explicitCrossOriginHops, []string{"sid=manual", "", ""})
  assertHeaderSequence(t, tlsDowngradeHops, []string{"sid=from-tls", ""})
  assertCookieBody(t, directB, "sid=from-b") // B 在跨源链上回的 Set-Cookie 未覆盖它。
  ```

  `TestCookieSessionExplicitHeadersBypass` 先存 `sid=jar`，分别发送下列请求到会返回 `sid=ignored` 的本地端点，并断言之后无显式头的探测仍为 `sid=jar`（禁用行测试使用不返回 `Set-Cookie` 的探测端点）。用 `r.Header.Values("Cookie")` 区分缺失与显式空值：

  ```go
  assertHeaderSequence(t, explicitCookieHeaders, []string{"sid=manual"})
  assertHeaderSequence(t, emptyCookieHeaders, []string{""})
  assertHeaderSequence(t, apiKeyCookieHeaders, []string{"sid=api"})
  assertHeaderSequence(t, customHostCookieHeaders, nil)
  assertHeaderSequence(t, disabledCookieRowHeaders, []string{"sid=jar"})
  assertCookieBody(t, followup, "sid=jar")
  ```

- [ ] **确认红灯。** `go test ./internal/service/request -run '^TestCookieSession' -count=1`；预期缺会话接口，或上述线路断言失败。
- [ ] **实现会话和 client 接线。** 会话内用互斥锁保护按 `httpOrigin` 索引的 jar 映射；用 `cookiejar.New(nil)` 创建桶，cookiejar 自己保护其内部状态。桶只供同一 origin 使用，因此不依赖公共后缀列表来阻止跨主机读取。`send` 在变量展开、脚本已有的修改和认证头应用之后检查最终 Header 的键是否存在；只有无 `Cookie`、无 `Host` 才把 `jarFor(httpReq.URL)` 挂到单次 client 副本，禁止修改缓存 client。更新 `func protectRedirects(client *http.Client, cfg model.Auth, cookieSessionEnabled bool)`：既有黏性跨源标记触发时，在启用会话的发送上同时清掉该副本的 `Jar` 和跳转请求的 `Cookie`，现有 Auth、跳转策略及响应解析继续工作。普通发送、SSE 和下载共用该 `send`，不加第二套逻辑。
- [ ] **确认绿灯。** `go test ./internal/service/request -count=1` 退出 0，现有 Auth、TLS、SSE 及重定向用例通过。
- [ ] **提交。** `git add internal/service/request/cookie_session.go internal/service/request/cookie_session_test.go internal/service/request/request.go`；`git commit -m "feat: support isolated in-memory HTTP cookie sessions"`。

### Task 2: 让桌面手动发送开启、清空和共享会话

**Files:** 新建 `internal/gui/cookie_session.go`、`internal/gui/cookie_session_test.go`；修改 `internal/gui/request.go`（`requestState`、`requestSettingsSection`）、`internal/gui/app.go`（`sendJob`、`Send`、`run`、历史错误写入）、`internal/i18n/request.go`。

**Interfaces:** 消费 Task 1 的会话 API。产出 `func (a *App) manualCookieSessionFor(t *Tab) *request.CookieSession`、`func (a *App) requestCookieControls(c ui.C)`；`requestState.cookies manualCookieState` 持有 `enabled bool` 和按 `manualCookieScope{collectionID, environmentID string}` 索引的会话映射，`sendJob.cookieSession *request.CookieSession` 持有发送时捕获的会话。

- [ ] **写失败测试。** `TestManualCookieSessionLifecycleAndScope` 使用 `newHarness` + 真实 `request.New()` + 本地 `/login`、`/me`，用 `200` 表示收到 `sid=manual-secret`，`401` 表示无登录态。点击「请求设置」、开关、发送、清空，覆盖同一作用域新标签、另一集合、另一环境和冷启动；通过现有环境选择逻辑设置两个环境、两集合，目标始终为同一本地服务。对真实探测响应做断言（辅助函数通过 harness 泵帧直到完成）：

  ```go
  assertStatus(t, defaultLoginThenProbe, http.StatusUnauthorized)
  assertStatus(t, enabledSameScopeOtherTab, http.StatusOK)
  assertStatus(t, otherCollection, http.StatusUnauthorized)
  assertStatus(t, otherEnvironment, http.StatusUnauthorized)
  assertStatus(t, backInOriginalScope, http.StatusOK)
  assertStatus(t, afterClear, http.StatusUnauthorized)
  assertStatus(t, afterDisableAndReenable, http.StatusUnauthorized)
  assertStatus(t, afterAppRestart, http.StatusUnauthorized)
  ```

  `TestManualCookieSessionClearDuringSend` 用 channel 让 `/slow-login` 等到用户点击「清空手动 Cookie 会话」才回 `Set-Cookie`。旧请求完成后 `/me` 仍为 `401`；在关闭再开启的子测试中复用同一断言。`TestManualCookieSessionSSE` 让有登录态的 `/events` 发 `data: authenticated\n\n`，断言收到首事件时仍在发送，断开、下一次普通 `/me` 仍按会话判断。

  `TestManualCookieSessionNeverPersists` 使用真实临时存储（沿用 `researchStore`、`researchDisk`）与 Cookie 哨兵。在 `/broken-redirect` 将收到的 `sid=manual-secret` 放入 `Location: /bad%2?echo=sid=manual-secret`，强制 transport 返回错误；界面可见原始错误，历史写固定文案。打开预览、生成现有可复制 cURL 并检查快照；保存前后设置、集合、环境字节一致，仅手动发送的正常历史文件变化。检查 `syncing.Capture` 的未加密序列化数据，避免因密文天然找不到明文造成假通过：

  ```go
  if !strings.Contains(tab.Err, "manual-secret") || newestHistory.Error != h.App.S.Request.CookieSessionHistoryError {
      t.Fatal("live diagnostics or persisted history did not follow the session policy")
  }
  for _, data := range [][]byte{historyJSON, snapshotJSON, []byte(curlCommand), []byte(previewText)} {
      if bytes.Contains(data, []byte("manual-secret")) { t.Fatal("automatic cookie escaped live execution") }
  }
  if tab.Dirty || !maps.Equal(beforeWithoutHistory, afterWithoutHistory) { t.Fatal("session controls changed authored data") }
  ```

- [ ] **确认红灯。** `go test ./internal/gui -run '^TestManualCookieSession' -count=1`；预期开关/清空未接线或会话行为断言失败。
- [ ] **实现设置和发送钩子。** 在请求设置的滚动内容顶部声明 `ui.Checkbox`，用 `.Changed()` 在关闭时丢弃整个映射；清空按钮启用时同样丢弃映射，保持勾选。两项均不修改 `Tab.Dirty`。`manualCookieSessionFor` 复用 `activeEnvironment`（与 `VarsFor` 同规则），在开启时惰性初始化当前作用域；`App.Send` 在创建 job 时捕获指针，`App.run` 在 SSE/普通分支前装入 context。清空后绝不把旧指针写回映射，后台 job 也不访问 GUI 状态。收取发送完成时，若有会话且 `r.Err != nil && r.TransportStarted`，`entry.Error` 用固定 i18n 文案；`t.Err` 保留实时诊断。
- [ ] **填写中英文文案。** `CookieSessionEnabled`：「使用临时 Cookie 会话」/ `Use temporary cookie sessions`；`CookieSessionClear`：「清空手动 Cookie 会话」/ `Clear manual cookie sessions`；`CookieSessionHistoryError`：「请求失败（未保存错误详情）」/ `Request failed (details not saved)`。
- [ ] **确认绿灯。** `go test ./internal/gui -run 'Test(ManualCookieSession|ResponseSSE|RequestSendAndDownload)' -count=1` 退出 0。用 harness 在 `960 × 600` 验证中英文、明暗主题下能看到并操作开关和按钮。
- [ ] **提交。** `git add internal/gui/cookie_session.go internal/gui/cookie_session_test.go internal/gui/request.go internal/gui/app.go internal/i18n/request.go`；`git commit -m "feat: add temporary cookie sessions to the request workbench"`。

### Task 3: 限定桌面 Runner 会话为单次运行

**Files:** 新建 `internal/gui/collection_cookie_session_test.go`；修改 `internal/gui/collection_runner.go`（配置状态、`startCollectionRun`、`collectionRunJob.run`、弹窗）、`internal/i18n/collections.go`、`README.md`。

**Interfaces:** 消费 Task 1 的会话 API 和既有 `func (s runner.Snapshot) RunReport(ctx context.Context, sender service.RequestService, options runner.Options) runner.Report`。新增内部 `useCookieSession bool` 到 `collectionRunnerState` 和 `collectionRunJob`；不改变 Runner 公共接口。

- [ ] **写失败测试。** `TestCollectionCookieSessionOptInAndIsolation` 用真实 HTTP 和现有 `collectionRunOverview` / `collectionRunReport` harness 辅助，跑固定 `/probe-first` → `/login` → `/me`。先给同一集合、同一环境的手动会话写入 `sid=manual-secret`，再打开 Runner；执行未勾选、已勾选、再次勾选、关闭重开未勾选四次。每次用 `/probe-first` 接收的真实请求头证明没有拿到任何 Cookie；另外验证四次运行前后文件字节相同，JSON 报告不含 `runner-cookie-secret`。关闭报告后再手动探测原集合、原环境，收到的仍是 `sid=manual-secret`；该探测正常记录历史，放在文件比较之后。状态码断言直接看报告行，HTTP 401 本身不算运行失败：

  ```go
  assertRunStatuses(t, defaultRun, []int{401, 200, 401})
  assertRunStatuses(t, optedInRun, []int{401, 200, 200})
  assertRunStatuses(t, freshOptedInRun, []int{401, 200, 200})
  assertRunStatuses(t, reopenedDefaultRun, []int{401, 200, 401})
  if !slices.Equal(firstProbeCookieHeaders, []string{"", "", "", ""}) || manualProbeCookieHeader != "sid=manual-secret" {
      t.Fatal("runner and manual sends shared a cookie session")
  }
  if !maps.Equal(before, after) || bytes.Contains(reportJSON, []byte("runner-cookie-secret")) {
      t.Fatal("runner stored or reported an automatic cookie")
  }
  ```

  `TestCollectionCookieSessionCancellation` 首步为 `/probe-first`：先记录探测结果（无 Cookie 为 `401`），同时通过 `Set-Cookie` 建立会话。等到第二步 `/slow` 收到 Cookie，点击现有取消按钮，服务端以 `r.Context().Done()` 结束；第三步 `/me` 未被调用。重新开始已勾选的运行，让 `/slow` 直接返回，首个探测仍收到 `401`。关键断言：

  ```go
  if report.Outcome != runner.Cancelled || report.Counts.Cancelled != 1 || report.Counts.Skipped != 1 || thirdRequestCount != 0 {
      t.Fatal("cancellation did not stop the authenticated run")
  }
  if freshRun.Requests[0].StatusCode != http.StatusUnauthorized { t.Fatal("cancelled run left a cookie behind") }
  ```

- [ ] **确认红灯。** `go test ./internal/gui -run '^TestCollectionCookieSession' -count=1`；预期找不到勾选控件或已勾选的 `/me` 仍收到 `401`。
- [ ] **接入运行配置。** 在现有「遇到失败时停止」之后声明复选框，初始值为 `false`。在 `startCollectionRun` 拍入 job；在 `collectionRunJob.run` 的现有取消上下文下、`RunReport` 前，勾选时创建新的 `CookieSession` 并只绑定到本次调用。无需在报告、页面状态或 job 中保留 jar。沿用现有取消和关闭复位；在 `i18n.Collections.RunUseCookieSession` 设置「在本次运行中使用 Cookie 会话」/ `Use cookie session for this run`。
- [ ] **补充使用说明。** `README.md` 在 Headless collection CLI 之前新增 `Cookie sessions` 小节，记载两个入口、隔离与清空规则、手写头优先、现有 CLI 不启用。保留 CLI 文档对无 Cookie jar 的说明。
- [ ] **确认绿灯和集成。** 先运行 `go test ./internal/gui -run 'TestCollection(CookieSession|Runner)' -count=1`，通过后跑 `gofmt -l .`（无输出）、`go vet ./...`、`go tool viva vet ./...`、`go test ./... -count=1`（全部退出 0）。用已有 Runner headless 尺寸验证方式检查 `960 × 600` 下中英文、明暗主题的复选框和开始、取消按钮可达。汇报新增测试、检查结果及提交哈希。
- [ ] **提交。** `git add internal/gui/collection_cookie_session_test.go internal/gui/collection_runner.go internal/i18n/collections.go README.md`；`git commit -m "feat: opt in to cookies for one desktop collection run"`。
