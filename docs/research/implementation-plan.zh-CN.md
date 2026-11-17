# 调研功能整理与实施分工

实施基线：`1a8ff8a`；汇总目标：`runbrick/onboarding-orchestration`。依据为本目录的
[综合研究](api-tools-comparison.zh-CN.md)、12 份逐项目 Markdown/JSON 和
[研究清单](research-manifest.json)。研究中的功能与议题状态沿用原始证据，
不将开放提案或 beta 能力当作稳定交付，也不将议题样本数量当作需求频率。

## 功能归并

| 能力 | 可借鉴的优势与来源 | Reva 已有基础 | 可实施功能与边界 |
| --- | --- | --- | --- |
| 可复现执行 | Bruno / Insomnia / Hoppscotch 的 GUI、CLI 回归；Hurl 的捕获与断言；Karate / REST Assured 的流程诊断 | HTTP、认证继承、变量、限时脚本、取消 | R1 共用发送语义 → R2 原生集合串行执行 → R3 无窗口入口和版本化报告；先不做并行或完整 Postman 运行时 |
| 变量透明度 | Insomnia 分层环境；Bruno 作用域；HTTPie 离线构造 | 集合与活动环境覆盖、未知变量标记 | R4 显示来源、覆盖与发送前只读预览；不执行脚本，不读上传文件，不发送，不保存；变量值默认遮盖 |
| 会话与秘密 | Bruno / Hoppscotch / Yaak 的秘密隔离；HTTPie / Hurl / REST Assured 的会话 | 本地 JSON、响应 Cookie 查看、可选加密备份 | R5a 内存秘密覆盖、R5b 默认关闭的 Cookie jar 分开评审；现有凭据文件不是 vault |
| 流式调试 | HTTPie 增量输出、Yaak SSE、Hoppscotch 实时协议 | HTTP 时间线、WebSocket、GraphQL 编辑 | R6 有界 SSE 服务与事件界面；认证沿用 HTTP，取消与内存上限必须可验证 |
| 专业工具互通 | Hurl 文本测试；WireMock 状态/故障/匹配；k6 与 JMeter 负载模型；Schemathesis 契约生成 | 请求/响应、导入审核、本地集合 | R7a Hurl、R7b WireMock 静态桩、R7c k6 受限导出分别实现；先兼容性预览再保存；不内嵌压测、模糊测试或复杂 mock 引擎 |
| 迭代与授权 | Insomnia / Hoppscotch / JMeter 数据驱动；Bruno / Yaak OAuth2 | 变量、认证编辑与继承 | R8a CSV/JSON 迭代依赖稳定 runner；R8b 先做 client_credentials，令牌仅内存，拒绝跨源泄露 |
| 契约与多协议 | Schemathesis 失败重放；Hoppscotch GraphQL 内省；Bruno / Yaak gRPC | OpenAPI JSON 导入、GraphQL 编辑、WebSocket | 保留为独立后续设计；导入不等于契约验证，编辑不等于内省；不在本轮改变请求模型 |

## 本轮独立 PR / 子工作树

首批交付 R1–R4，每个 PR 在独立 Orca Codex 子工作树实现并提交。依赖项完成后，
协调者先审查并合入当前汇总分支，再从更新后的分支创建依赖工作树，避免重复 cherry-pick。
所有 worker 不推送、不创建远程 PR、不合并 main；用户已授权最终合入当前分支。

| PR | 工作树名 | 职责及文件所有权 | 依赖 | 用户行为验收 |
| --- | --- | --- | --- | --- |
| R1 | research-send-pipeline | 新 `internal/execution/*`；`internal/gui/app.go`、`scripts.go`、`scripts_test.go`；对应新增 GUI 管线测试；独立 R1 说明 | 无 | GUI 与后续 runner 复用认证/选项/脚本顺序；前置脚本失败不发送；取消、断言与原始历史不泄露展开值 |
| R4 | research-request-preview | 新 `internal/preview/*`、`internal/gui/preview.go` 及测试；`request.go`、`internal/i18n/request.go` 及新增本节测试；独立 R4 说明 | 无，与 R1 并行 | 来源与覆盖可见，禁用行回退，切换环境刷新，默认不显示变量与凭据，预览无网络/脚本/文件读取/写盘副作用 |
| R2 | research-collection-runner | 新 `internal/runner/*`、独立 R2 说明 | R1 | 深度优先按原顺序串行运行；执行变量快照；取消阻止后续；可选失败即停；不保存环境/集合/历史 |
| R3 | research-runner-cli | 新 `cmd/reva-run/*`、`README.md` CLI 用法、独立 R3 说明 | R2 | 无 Viva/GUI 依赖；明确集合/可选环境；稳定退出码；schema 版本；脱敏 JSON，不输出请求、响应体或脚本日志中的凭据 |
| QA | research-runner-regressions | 仅新增跨模块行为测试与独立验收说明 | R1–R4 | 临时目录真实磁盘、httptest、取消和脚本错误、报告脱敏；所有四项检查通过 |

测试随各功能提交，QA PR 补充跨模块回归。不得修改另一 worker 的所有权文件；需要扩展
所有权时向协调者说明。复用现有 model/service 接口，不加依赖或改动持久化 schema。

## 后续任务及验收

- R5a（依赖 R1）：可清空内存覆盖；历史/导出不包含内存秘密，重启消失；先单独确定秘密标记与遮盖策略。
- R5b（依赖 R1）：默认关闭的运行 Cookie jar；两个本地源证明隔离，显式 Cookie 行为明确，清空和新运行重建有效。
- R6（接口设计先行）：SSE 分块 flush 后首事件在结束前可见；慢流可取消；事件数、单条/总字节和背压有上限。
- R7a/b/c（分别独立）：确定支持子集、参数化认证、列出不支持的脚本/body；显式确认保存；不启动外部软件。
- R8a（依赖 R2/R3）：CSV/JSON 行错误可定位，运行局部变量不修改环境；输入与迭代数有上限。
- R8b（依赖会话设计）：本地 token 服务验证取消、到期、跨源跳转；client secret 与 token 不落盘。

这些后续项仅整理和排序，不标为本轮已交付。完整功能需要额外的接口、交互和行为边界，
不与首批共用执行语义的交付捆绑。

## 已有 onboarding 与工程约束

setup state、settings UI、copy、tests 已通过 PR #12 合入，不重复创建。
见 [原拆分证据](../onboarding-prs.md) 和综合研究的 onboarding 节。

当前检出没有 AGENTS.md 所引用的 `TASK.md` 与
`.claude/skills/building-with-viva/SKILL.md`；搜索本仓库及用户技能目录未找到。
任务以本文件及 worker 自包含规格为准，Viva API 以 Go 模块缓存 v0.6.0 的源码、
注释与示例为准，继续遵守 AGENTS.md 的 theme/i18n/Effect/Key/测试规则。

每个 worker 报告分支、commit、文件、行为测试、四项检查尾部与未解决事项。
汇总后因存在跨分支组合，再执行 `gofmt -l .`、`go vet ./...`、
`go tool viva vet ./...`、`go test ./... -count=1`，并记录最终集成结果。
