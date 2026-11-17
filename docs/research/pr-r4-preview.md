# R4：变量来源与发送前只读预览

分支：`runbrick/research-request-preview`。这是独立本地功能提交的 PR 说明和验收记录；未推送、未创建远程 PR、未向父分支或 main 合并。经协调者明确授权，将父分支当前基线合入本子分支以适配最新执行管线；依赖合并与功能修正分开提交。

## PR 描述

请求编辑器新增 **请求预览 / Request preview** 操作。在发送前置脚本运行之前，按当前请求草稿、已保存集合和当前活动环境生成只读预览，展示变量来源、覆盖关系、禁用回退、未知变量、脱敏 URL 和有效认证来源。切换活动环境、修改草稿或修改保存的父级认证后，打开的预览在下一次绘制时重新计算。

变量表保留各作用域的原始行号，区分集合 / 环境和生效 / 被后面的启用行覆盖 / 禁用。顺序与 `vars.Lookup` 一致：最后一个启用行生效；禁用环境行不遮蔽集合行。未知变量名称单独显示，包括 URL、启用的查询参数及请求头、当前请求体模式和有效认证中使用的名称。

预览复用 `vars.Apply`、`vars.Expand`、`vars.Unresolved` 和 `auth.Resolve`，认证查询参数使用 `auth.ApplyQuery`。URL 与 Params 在实际变量展开后按发送服务的多重集合规则合并，既不重复编辑器同步的查询行，也保留用户有意重复的参数。查询 API key 替换所有同名值一次，显式请求头（包括空值）优先于自动认证。认证只显示类型和来源作用域，不显示父级名称、ID 或认证字段。

## 隐私及行为边界

- **所有变量值隐藏**，不按变量名称猜测敏感性；禁用变量值也不进入结果。不存在“显示秘密”操作，也不在 UI 状态中缓存解析后的快照。
- 变量用于主机、基础 URL 或路径时同样替换为 `[masked]`。可见 URL 是字面路由结构及隐藏标记，不能据此读取展开后的实际环境目标。环境名称和每行来源 / 胜出状态用于确认当前作用域；这项保守选择明确写入中英文界面。
- 查询参数值全部隐藏，包括通常非敏感的值。用户直接填写的参数名称可见；变量生成的名称隐藏。由变量值注入的额外参数名称也隐藏，防止通过 `&secret=value` 泄露部分变量内容。
- 请求头值全部隐藏，包括 Authorization、Proxy-Authorization、Cookie；Basic 用户名和密码、Bearer、API-key 配置值均不进入显示结果。URL 用户信息和片段全部隐藏。已知变量 / 认证值在可见路由和元数据中出现的字面或转义副本也隐藏；因此某些普通文字可能被保守遮盖。
- 请求体只显示模式。文本、表单内容、文件路径及文件内容均不展示，不打开上传文件。不执行脚本、不发送请求、不保存集合、环境、设置或历史。
- 上述只读保证描述的是预览自身。用户此前已经明确发起的独立工作可以继续完成，包括“发送并下载”已经开始的网络请求、历史记录及下载文件写入；打开预览不会暂停或取消这些工作，也不会额外发起发送或写入。
- URL 解析失败只返回安全状态，不把包含 URL 或认证信息的解析错误拼入界面。
- 这是**前置脚本之前**的说明性预览，不是精确线上报文。脚本结果、网络库自动生成的请求头、请求体编码、上传文件内容和重定向结果不计算。中英文界面明确说明这些限制。
- WebSocket 编辑器不提供 HTTP 请求预览。切到其他请求或直接切到 WebSocket 时清除预览打开状态。

## 窗口快照与文件所有权扩展

Viva 的窗口快照包含模态窗口下方的文字和命中区域名称。请求编辑区在预览期间只声明脱敏视图，避免原始 URL 输入框继续进入快照；不为此修改草稿。

原有标签栏把完整 URL 用作标签标题和可访问名称，所以仅遮住编辑区仍不足以保护默认快照。协调者在问题 `msg_066ebf11ce66` 的回复中明确批准扩展 `internal/gui/tabs.go` 所有权，**仅用于预览打开期间的标题遮盖**。现有 `requestTabTitle` 在预览打开时对所有标签统一返回本地化预览标题，覆盖绘制、可访问名称及溢出菜单；关闭预览立即恢复原始标题。标签 ID、脏标记、关闭和切换行为保持原有实现，并有行为测试覆盖。

原始授权范围：

- 新增 `internal/preview/preview.go`、`internal/preview/preview_test.go`。
- 新增 `internal/gui/preview.go`、`internal/gui/preview_test.go`。
- 修改 `internal/gui/request.go` 和 `internal/i18n/request.go`。
- 新增 `internal/i18n/preview_test.go` 和本文档。

后续 QA 发现环境标题、集合 / 请求名称及先前响应 / 脚本输出可能重复已知秘密。协调者明确扩展 `internal/gui/env.go` 所有权，仅用于预览期间的标题栏 / 菜单显示脱敏；扩展 `internal/gui/app.go` 所有权，仅用于预览期间的背景渲染门控。未修改 scripts.go、auth.go、model、service 或共享 harness。复用现有 `icons.Eye`，无新增图标、依赖或 schema 变更。缺失的 TASK.md / 本地 Viva skill 按调度规格处理；Modal、Scroll、Driver API 来自真实 Viva v0.6.0 模块缓存源码。

