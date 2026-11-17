# R4：变量来源与发送前只读预览

分支：`runbrick/research-request-preview`。这是独立本地功能提交的 PR 说明和验收记录；未推送、未创建远程 PR、未合并父分支或 main。

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
- URL 解析失败只返回安全状态，不把包含 URL 或认证信息的解析错误拼入界面。
- 这是**前置脚本之前**的说明性预览，不是精确线上报文。脚本结果、网络库自动生成的请求头、请求体编码、上传文件内容和重定向结果不计算。中英文界面明确说明这些限制。
- WebSocket 编辑器不提供 HTTP 请求预览。切到其他请求或直接切到 WebSocket 时清除预览打开状态。

## 窗口快照与文件所有权扩展

Viva 的窗口快照包含模态窗口下方的文字和命中区域名称。请求编辑区在预览期间只声明脱敏视图，避免原始 URL 输入框继续进入快照；不为此修改草稿。

原有标签栏把完整 URL 用作标签标题和可访问名称，所以仅遮住编辑区仍不足以保护默认快照。协调者在问题 `msg_066ebf11ce66` 的回复中明确批准扩展 `internal/gui/tabs.go` 所有权，**仅用于预览打开期间的标题遮盖**。现有 `requestTabTitle` 在预览打开时对所有标签统一返回本地化预览标题，覆盖绘制、可访问名称及溢出菜单；关闭预览立即恢复原始标题。标签 ID、脏标记、关闭和切换行为保持原有实现，并有行为测试覆盖。

其他修改均在原始授权范围：

- 新增 `internal/preview/preview.go`、`internal/preview/preview_test.go`。
- 新增 `internal/gui/preview.go`、`internal/gui/preview_test.go`。
- 修改 `internal/gui/request.go` 和 `internal/i18n/request.go`。
- 新增 `internal/i18n/preview_test.go` 和本文档。

未修改 app.go、scripts.go、env.go、auth.go、model、service 或共享 harness。复用现有 `icons.Eye`，无新增图标、依赖或 schema 变更。缺失的 TASK.md / 本地 Viva skill 按调度规格处理；Modal、Scroll、Driver API 来自真实 Viva v0.6.0 模块缓存源码。

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
- `TestPreviewTranslationsPresentInBothLanguages`：新增 Request 预览文案完整，明确前置脚本时间点及隐藏标记解释。

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
ok   github.com/runbrick/reva/internal/gui              16.958s
ok   github.com/runbrick/reva/internal/i18n              1.180s
ok   github.com/runbrick/reva/internal/preview           3.574s
...
ok   github.com/runbrick/reva/internal/service/request   3.403s
ok   github.com/runbrick/reva/internal/service/store     2.859s
ok   github.com/runbrick/reva/internal/storage           1.103s
ok   github.com/runbrick/reva/internal/syncing           2.682s
ok   github.com/runbrick/reva/internal/update            1.340s
ok   github.com/runbrick/reva/internal/vars              1.132s
ok   github.com/runbrick/reva/internal/websocket         3.300s
```

`git diff --check` 无输出。完整套件还覆盖原有响应 HTML 预览，新的操作名使用 Request preview / 请求预览，避免与响应 Preview / 预览冲突。

## 父分支集成

本功能保持 `request.go` 原有 `sendJob.req` / `sendJob.vars` 使用方式，未引入对 R1 新执行管线的依赖。当前无额外文件适配请求；协调者合入并行 R1 后按总体验收规则再次执行四项检查。实际目标值的隐藏、无 reveal、无线上报文精确性承诺为本 PR 的有意边界。
