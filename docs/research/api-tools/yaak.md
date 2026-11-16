# Yaak 对 Reva 的产品调研

**结论：优先借鉴可分享数据的边界和可复用的本地执行能力，不追求全协议、全插件的复制。** Yaak 是与 Reva 接近的本地优先 API 桌面客户端；gRPC/SSE、OAuth2 生命周期、响应引用模板、目录 Git 协作及秘密字段保护已经形成完整工作流。Reva 则已经有任意前置脚本、响应断言、GraphQL、WebSocket 和加密自托管同步，不能把这些写成待补空白。建议三个独立的小 PR：安全分享导出（P0）、复用现有断言的 HTTP 集合 CLI（P1）、有界 SSE 事件查看（P2）。

## 调研范围与可复核基线

- 规范仓库：[mountain-loop/yaak](https://github.com/mountain-loop/yaak)，分类 `desktop-client`；上游元数据、README、许可证、源码、官方文档、发行说明和功能反馈均实际在线获取。
- 检索完成时间：**2026-11-16T14:33:27Z**；本机观测日期为 **2026-11-16**。这不是预设的知识截止日期。
- Reva 起始 `git rev-parse HEAD`：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；工作分支 `runbrick/research-yaak`。下文 Reva 判断均针对这个快照。
- 观察到的上游 `main`：`b9127dcca4c06c1cc495ca2a3ad78807110b291a`。固定源码链接用于复核；官网文档和反馈页面可能继续变化。
- 本次抓取的最近稳定版是 [`v2026.9.1`](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.1)，最近预发行版是 [`v2026.10.0-beta.2`](https://github.com/mountain-loop/yaak/releases/tag/v2026.10.0-beta.2)，均于 2026-11-01 发布。源码/文档已实现、稳定发行已验证和 beta 变化分别处理；没有安装 Yaak 或测试其二进制。
- 按任务只修改本文件和 [yaak.json](yaak.json)，未安装依赖、未改代码、未新增行为测试、未推送或创建远程 PR。已读 AGENTS.md、README.md、docs/onboarding-prs.md；TASK.md 和指定本地 Viva 技能确实不存在，此任务不涉及 GUI 编码。

## 定位、架构和许可

Yaak 的桌面核心是交互调试与本地工作区：请求、环境、响应历史存入 SQLite，Rust 模块处理 HTTP、gRPC、SSE、WebSocket、模板、同步和 Git，Tauri 加载 React 界面。TypeScript 插件在附带的 Node.js 中扩展认证、模板和菜单动作；CLI 直接使用同一本地数据库，CLI 改动可在桌面呈现。目录同步是 SQLite 之外的协作投影，每资源一份 YAML，并非把主存储简单换成文件夹树。[证据 5–7、22–25]

Reva 的 Go/Viva 原生绘制、无 WebView、JSON 文件主存储已有明确约束；不建议为模仿 Yaak 引入 Tauri、React 或 Node 插件宿主。Yaak 仓库还包括网页与代理组件，本报告以桌面为主；桌面直接向目标服务发请求，网页通过代理的隐私边界不能混用。[证据 5]

**当前根许可证为 SPDX `MIT`，是 OSI 批准的开源许可证。** 官方 README 明确：源码可自行构建并免费用于个人和商业用途；官方预编译二进制按另列的商业使用条款分发，个人免费，工作/商业用途需许可。因此不能称“源码仅非商业可用”，也不能以源码 MIT 推导“官方商业二进制免费”。本次没有发现核心协议/模板等按免费版与付费版锁定的证据；定价主要区分商业使用权、支持、许可证转让、团队 SSO 与用户配置管理，不是云端请求共同编辑能力的证明。[证据 2–4、19–20]

当前页面列个人年费 US$79、个人终身 US$349、Business US$149/人/年，提供 30 天商业试用及符合条件的到期版本继续使用权；价格仅记录检索时状态。个人许可不可转让，Business 可在组织内重新分配。**源码许可与官方分发条款必须分别记录**；这不是对所有附带第三方组件的完整法律审计。[证据 4]

“本地优先”也不等于没有联网：更新、通知、插件目录和许可检查会发送相应运行元数据，部分可关闭；官方说明不上传请求/响应/环境作为分析数据。OAuth 外部浏览器默认的 Hosted Redirect 会经过 `oauth.yaak.app`，可选 Localhost callback。Reva 的本地免账号定位应保留，不必复制许可账号或托管回调。[证据 5]

## 已实现且有辨识度的能力

下面十项有当前文档或源码证据；只在存在发行说明时额外写出稳定交付版本，不把路线图当成功能清单。

| 能力 | 已实现的具体行为与边界 | 一手证据 |
| --- | --- | --- |
| gRPC 工作台 | 服务反射或手选 proto/import 目录，schema 补全和实时错误提示；一元、服务端流、客户端流、双向流均有消息时间线。仅桌面成立，CLI 发送仍不支持。 | [gRPC 文档](https://yaak.app/docs/request-types/grpc-requests)、[CLI 明确拒绝发送](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-cli/yaak-cli/src/commands/request.rs#L619) |
| SSE 渐进展示 | 自动识别 `text/event-stream`，收到响应头就显示状态，逐事件更新内容/大小/耗时，可取消长连接。 | [SSE 文档](https://yaak.app/docs/request-types/server-sent-events) |
| OAuth2 生命周期 | 授权码及 PKCE、客户端凭证等四种流程；自动取令牌及刷新，可在文件夹/工作区继承。JWT/AWS 等还通过内置插件提供。未实测各身份提供方。 | [OAuth2 文档](https://yaak.app/docs/authentication/oauth2)、[插件源码](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/plugins/auth-oauth2/src/index.ts) |
| 响应引用请求链 | 模板从另一请求的响应中以 JSONPath/XPath 提取值；没有响应时可先发送源请求；模板可保存在环境中复用。不是通用前后置脚本。 | [请求链文档](https://yaak.app/docs/templating/request-chaining-scripting)、[脚本能力边界](https://yaak.app/docs/templating/pre-post-request-scripting) |
| 分层变量 | 工作区全局环境、活动子环境、文件夹变量；输入补全和活动环境颜色标记。语法 `${[ NAME ]}` 与 Reva 不兼容。 | [变量文档](https://yaak.app/docs/templating/environments-and-variables) |
| 字段级秘密保护 | 启用后敏感值默认加密，`secure(...)` 可显式保护其他值；XChaCha20-Poly1305 工作区密钥再由系统密钥链主密钥保护。分享环境含明文时提示；不加密整个数据库和响应历史。 | [加密文档](https://yaak.app/docs/collaboration/secrets-encryption)、[安全说明](https://yaak.app/docs/getting-started/security-and-privacy) |
| 目录同步与 Git | `yaak.<id>.yaml` 每资源一文件；应用改动自动落盘、文件传入改动先确认；可视化分支、提交、推拉和差异审阅。环境默认不共享。 | [目录/Git 文档](https://yaak.app/docs/collaboration/local-directory-git-sync) |
| 批量迁移预览 | Postman、Insomnia、OpenAPI/Swagger、curl、Bruno；文件/目录/URL 多来源预览，可导入现有工作区。Bruno 和多文件能力在 v2026.9.0 有稳定交付记录。 | [导入文档](https://yaak.app/docs/collaboration/import-and-export-data)、[发行说明及 PR #706](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.0) |
| 本地 HTTP 批量 CLI | `yaak send` 可发送请求/文件夹/工作区，默认按顺序，支持 `--parallel`、`--fail-fast`、环境选择及发送计数。批量跳过非 HTTP；不是业务断言报告或 CSV 运行器。 | [运行边界文档](https://yaak.app/docs/advanced/running-multiple-requests)、[send.rs](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-cli/yaak-cli/src/commands/send.rs) |
| 可扩展插件 | 认证、导入、动态模板、复制 curl/grpcurl、Send All 等由插件提供；自定义 TypeScript 函数可访问 Node.js。插件暂不能任意渲染自己的界面面板。 | [插件运行时源码](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/packages/plugin-runtime/src/index.ts)、[运行器文档](https://yaak.app/docs/advanced/running-multiple-requests) |

`Send All` 桌面动作是并发发送，不适合依赖严格顺序的业务链；CLI 默认顺序也没有自动赋予任意脚本断言、CSV 迭代和 JUnit 报告能力。CLI 的 `schema` 子命令描述 Yaak 自身模型，gRPC 的校验针对 protobuf 编辑，这两者都不是 OpenAPI 响应契约验证。[证据 8、21–24]

## 与当前 Reva 的直接比较

以下是读过实际模型、服务、发送流程和 UI 后的比较，缺失判断还结合全仓入口及关键词检索；不以 README 是否提及为唯一依据。`internal/service/fake` 是仓库测试替身，不能写成用户 Mock 服务。

| 领域 | Yaak | Reva 基线及源码定位 | 对 Reva 的含义 |
| --- | --- | --- | --- |
| 协议 | 桌面 HTTP/GraphQL/WS/SSE/gRPC；gRPC 四模式及反射。 | 已有 HTTP；`gui/graphql.go` 用 POST+JSON 提供 query/variables 编辑器；`websocket/client.go`、`gui/websocket.go` 为真实 RFC6455 收发。没有 gRPC；`request.Send` 完整读完流才返回，缺少实时 SSE。 | SSE 可小步补齐；gRPC 流序列自动化先接专业工具。 |
| 认证/会话 | OAuth2 生命周期、JWT/AWS/Digest/NTLM、客户端证书、Cookie Jar。 | `model.AuthTypes`、`auth`、`model/tree.go` 已有 Bearer/Basic/API key 及继承；`protectRedirects` 处理跨源认证。`options.go` 仅解析 Set-Cookie，client 没有 Jar；无 OAuth2 取/刷新或客户端证书配置。S3 签名只用于同步。 | 不重复造已有继承；未来 OAuth 可先限定客户端凭证。Cookie 显示不能冒充持久会话。 |
| 变量/链 | 文件夹变量、动态函数、响应 JSONPath/XPath 引用及依赖请求。 | `gui/env.go:VarsFor` 合并集合和环境，后者覆盖；`vars.Expand` 为单次 `{{name}}` 替换；`Item` 无文件夹变量。脚本变量改变只影响本次发送，测试脚本不把值持久化给下一请求。 | 先明确作用域、输出绑定与失效，不悄悄引入自动依赖请求。 |
| 脚本/断言 | 文档明确没有任意前置/后置请求脚本，以模板和 Node 插件代替部分用途。 | `script.RunPre/RunTests`、`script/api.js`、`gui/scripts.go` 已有 Otto JS、`pm.test/expect`、响应 JSON/headers、请求头修改及超时，GUI 显示结果；不支持异步测试，也不是完整 Postman SDK。 | 这是 Reva 已有优势，应复用而不是标成缺失。 |
| 运行器/CI | 当前 CLI 可批量 HTTP；无内置 CSV/完整断言报告 UI；CLI 不发送 gRPC/WS。 | `cmd/reva` 只有 GUI 入口，`App.Send/run` 按请求工作；未发现集合 CLI/JUnit 输出。现有 GitHub CI 是测试软件本身。 | 最小集合 CLI 可以真正复用 Reva 断言，需定义报告与退出码。 |
| 迁移/导出 | 多格式、多来源预览；原生 JSON、curl 导出；OpenAPI 导出仍 Backlog。 | `importer/result.go/openapi.go/fetch.go`、`gui/importer.go` 已有原生/Postman/HAR/curl/Swagger2/OpenAPI3.0/3.1 JSON 预览、URL 获取和追加；原生 JSON、加密备份可导出。YAML、Insomnia/Bruno 原生和 OpenAPI 导出未实现。 | 保留有界、先预览再确认的强项；跨厂商格式不承诺无损。 |
| Mock/schema/压测 | 本次未证实通用响应契约校验、Mock 服务或压测引擎。 | `importer/openapi.go:sample` 明确只生成样例，不验证 schema；`format` 处理 JSON 展示；`script` 可业务断言；未见 Mock 或负载发生器。 | 分别集成专业契约、Mock 和性能工具，勿将格式检查当契约测试。 |
| 协作/本地数据 | SQLite 主库、YAML 目录同步、内置 Git、密钥链保护选定字段；不是实时云共同编辑。 | `storage`/`service/store` 为 JSON；`syncing.Seal/Merge/Shared` 及 `gui/sync.go` 已有加密备份、WebDAV/S3/Gist 同步和冲突检查。原生导出包含配置凭据，输入遮罩不是加密；无内置 Git UI。 | 先改可分享副本的字段边界，不改本地免账号架构。 |
| 性能/隐私 | Tauri/Rust/React 附带 Node/protoc；桌面直连，但运营检查仍联网。 | Go/Viva 原生绘制；`request/options.go` 重用连接池，`Send` 默认保留最多 10 MiB 但继续统计下载量，`trace.go` 有时间线。README 成品测量约 15 MB，与开头约 5 MB 历史概括不同。 | 无同机基准，不能声称任何一方更快更小；应另测大响应、长流、空闲内存和完整包体积。 |

Reva 对照的可点击本地证据：

- [模型](../../../internal/model/model.go)、[认证树](../../../internal/model/tree.go)、[认证处理](../../../internal/auth/auth.go)、[服务契约](../../../internal/service/service.go)。
- [HTTP 发送](../../../internal/service/request/request.go)、[连接池与 Cookie](../../../internal/service/request/options.go)、[阶段时间线](../../../internal/service/request/trace.go)、[WebSocket](../../../internal/websocket/client.go)、[GraphQL 编辑器](../../../internal/gui/graphql.go)。
- [变量作用域](../../../internal/gui/env.go)、[变量替换](../../../internal/vars/vars.go)、[脚本执行](../../../internal/script/script.go)、[JS API](../../../internal/script/api.js)、[GUI 脚本编排](../../../internal/gui/scripts.go)、[发送入口](../../../internal/gui/app.go)。
- [导入格式检测](../../../internal/importer/result.go)、[OpenAPI 模板转换](../../../internal/importer/openapi.go)、[URL 获取](../../../internal/importer/fetch.go)、[导入预览 UI](../../../internal/gui/importer.go)、[原生集合导出](../../../internal/gui/collections.go)。
- [本地存储](../../../internal/storage/storage.go)、[加密快照与合并](../../../internal/syncing/sync.go)、[同步 UI](../../../internal/gui/sync.go)、[远端条件写接口](../../../internal/cloudsync/cloudsync.go)、[S3 同步签名](../../../internal/s3/sigv4.go)。
- [程序入口](../../../cmd/reva/main.go)、[自测 CI](../../../.github/workflows/ci.yml)、[README 包装测量](../../../README.md)、[测试替身](../../../internal/service/fake/fake.go)。

## 功能问题：当前跟踪器事实及交付核验

**当前没有可供抽样的普通 GitHub issue。** 仓库元数据 `has_issues=false`；一次 `is:issue` 搜索返回 `total_count=0`、`incomplete_results=false`。通用 `/issues?state=all&per_page=100` 列表实际是 PR，不能把这些条目冒充 issue。README 指向官方反馈站，因此下表使用 **六条官方反馈**：四条开放、两条 Released；它们的 YK 编号、状态、标签不是 GitHub 编号或 open/closed 元数据。[证据 2、26–28]

未达成“至少五条 GitHub feature issue、两开两关”的数量条件是跟踪器范围限制，不补造条目。下面尽可能保留同样的可审阅字段；反馈页没有可靠的 `updated_at` 字段，JSON 中全部为 `null`，表内改列**最后可见活动**，且明确区分创建时间。页面相对日期偶有粗略表述，均以页面显示的 UTC 绝对日期为准。

| 编号、精确标题及链接 | 当前原始状态/标签 | 最后可见活动（不是记录更新时间） | 用户需求与交付判断 |
| --- | --- | --- | --- |
| YK-1110 [Yaakcli to support GRPC](https://yaak.app/feedback/posts/yaakcli-to-support-grpc) | `Backlog`；`Feature Request`, `Improvement` | 评论 `2026-11-03T08:44:44Z` | 团队想让 GUI 的 gRPC 定义成为 CLI、智能体和评估脚本共同来源，还需要变量链和消息序列。**开放提议**：当前 [request.rs](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-cli/yaak-cli/src/commands/request.rs#L619) 仍返回未实现错误；桌面支持不能代替 CLI 交付。 |
| YK-104 [Multi-tab Support](https://yaak.app/feedback/posts/multi-tab-support) | `Under Consideration`；`Feature Request` | 评论 `2026-11-16T05:38:11Z` | 多个常用请求需保留上下文，但反对标签页的用户同样强烈。**开放提议**，未发现交付证明；对 Reva 的启示是已有 tabs 保持轻量且可控，不是再加一套 tabs。 |
| YK-211 [Automated Testing : scripting, collection run and report](https://yaak.app/feedback/posts/automated-testing-scripting-collection-run-and-report) | `Needs Triage`；`Feature Request` | 无评论；仅创建时间 `2024-09-15T21:49:06Z` | 用户需要脚本、集合运行、汇总报告，而非只 Send All。**组合需求尚未完整交付**：CLI 批量发送已存在，[文档](https://yaak.app/docs/advanced/running-multiple-requests) 同时否定 CSV 迭代，另页明确无[通用脚本](https://yaak.app/docs/templating/pre-post-request-scripting)。 |
| YK-186 [Export to OpenAPI](https://yaak.app/feedback/posts/export-to-openapi) | `Backlog`；`Feature Request` | 评论 `2026-08-05T12:20:20Z` | 不同 API 客户端之间需要中立格式共享。**开放提议**；评论提到 PR #324，但本次未核验合并/发布，不能据此认定完成；[当前导出文档](https://yaak.app/docs/collaboration/import-and-export-data) 只承诺原生 JSON/curl。 |
| YK-71 [import from bruno](https://yaak.app/feedback/posts/import-from-bruno) | `Released`；`Feature Request` | 发布状态 `2026-10-10T17:17:54Z` | 中转 Postman/OpenAPI 会漏环境。**已验证原生导入交付**：[v2026.9.0](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.0) 关联 PR #706，并有 [importer-bruno 源码](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/plugins/importer-bruno/src/index.ts)。 |
| YK-413 [Support for Importing zip or multiple files and only allow importable file formats.](https://yaak.app/feedback/posts/support-for-importing-zip-or-multiple-files-and-only-allow-importable-file-forma) | `Released`；`Feature Request`, `Bug`, `Improvement` | 发布状态 `2026-10-10T17:17:54Z` | 多工作区/多环境逐个迁移成本高，需要目录/ZIP/多文件预览。**核心批量导入已验证交付**：[v2026.9.0](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.0) 及[当前文档](https://yaak.app/docs/collaboration/import-and-export-data) 互证；文件选择过滤等原始子要求未逐项实测。 |

YK-71 特别说明关闭状态的误导性：2025-09-17 维护者因已有 Postman 中转方案而关闭，当时**不是原生导入交付**；2026-08-17 又因中转丢失环境重开；直到 2026.9.0 才有原生实现的稳定发行证明。页面同时列出一次 “Import from Bruno” 合并反馈，但未单独提供那个被合并条目的编号和完整元数据，本报告不计为第七项或独立交付。

这些需求体现三个产品判断：共同维护定义不等于每种协议都能自动化；发送汇总不等于断言报告；可迁移的协议格式也不一定带走环境、脚本或秘密。贡献政策还要求非 bug PR 先获得维护者在反馈项上的明确同意，不能把仓库可读等同于任何功能补丁都会被接受。[证据 36]

## 三个可独立审阅的 Reva 改进

### 1. P0：默认排除秘密的可分享集合导出

**场景与现有缺口：** 用户要把集合交给同事或提交 Git，同时保留自己的凭据。`exportCollection` 直接序列化集合，集合/文件夹认证、变量和显式 `Authorization`、`Cookie` 等都可能外泄。加密备份适合恢复，不适合分享可审阅且不带凭据的定义。

**最小 PR：** 在原生导出旁增加“分享副本”模式，预览变量、认证和已知敏感头，默认移除具体秘密值，以可编辑占位符保留键和作用域；允许用户额外排除任意字段。只改副本，不写回原集合。URL/body 中任意业务秘密无法可靠自动识别，完整结果预览必须使其可见，并说明人工检查责任，不宣称自动脱敏万无一失。

**依赖：** 复用原生 JSON 导入/导出、深拷贝或序列化副本、现有确认弹窗、中英文文案；新增纯转换函数即可。无需账号、网络、第三方依赖或先实现系统密钥链。完整恢复继续走现有备份路径。

**验收：** 含三层认证、重复敏感头、同名变量和 URL/body 自定义秘密的样本，默认结果不含已知凭据；人工勾选的额外秘密消失，未勾选风险在预览中可见；重新导入仍保留继承边界；导出前后原内存和磁盘字节相同。取消、保存失败均不写原集合。这里的 P0 针对分享时凭据暴露的直接风险，而非要求先重建秘密存储系统。

### 2. P1：复用单请求断言的最小 HTTP 集合 CLI

**场景与现有缺口：** 桌面维护一组健康检查或回归请求，想在本地/CI 用同一份集合运行。Reva 已有脚本断言，缺少批量编排和机器可读报告；Yaak 的发送成功计数也不足以表示业务正确。

**最小 PR：** 提取无 UI 的单请求执行编排，新增命令按集合树顺序发送 HTTP，显式选择环境，复用认证继承、`Options.Resolve`、`RunPre`、`RunTests`。提供取消、`--fail-fast`、稳定 JSON 报告及失败非零退出码。首版不并行、不迭代 CSV、不写环境或历史；变量修改继续限于当前请求，不暗示已有自动 token 链。

**依赖：** 小幅拆分 `gui/app.go`/`gui/scripts.go` 的编排到独立包，新命令复用 store、request、auth、vars、script。`REVA_HOME` 允许 CI 指定隔离目录。无需引入 UI 或 Node/账号系统。

**验收：** `httptest` 三端点记录实际调用顺序及继承头；HTTP 200 但 `pm.test` 失败必须导致报告失败和非零退出码。开启 fail-fast 后第三端点不被调用；取消能中止，死循环脚本按时终止；报告可解析且不默认输出正文/凭据；集合、环境、历史文件逐字节不变。

### 3. P2：有界 SSE 事件查看和即时取消

**场景与现有缺口：** 调试通知或模型流，第一条事件到达就要看到内容。当前 `Send` 等待响应体读取结束；WebSocket 会话不能直接替代 SSE。

**最小 PR：** 新增可选流式服务接口，复用 HTTP 构造、变量、认证、TLS/跳转与取消；服务侧解析 SSE `data/event/id`、多行 data，只保留固定数量最近事件并显示丢弃计数；通过 Viva `ui.Effect` 给界面传递批次。第一版不自动重连或重放，避免重复执行业务请求。

**依赖：** 服务契约、发送器和响应 UI 的小范围扩展及双语文案。网络/缓冲在服务侧，UI 不新增 goroutine、定时器或轮询；复用跨源认证保护。已有 HTTP 流程保持回归检查。

**验收：** `httptest` Flush 第一条后保持连接，headless GUI 必须在发送第二条前显示第一条；覆盖分片 UTF-8、多行 data、注释及 CRLF。验证容量上限、丢弃提示和取消后服务端 context 结束；普通 JSON 与跨源凭据剥离测试继续通过，全程无互联网测试。

### 更适合集成的专业工具

gRPC 复杂消息序列先考虑生成/复制 `grpcurl` 命令或打开 Yaak；契约测试可接 Schemathesis 一类工具，Mock 可接 Prism/WireMock，性能可接 k6/Vegeta。这是后续集成方向，不是本次已经验证的适配承诺；各工具的许可、输入格式和平台支持需各自核对。让 Reva 提供清楚的导出/命令接口，比把协议测试、Mock 与负载引擎全部塞进 Viva 客户端更符合体积和维护目标。Yaak CLI 的 HTTP 批量能力同样不能替代专门压测或契约模糊测试。

## 验证、范围限制与本地 PR 交付

本任务无新增行为测试。所有仓库必需检查在该 worktree 执行，进程环境设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2` 限制并发；没有修改全局 Go 设置、下载新项目依赖或测试访问互联网。检查均通过，退出码与实际输出尾如下：

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
ok   github.com/runbrick/reva/internal/service/request  2.407s
ok   github.com/runbrick/reva/internal/service/store    0.526s
ok   github.com/runbrick/reva/internal/storage          0.846s
ok   github.com/runbrick/reva/internal/syncing          2.163s
ok   github.com/runbrick/reva/internal/update           0.444s
ok   github.com/runbrick/reva/internal/vars             0.978s
ok   github.com/runbrick/reva/internal/websocket        1.178s
Exit: 0
```

同轮测试还包括 `internal/gui 10.412s`、`internal/importer 1.681s` 和 `internal/script 0.895s`，均 `ok`。提交前验证还通过：UTF-8 JSON 解析、全部必需字段与子项字段校验、10 项能力/9 个差异领域/6 条反馈/3 个建议/37 个来源数量核对；30 个 Markdown 本地链接均存在，代码块成对，反馈精确标题、基线和时间戳与 JSON 一致。`git diff --cached --check` 退出 0，暂存范围恰为两个获授权文档。

限制及不确定性：

1. GitHub 普通 issue 当前不可用，不能提供两开两关；六条替代反馈的 `updated_at` 未公开，明确留空而不拿评论时间替代。Released 条目有独立发行证据，开放条目保持开放。
2. 未运行 Yaak 二进制，当前源码/文档证明实现但不保证每个平台、身份提供方、ZIP 变体均兼容；没有双端性能、包体积或内存实测。
3. MIT 与预编译分发条款结论来自官方原文，不是所有依赖和商用合同的法律审计；未发现核心协议能力按版本收费的证据，不夸大为永久承诺。
4. 未证实的 Mock、通用 schema 或压测能力限定于本次文档和相关源码范围，不能排除第三方插件。OpenAPI 生成请求、protobuf 编辑验证及 CLI 模型 schema 均不等于响应契约验证。
5. Reva 基线明确且读过相关实现；所有建议是未来独立 PR，本次未做源码修改。未比较 HTTPie，也未从其商业桌面产品外推 CLI 功能。
6. 官网文档、价格和反馈会变；源码以固定提交链接记录。最近 beta 的 MCP 迁移单独记录在来源中，不算作本表稳定发行的功能证明。

本地 PR 建议标题：`docs: 研究 Yaak 的本地协作与自动化边界`。两文件同时审阅：Markdown 供人阅读，JSON 保留结构化的能力、差异、问题状态、交付证据及可验收改进。提交哈希从 `git log -1 --format=%H -- docs/research/api-tools/yaak.md` 获取，避免文档写入自身提交哈希；无远程 PR、无推送、无合并。

## 编号证据来源

1. [规范仓库与主分支](https://github.com/mountain-loop/yaak)
2. [上游说明与源码/二进制许可边界](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/README.md)
3. [MIT 许可证原文](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/LICENSE)
4. [官方商业分发、个人/商业用途及团队管理条款](https://yaak.app/pricing)
5. [桌面本地数据、网络请求及密钥保护边界](https://yaak.app/docs/getting-started/security-and-privacy)
6. [Rust 工作区与协议模块](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/Cargo.toml)
7. [桌面 Tauri 配置与附带 Node/protoc 资源](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-tauri/yaak-app-client/tauri.conf.json)
8. [gRPC 反射、proto 和流模式](https://yaak.app/docs/request-types/grpc-requests)
9. [SSE 渐进响应](https://yaak.app/docs/request-types/server-sent-events)
10. [OAuth2 流程、继承和令牌刷新](https://yaak.app/docs/authentication/oauth2)
11. [内置 OAuth2 插件实现](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/plugins/auth-oauth2/src/index.ts)
12. [响应模板请求链](https://yaak.app/docs/templating/request-chaining-scripting)
13. [明确尚无通用请求脚本](https://yaak.app/docs/templating/pre-post-request-scripting)
14. [工作区、子环境和文件夹变量](https://yaak.app/docs/templating/environments-and-variables)
15. [秘密字段加密与环境分享](https://yaak.app/docs/collaboration/secrets-encryption)
16. [目录同步与内置 Git](https://yaak.app/docs/collaboration/local-directory-git-sync)
17. [导入预览、支持格式与原生导出](https://yaak.app/docs/collaboration/import-and-export-data)
18. [稳定版本批量导入及 Bruno 交付证据（关联 PR #706）](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.0)
19. [最近稳定发行版及分发许可说明](https://github.com/mountain-loop/yaak/releases/tag/v2026.9.1)
20. [最近预发行版；MCP 移至 CLI，不当成稳定版能力](https://github.com/mountain-loop/yaak/releases/tag/v2026.10.0-beta.2)
21. [CLI 顺序执行、桌面并发、CSV/报告边界](https://yaak.app/docs/advanced/running-multiple-requests)
22. [CLI 与本地数据库和自动化](https://yaak.app/docs/getting-started/cli-usage)
23. [批量 HTTP 发送与非 HTTP 跳过源码](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-cli/yaak-cli/src/commands/send.rs)
24. [CLI gRPC/WebSocket 尚不可发送的源码](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/crates-cli/yaak-cli/src/commands/request.rs)
25. [插件运行时](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/packages/plugin-runtime/src/index.ts)
26. [仓库元数据：has_issues=false](https://api.github.com/repos/mountain-loop/yaak)
27. [普通 GitHub issue 检索为零且结果完整](https://api.github.com/search/issues?q=repo:mountain-loop/yaak+is:issue&per_page=100&sort=updated&order=desc)
28. [当前官方功能反馈与路线图入口](https://yaak.app/feedback)
29. [YK-1110 原始需求](https://yaak.app/feedback/posts/yaakcli-to-support-grpc)
30. [YK-104 原始需求](https://yaak.app/feedback/posts/multi-tab-support)
31. [YK-211 原始需求](https://yaak.app/feedback/posts/automated-testing-scripting-collection-run-and-report)
32. [YK-186 原始需求](https://yaak.app/feedback/posts/export-to-openapi)
33. [YK-71 关闭、重开与发布记录](https://yaak.app/feedback/posts/import-from-bruno)
34. [YK-413 批量迁移需求与发布记录](https://yaak.app/feedback/posts/support-for-importing-zip-or-multiple-files-and-only-allow-importable-file-forma)
35. [Bruno 导入实现](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/plugins/importer-bruno/src/index.ts)
36. [功能贡献需事先通过反馈项获得维护者同意](https://github.com/mountain-loop/yaak/blob/b9127dcca4c06c1cc495ca2a3ad78807110b291a/CONTRIBUTING.md)
37. [Cookie Jar](https://yaak.app/docs/authentication/cookies)