## 行为测试

库测试：

- `TestPreviewLastEnabledWinsAndDisabledEnvironmentFallsBack`：同作用域重复行、环境覆盖、禁用回退及未知变量。
- `TestPreviewMasksCredentialsAndAllVariableValuesWithoutMutation`：Basic、Bearer、API-key 及 URL / 请求头 / 元数据脱敏，输入不变。
- `TestPreviewAuthInheritanceAndExplicitHeaderPrecedence`：最近文件夹、集合继承、None 屏障、缺失上下文、空显式请求头优先。
- `TestPreviewQueryMultiplicityMatchesHTTPService`：本地 httptest 与真实发送服务比较查询项数，覆盖镜像行、重复值、禁用行、展开后的相等比较和查询认证替换。只有这项测试显式发送到它自己的本地服务器；预览函数无网络入口。
- `TestPreviewUnknownNamesAcrossActiveInputs`：只扫描当前有效输入，脚本及不活动请求体字段不参与。
- `TestPreviewInjectedQueryAndInvalidErrorsDoNotRevealSecrets`：变量注入查询名称及错误 URL 不泄密。
- `TestPreviewAllBodyModesHidePayloadAndFileReferences`：所有请求体模式均隐藏内容与文件引用。

GUI / 本地化测试：

- `TestPreviewReadOnlyAndMaskedInPaintedSnapshot`：请求 fake 在发送时令测试失败；保存集合 / 请求 / 环境 / 设置 / 历史的入口均安装失败保护；抛错前置脚本未执行，上传路径未读取，临时数据目录和编辑草稿不变。滚动检查实际绘制标签及可访问名称均不含秘密。
- `TestPreviewRefreshesEnvironmentDraftAndAuthContext`：打开期间刷新活动环境、重复行胜出、禁用回退、URL / Params 合并以及丢失继承上下文。
- `TestPreviewMasksAllTabNamesAndRestoresTitlesOnClose`：其他标签的名称 / URL 同样隐藏，关闭后恢复；正常切换、关闭及脏状态保留。
- `TestPreviewLayoutsChineseEnglishLightDarkAtMinimumWindow`：960×600 下中英、明暗四组合均可打开，模态关闭操作完整处于窗口内。验证使用现有 headless Driver 的文字和控件边界，不是原生窗口截图或像素级设计验收。
- `TestPreviewIsScopedToHTTPAndClosesOnTabChange`：HTTP 与 WebSocket 之间的状态和操作边界。
- `TestPreviewAllowsPreviouslyAuthorizedDownloadToComplete`：通过真实“发送并下载”菜单及 headless 保存文件对话框授权一次下载；分别在响应尚未返回和写入 Effect 已运行时打开预览，保持模态打开直到文件写入及结果消费完成。使用可控释放的发送 fake 与实际临时文件写入，验证 Effect 不被取消或重启、没有新增发送或写入、草稿和脏状态不变；无先前下载时打开预览仍不发起工作。
- `TestPreviewHidesSensitiveBackgroundAndRestoresOnClose`：在真实 headless 快照的文字及命中标签中检查环境标题 / 所有菜单候选、集合 / 请求名称、既有响应或脚本错误 / 断言 / 控制台、搜索和 toast 的秘密副本。打开时全部脱敏或隐藏，关闭后恢复原始背景；独立菜单 / 搜索状态、草稿、脏状态及保存数据不被修改，发送和保存入口有失败保护。
- `TestPreviewTranslationsPresentInBothLanguages`：新增 Request 预览文案完整，明确前置脚本时间点及隐藏标记解释。

### 下载声明顺序修正

对初始提交 `e3836ac1477c691662b10093a718ff27f753c4fd` 的复审发现，预览提前返回位于原有 `requestDownloads(c)` 之前，会阻止待返回响应进入下载写入，并停止声明已经运行的写入 Effect。新回归在修正前的两个子用例均失败（分别超时等待写入开始、下载完成），修正后均通过。

修正只将原有 `requestDownloads(c)` 调用移动到预览判断之前，仍使用原来的 `c`、同一个 EffectState 和同一个任务 key，保证 UI Effect 的作用域与身份不变。没有扩大文件所有权，未修改发送管线或模态实现。

### 初次打开保留 URL / Params 原始语义

初次打开请求不再解析 URL 来填充 Params，也不把已有 Params 写回 URL。两种原始表示都完整保留，包括 URL 独有参数、重复值、禁用行、空名称、占位符和 Description；不展开秘密，不标记脏状态，不修改保存的集合。表格展示保存的 Params，URL 独有的查询仍显示在 URL 中；用户明确编辑 URL 后由 URL 更新表格，明确编辑或删除表格行后由表格重建 URL，原有编辑权威规则不变。

