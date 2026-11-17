# 首批调研功能交付与集成结果

R1–R4 与独立 QA 已通过五个 Orca Codex 子工作树实现并合入当前分支
`runbrick/onboarding-orchestration`。实施前先提交了
[功能归并与任务分工](implementation-plan.zh-CN.md)，依赖关系为 R1 → R2 → R3，
R4 独立并行，最后由 QA 验证完整组合。Orca Run：`run_bc4ce37e6778`。

## 独立 PR 范围与提交

这些是本地可独立审查的 PR 单元，保留原始分支与提交历史，没有创建或推送远程 PR。
用户授权的集成目标是当前分支；没有合并或修改 main。

| PR | 子分支 / 工作树 | 功能提交 | 交付说明与所有文件、测试明细 |
| --- | --- | --- | --- |
| R1 | `runbrick/research-send-pipeline` / `research-send-pipeline` | `d4057db321280d325fce06d0c5b112955d61f0d3` | [共用发送与脚本管线](pr-r1-execution.md) |
| R2 | `runbrick/research-collection-runner` / `research-collection-runner` | `e707ee04e8d0450d9cc2f16d180874c6c05f885e` | [确定性集合串行执行与报告](pr-r2-runner.md) |
| R3 | `runbrick/research-runner-cli` / `research-runner-cli` | `ade01fbdf3e901f4f9ac4bade1f8137ad200376f`；帮助本地化 `5c046c232f8ec08e62b85c0ed71188f726e47b6e` | [无窗口 CLI](pr-r3-cli.md) |
| R4 | `runbrick/research-request-preview` / `research-request-preview` | `e3836ac1477c691662b10093a718ff27f753c4fd`；下载兼容 `ee19ba0adae58b91a3f6abec3e6615439dd066fa`；查询一致性 `eb1a21d8af46fce1087f5250a235df0c452ea23d`；背景隐私 `73e6d765c5ea57405f814f6c3dd931ebcacb3fef` | [变量来源与发送前只读预览](pr-r4-preview.md) |
| QA | `runbrick/research-runner-regressions` / `research-runner-regressions` | `0b12b9215729807580771fb2298585b906edd970` | [五项独立跨模块回归、缺陷复现与修复验收](pr-qa-regressions.md) |

集成使用 merge 保留原提交和依赖祖先，没有重复 cherry-pick。最后功能/QA 集成提交为
`c01d4ab`；本文件所在提交仅补交付文档。所有子工作树均保留，可继续独立查看。

onboarding 的 setup state、settings UI、copy、tests 已通过 PR #12 存在于实施基线，
本轮没有重复实现。[原拆分记录](../onboarding-prs.md)保留不变。

## 可使用的行为

- 共用执行管线按认证继承、选项快照、前置脚本、单次变量展开、认证校验、发送和响应测试
  执行；前置脚本失败阻止发送。GUI 脚本通过 Viva Effect 在 UI 线程外运行，历史保留原始
  发送时草稿与未展开的有效认证。
- 原生集合按原始树顺序深度优先串行执行，默认失败后继续，可选择失败即停；取消阻止
  后续请求。每个请求使用同一运行快照，脚本变量仅在当前请求内生效；runner 不写历史、
  设置、集合或环境。
- 独立 `reva-run` 命令只读加载明确选择的集合和可选环境；不自动选择 GUI 的活动环境。
  有界输入校验、原生版本检查、稳定退出码和报告版本 1 已实现，依赖闭包不含 Viva/GUI。
- 请求预览按当前草稿与已保存环境实时显示变量作用域、胜出/覆盖/禁用状态、未知变量及
  认证来源。所有变量值、查询值、请求头值、请求体与上传路径默认隐藏；不运行脚本、
  不发送、不保存、不读取上传文件。背景侧栏、响应、脚本日志等在预览时隐藏，环境名称
  与标签标题同步遮盖；关闭后恢复。已明确授权的后台发送和下载仍可完成。

```powershell
go run ./cmd/reva-run --help
go build -o bin/reva-run.exe ./cmd/reva-run
./bin/reva-run.exe --data-dir C:/path/to/Reva --collection <collection-id> --environment <environment-id> --stop-on-failure
```

实际执行时用真实 ID 替换尖括号占位符，省略 `--environment` 即不选择环境。
退出码为 0（通过或帮助）、1（运行/断言/输出失败）、2（参数/加载错误）、130（取消）。
需要读取实际程序退出码时使用编译后的程序，`go run` 可能包装非零退出状态。

## 审查与 QA 修复