单次原始文本合并无法保持任意变量展开后的相等性：`URL ?x={{v}}` 加 `Params x=a` 在 `v=a` 时应只发送一项，原始合并会变成两项；无查询 URL 若提前拼入参数 `{{key}}=a` 且 key 为 `x&y`，会把本应编码的名称误解释为 URL 分隔符。反向解析 URL 也不安全：URL 中变量值 `a&y=b` 与 Params 中相同变量有不同编码边界，百分号编码的 `{{v}}` 在原 URL 中甚至不应展开。因此采用保留原始表示，而非在加载时猜测变量值或尝试归一化。

协调者明确批准新增 `internal/gui/request_load_test.go`，并仅为纠正原有初始物化假设而扩展 `internal/gui/request_test.go` 所有权。新增 `TestRequestLoadPreservesAuthoredURLParamsAndStoredMetadata`、`TestRequestLoadPreservesExpandedHTTPQuerySemantics` 和 `TestRequestLoadKeepsLaterTableEditsAndDeletionsAuthoritative`；真实本地 HTTP 服务验证 URL 独有项、重复项、展开后相等、参数键 / 值中的分隔符、URL 自身变量分隔符和已转义占位符。修正前这些边界均复现原始 URL 改写或实际 HTTP 参数漂移，修正后通过。

该修正独立提交为 `eb1a21d8af46fce1087f5250a235df0c452ea23d`。合入最新基线后的全量测试另外发现 `TestImportCurlOpensRequest` 的中英文断言也依赖初次绘制改写 URL；协调者在 `msg_951cd264b544` 中明确授权仅修改 `internal/gui/importer_test.go` 的这项初始加载断言。现在检查导入器原本生成的 URL `https://example.test/users` 和完整 Params 行 `q=active`，保留方法、请求体、请求头和未保存草稿的原有检查，无导入器生产改动。

### 预览期间隐藏敏感背景

窗口快照仍可包含模态底层的数据，因此预览期间不声明侧栏集合 / 请求列表、响应 / 脚本诊断内容、其他原始数据对话框、搜索结果及 toast。保留原有布局容器、请求编辑器及下载声明的作用域，保留 Page 中发送 / WebSocket Effect 的原始声明位置；没有修改执行管线。环境标题及菜单候选使用 `preview.Build` 的同一脱敏规则：候选环境值与活动环境值均进入只读副本，仍保持活动环境的展开优先级；不复制秘密名称判定规则，不修改保存数据。未知变量提示也使用脱敏快照中的名称。

关闭预览恢复原始内容与正常名称；切换标签、WebSocket、环境管理或其他主页面时清除预览打开状态，避免对已离开的请求视图继续宣称预览保护。先前独立操作的状态不因背景隐藏而清空，发送 / WebSocket / 下载 Effects 保持原有作用域和身份；下载回归在背景门控后仍通过。此保护范围是预览打开期间的界面声明，关闭后恢复普通编辑器的原始数据展示，不承诺普通编辑模式隐藏秘密。

新背景回归在修正前的响应和脚本诊断两种情形均失败，快照可见上述秘密副本；修正后均通过。它也覆盖非活动环境菜单名称中同时出现活动与候选环境秘密的情形，以及关闭后的内容恢复。

## 四项必需检查

在本子工作树最终 Go 变更上执行，全部退出 0：

```text
> gofmt -l .
(empty)

> go vet ./...
(empty)

> go tool viva vet ./...
(empty)

> go test ./... -count=1
ok   github.com/runbrick/reva/cmd/reva-run                6.431s
ok   github.com/runbrick/reva/internal/execution         4.482s
ok   github.com/runbrick/reva/internal/gui              15.227s
ok   github.com/runbrick/reva/internal/i18n              0.836s
ok   github.com/runbrick/reva/internal/preview           2.776s
ok   github.com/runbrick/reva/internal/runner            2.407s
...
ok   github.com/runbrick/reva/internal/service/request   3.842s
ok   github.com/runbrick/reva/internal/service/store     2.761s
ok   github.com/runbrick/reva/internal/storage           0.876s
ok   github.com/runbrick/reva/internal/syncing           3.917s
ok   github.com/runbrick/reva/internal/update            2.806s
ok   github.com/runbrick/reva/internal/vars              0.803s
ok   github.com/runbrick/reva/internal/websocket         2.467s
```

`git diff --check` 无输出。完整套件还覆盖原有响应 HTML 预览，新的操作名使用 Request preview / 请求预览，避免与响应 Preview / 预览冲突。

## 父分支集成

本功能保持 `request.go` 原有 `sendJob.req` / `sendJob.vars` 使用方式。协调者明确授权将 `runbrick/onboarding-orchestration` 基线 `3fd1643` 合入本子分支，依赖合并提交为 `f9ee05962189f7b3f1a6ab4f34bbc806e901aad4`；预览背景适配在该基线之上单独提交，没有向父分支或 main 合并。当前无额外文件适配请求，最终父分支集成由协调者执行。实际目标值的隐藏、无 reveal、无线上报文精确性承诺为本 PR 的有意边界；初次加载的 Params 表仅展示原本保存的行，合并后的发送查询由发送服务及预览计算，用户明确编辑后仍按原来的编辑权威规则同步。