审查修复了预览提前返回导致已有下载停止声明 Effect 的问题，并验证只完成先前已授权的
下载，不重复发送或写入。CLI 帮助已移至独立中英文本地化表。

独立 QA 额外发现并复现两类跨模块缺陷，均交还功能所有者修复：

1. 首次打开时将 Params 写入 URL，可能丢失 URL 独有的查询项；提前序列化还会改变
   变量展开后的相等比较、重复参数与 `&` 分隔符语义。现在初始加载保留两份原始表示，
   10 个 GUI/runner 对照场景通过，存储字节不变。用户实际编辑 URL 或表格后仍按既有
   规则同步；初始表格只显示原生 Params，URL 独有参数留在 URL 中。
2. 预览模态已遮盖内容，但顶部环境名称、集合侧栏和先前响应/日志仍可能在背景快照中
   显示已知秘密。现在预览期间统一保护这些背景显示。QA 保留短凭据样例以避免文字
   截断造成假通过，并验证关闭后恢复原始显示和状态。

不存在未解决的本轮缺陷或跨所有权修改请求。功能与 QA 的测试名称、文件列表及各自
完整检查输出均在上表链接的 PR 文档内。

## 最终当前分支验证

在合入全部功能、后续修复和独立 QA 后，协调者于当前工作树实际执行以下检查。
宿主为 `go version go1.26.3 windows/amd64`。四项全部退出 0；检查后只更新 Markdown
交付记录，没有 Go 代码变更。

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
?    github.com/runbrick/reva/cmd/reva [no test files]
ok   github.com/runbrick/reva/cmd/reva-run 9.134s
ok   github.com/runbrick/reva/internal/auth 1.482s
?    github.com/runbrick/reva/internal/cloudsync [no test files]
ok   github.com/runbrick/reva/internal/execution 4.484s
ok   github.com/runbrick/reva/internal/format 0.870s
ok   github.com/runbrick/reva/internal/gist 2.874s
ok   github.com/runbrick/reva/internal/gui 17.418s
?    github.com/runbrick/reva/internal/gui/icons [no test files]
ok   github.com/runbrick/reva/internal/i18n 0.707s
ok   github.com/runbrick/reva/internal/importer 5.129s
ok   github.com/runbrick/reva/internal/model 0.718s
ok   github.com/runbrick/reva/internal/preview 1.404s
ok   github.com/runbrick/reva/internal/runner 2.572s
ok   github.com/runbrick/reva/internal/s3 1.438s
ok   github.com/runbrick/reva/internal/script 1.101s
?    github.com/runbrick/reva/internal/service [no test files]
?    github.com/runbrick/reva/internal/service/fake [no test files]
ok   github.com/runbrick/reva/internal/service/request 3.251s
ok   github.com/runbrick/reva/internal/service/store 2.575s
ok   github.com/runbrick/reva/internal/storage 0.680s
ok   github.com/runbrick/reva/internal/syncing 3.461s
ok   github.com/runbrick/reva/internal/update 3.683s
ok   github.com/runbrick/reva/internal/vars 0.897s
ok   github.com/runbrick/reva/internal/websocket 2.261s
Exit: 0
```

GUI 验证使用现有 headless Driver 的实际绘制文字、可访问命中区域、边界和交互，包含
中英文/明暗主题的最小窗口测试；未进行原生窗口或像素截图验收，未在 macOS 执行。
取消在生产 CLI 入口的真实 HTTP 上验证，未向 Windows 子进程注入操作系统 Ctrl+C。

## 交付边界与后续任务

报告有意保留序号、状态、时间与计数，省略请求 ID/名称、URL、凭据、请求/响应正文、
脚本日志和任意错误文字；详细内存执行结果仍可能含秘密，不能直接格式化输出。
HTTP 4xx/5xx 本身不算断言失败，需要脚本显式要求状态码。脚本仍使用已有有界 ES5
沙箱，取消运行中的脚本最多等待当前两秒上限。执行可信集合仍会按启用上传引用读取文件。

变量预览在前置脚本之前计算，主机和路径变量也遮盖，不宣称完整线上报文。
没有新增依赖、图标、持久化 schema 或共享 service/model 接口。

本轮实现范围是首批 R1–R4；R5 秘密/Cookie 会话、R6 SSE、R7 专业工具导出、R8
数据驱动/OAuth2 及契约/gRPC 等后续项已在实施计划中整理依赖和验收条件，尚未实现。
原研究 Word 文档保留为调研时点产物；新的实现与验证记录以这些 Markdown 为准。
