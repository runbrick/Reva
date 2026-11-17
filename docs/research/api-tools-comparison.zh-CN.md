# 开源 API 开发测试工具与 Reva 功能对比

面向 Reva 维护者的产品调研与开发拆分建议。建议优先复用现有 HTTP、变量、认证与脚本基础，补齐可复现的集合回归、可解释的变量/会话状态和流式诊断；负载调度、契约模糊测试及复杂 mock 通过明确格式对接专业工具。

后续实施已完成首批 R1–R4：见[功能整理与分工](implementation-plan.zh-CN.md)和
[独立子工作树提交及最终验收](implementation-results.zh-CN.md)。本文中的基线比较、
原研究验证和 Word 版本仍对应调研时点；后续实现结果单独记录。

检索日期为 2026 年 11 月 16 日；Reva 比较基线为 `72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`。本轮覆盖 12 个代表项目、118 项已实现能力记录、75 条功能相关议题或官方反馈，以及 428 条项目内编号来源记录。来源可能跨项目重复，不作为唯一网址数量。

## 范围与证据判定

范围覆盖桌面/Web 客户端、命令行、测试框架、契约生成、压测和 mock。该集合并非所有开源工具的穷尽目录；Postman、Apifox、ApiPost 等闭源客户端没有作为开源项目计入。HTTPie 只研究 CLI，不能把专有 Desktop/Web 能力合并计算。

每个项目均由独立 Orca Codex 子工作树研究和提交，原始分支及提交见文末。公开需求定性抽样不代表投票、市场份额或需求频率。没有安装运行全部上游产品，也没有可比的启动、内存或吞吐基准。

Issue 的 open/closed、代码是否合并、是否稳定发布分别记录。关闭可以意味着拒绝或重复，开放也可能是状态滞后。候选版、beta 和开发分支有明确标注。Yaak 禁用 GitHub Issues，所列六条来自官方反馈站并保留 YK 编号和原状态。

## 工具定位与主要差异

| 项目 | 类型 | 已实现特色 | 对 Reva 的启发与边界 | 核心许可 |
| --- | --- | --- | --- | --- |
| [Bruno](api-tools/bruno.md) | 文件式客户端 | 文件项目、CLI 报告、脚本、gRPC、OAuth2 | 可审阅集合执行和本地覆盖；CLI 文档与代码有差异 | MIT |
| [Insomnia](api-tools/insomnia.md) | 桌面与 CI | 多协议、数据驱动运行器、Spectral、Inso CLI | 回归入口、变量来源、OAuth2；关联 Mock 许可独立 | Apache-2.0 |
| [Hoppscotch](api-tools/hoppscotch.md) | Web 桌面 CLI | 实时协议、GraphQL 内省、数据驱动 GUI runner | 回归、会话秘密、SSE；Web 与桌面权益不能互推 | MIT |
| [Yaak](api-tools/yaak.md) | 本地桌面 | gRPC、SSE、OAuth2、响应引用、文件同步 | SSE 与秘密导出；Reva 已有通用脚本而 Yaak 没有 | MIT |
| [HTTPie CLI](api-tools/httpie.md) | 终端客户端 | 离线构造、流式输出、会话、shell 组合 | 发送前预览、临时 Cookie、增量显示 | BSD-3-Clause |
| [Hurl](api-tools/hurl.md) | 文本测试 CLI | 响应捕获、会话链、断言、JUnit 报告 | 显式捕获和 Hurl 导出；复杂 CI 可交给 Hurl | Apache-2.0 |
| [Karate](api-tools/karate.md) | JVM 测试框架 | 结构匹配、流程复用、有状态 mock、并行测试 | 执行与诊断；完整 DSL 和负载测试留在专业工程 | MIT |
| [REST Assured](api-tools/rest-assured.md) | JVM 测试库 | 可读断言、过滤器、会话、可选 schema 模块 | 串行回归、隔离 Cookie、断言生成 | Apache-2.0 |
| [Schemathesis](api-tools/schemathesis.md) | 契约生成测试 | 边界输入、状态链、失败缩减、pytest/CI | 保留契约与交接包；不重写模糊测试引擎 | MIT |
| [Grafana k6](api-tools/k6.md) | 性能测试引擎 | 负载执行器、指标阈值、gRPC、WebSocket | 运行报告与受限导出；容量模型交给 k6 | AGPL-3.0 |
| [Apache JMeter](api-tools/jmeter.md) | 性能测试计划 | 线程计划、CSV、关联、录制、分布式执行 | 串行与数据驱动执行；HTTP/2 仅有开发分支证据 | Apache-2.0 |
| [WireMock](api-tools/wiremock.md) | API 模拟服务 | 匹配、模板、状态、录制、故障、近似匹配 | 静态桩导出与外部诊断；WebSocket 为 4.x beta | Apache-2.0 |

## Reva 当前已有能力与真实缺口

| 领域 | 当前基线行为 | 源码依据 |
| --- | --- | --- |
| HTTP 与诊断 | 已实现 HTTP(S)、取消、请求选项、响应 Cookie 查看与 DNS/TCP/TLS 等阶段时间线。 | internal/service/request/request.go；trace.go；internal/gui/response.go |
| GraphQL 与 WebSocket | 已有 GraphQL query/variables 编辑与 WebSocket 客户端。未找到 gRPC 调用模型或专用 SSE 事件视图。 | internal/gui/graphql.go；websocket.go；internal/model/model.go |
| 认证 | 已有 Basic、Bearer、API key 和集合/文件夹继承。尚无 OAuth2 token 生命周期。 | internal/auth/auth.go；internal/model/model.go |
| 变量与脚本 | 集合变量与活动环境叠加；Otto 限时脚本、pm.test/expect 有限子集已实现。没有完整 Node/Postman 运行时。 | internal/gui/env.go；internal/vars/vars.go；internal/script |
| 数据与迁移 | 本地 JSON、可选加密备份/同步、导入审核和集合追加已实现；配置文件本身不是秘密 vault。 | internal/storage；internal/syncing；internal/importer；README.md |
| 执行与契约 | 唯一 cmd/reva 入口启动 GUI；未找到面向用户集合的 headless runner、数据驱动运行或通用响应 schema 校验。 | cmd/reva/main.go；internal/gui/app.go；internal/model/model.go |

不能把 GraphQL 请求编辑等同于内省与模式浏览，不能把响应 Cookie 查看等同于跨请求 Cookie jar，也不能把导入 OpenAPI 等同于运行时契约验证。已有加密备份/同步不代表本地配置凭据自动加密。

## 许可和产品边界

Insomnia 主仓库 Apache-2.0 与关联自托管 Mockbin 的商业源码可见许可分开。Yaak 源码 MIT 与官方二进制商业使用条款分开。Hoppscotch 根 MIT 与后端包 UNLICENSED 元数据冲突保留待澄清，企业版/云端权益另计。Bruno 公共核心、CLI 与桌面商业权益分开。k6 保留上游 AGPL-3.0 标识，不自行消解 only/or-later；Cloud、Studio、Operator 和扩展分开审视。其余框架的核心许可也不自动覆盖传递依赖和外部商业服务。具体许可链接和范围见各项目报告。

## 可独立评审的后续 PR

以下为建议，并未实现这些功能，也未创建远程 PR。优先级来自 Reva 的本地免账户定位和复用成本，没有把议题样本当作需求频率。R1 → R2 → R3 是执行主线；R4 可以先行，R5/R6 可在接口明确后并行，R7/R8 按场景选做。

### R1 P1 统一可复用的发送与脚本语义

先明确变量、认证继承、请求选项、前置失败、响应断言、取消与历史写入的顺序，再抽出 GUI 与执行器共用管线。

依赖：无。验收：httptest 验证有无脚本、认证继承、取消、前置错误的实际发送行为；结果不泄露展开凭据，现有 GUI 行为有明确兼容边界。

### R2 P1 原生集合的确定性串行执行

先固定树遍历顺序、单次执行变量快照、失败策略与超时；默认只读集合/环境，不写历史。

依赖：R1。验收：三个本地端点记录顺序；取消阻止后续请求；断言失败进入结果；执行前后数据文件字节不变。

### R3 P1 无窗口 CLI 与版本化运行报告

把 R2 暴露为独立命令，给出稳定退出码和脱敏 JSON 结果；以后再加 JUnit。软件自身构建 CI 不等于此能力。

依赖：R2。验收：在无 GUI 条件下执行临时集合；成功退出 0、断言或脚本失败非零；报告 schema 有版本且不含 token。

### R4 P1 变量来源与发送前只读预览

显示集合/环境覆盖、最终 URL 和未知变量；预览在脚本之前计算，秘密值默认隐藏。

依赖：共用现有 vars 规则。验收：切换环境改变来源和目标；禁用行回退；预览不执行脚本、不发送、不写盘，快照不含凭据。

### R5 P1 执行作用域内的秘密与 Cookie 会话

先分别设计可清空的内存秘密覆盖和默认关闭的 Cookie 会话；它们是两个可独立提交的 PR。

依赖：R1；R2 可复用。验收：两个测试服务器验证跨源隔离；导出与历史不含内存秘密；清空或重建 App 后会话消失。

### R6 P1 有界 SSE 事件展示

在 HTTP 服务边界增加流式消费及背压、体积上限、取消；支持既有 header/认证，UI 沿用 Viva Effect。

依赖：流式服务接口与 GUI 需分别约定。验收：httptest 分批 flush 事件，首条在连接关闭前可见；慢流可取消，事件上限和错误重试均有行为覆盖。

### R7 P2 受限导出连接专业工具

Hurl 测试文件、WireMock 静态桩、k6 脚本各自独立 PR；先出预览和不支持项告知，再由用户确认保存。

依赖：各格式的兼容子集。验收：用确定样例验证方法/URL/body 与禁用行；认证默认参数化；不启动外部工具、不发送请求、不静默丢脚本。

### R8 P2 数据驱动或 OAuth2 按真实场景追加

CSV/JSON 迭代依赖稳定执行结果；OAuth2 先限定 client_credentials。两者分别评审，不捆绑发布。

依赖：数据驱动依赖 R2/R3；OAuth2 依赖会话规则。验收：数据行错误可定位且不改环境；取令牌使用本地测试服务，跨源重定向不带 secret，取消可用且令牌不落盘。

## 逐项目比较与全部议题摘要

### Bruno

Bruno 是文件优先的跨平台 API 开发与测试客户端，差异集中在可 Git 评审的项目、脚本和断言、可进入 CI 的 CLI。Reva 已有 HTTP、GraphQL、WebSocket、限时 JavaScript、认证继承及可选加密同步；应先补会话本地覆盖与最小执行管线，通过明确导出连接专业工具，保持免账号、纯 Go/Viva。

仓库：https://github.com/usebruno/bruno。许可：MIT。查询：2026-11-16T22:29:29+08:00。

[完整研究与编号来源](api-tools/bruno.md) · [结构化数据](api-tools/bruno.json)

已实现特色：文件项目、YAML 与 Git 协作；公共 CLI 执行与机器报告；声明式断言与脚本结构校验；多层脚本与可选高权限模式；gRPC 流式调用及多协议编辑；OAuth2 系统浏览器与 PKCE；变量类型、作用域及环境继承；秘密值与共享文件分离；响应示例与本机 Mock；多工具和规范导入导出。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#2650](https://github.com/usebruno/bruno/issues/2650) | Use system browser for OAuth authentification | closed | 已核实交付；依据官方操作文档，不以关闭状态推断，未追溯首次合并 PR。 |
| [#1235](https://github.com/usebruno/bruno/issues/1235) | Save Response Examples | closed | 已核实交付；文档有保存步骤。v4.3.0 Try PR #9342 是后续重放能力，不是最初保存功能合并证明。 |
| [#5887](https://github.com/usebruno/bruno/issues/5887) | Socket.IO support | open | 开放提案；本次没有找到对应交付文档或合并证据。 |
| [#1216](https://github.com/usebruno/bruno/issues/1216) | [Feature Request] variable initial and local value | open | 开放提案；秘密变量和运行时变量解决部分场景，未核实通用初始值/本地值双栏交付。 |
| [#292](https://github.com/usebruno/bruno/issues/292) | Nested Environments? | open | 议题仍开放，但功能已核实：文档称 4.2.0 起有 extends，v4.3.0 源码有父链合并和环检测；不能据状态滞后认定缺失。 |
| [#949](https://github.com/usebruno/bruno/issues/949) | Import secrets from 1password | open | 开放提案；Ultimate 已列四类外部管理器，但不含 1Password，未核实此需求交付。 |

建议小 PR：会话本地覆盖，避免个人值进入共享集合；共享执行管线与最小顺序集合 CLI；带兼容审查的 Postman v2.1 导出连接 Bruno。

### Insomnia

Insomnia 的可借鉴优势在于把多协议调试、分层变量、数据驱动集合运行和规范/CI 工作流连在一起；Reva 已有脚本、GraphQL、WebSocket、共享认证和可选加密同步，应优先补可复现执行与变量透明度，保持本地免账户与 Go/Viva 方向。

仓库：https://github.com/Kong/insomnia。许可：Apache-2.0。查询：2026-11-16T22:29:46+08:00。

[完整研究与编号来源](api-tools/insomnia.md) · [结构化数据](api-tools/insomnia.json)

已实现特色：多协议请求工作台；多种认证配置；分层环境与本地秘密；可异步执行的前后置脚本；数据驱动集合运行器；同一项目数据的 CI 命令行；规范编辑与规则校验；云端与自托管 mock；多种数据归属与协作方式。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#1638](https://github.com/Kong/insomnia/issues/1638) | [FEATURE REQUEST] Ability to save and work on a collection/doc in local file system | open | 开放提案；现有 Git Sync 是相关能力，未验证其满足任意路径直接打开编辑的原诉求。 |
| [#1784](https://github.com/Kong/insomnia/issues/1784) | [Feature Request] Shared Fragments | open | 开放提案；维护者在 2025-02-25 说明可用环境变量复用，多行编辑仍不直观；未核实原生共享 fragment 管理交付。 |
| [#3294](https://github.com/Kong/insomnia/issues/3294) | [FEATURE REQUEST] Looping through a Data File like in the Postman Collection Runner | closed | 已核实当前交付；2024-11-19 维护者称 GUI/CLI 已交付，现行 CLI 文档明确列出 CSV/JSON 参数，13.4 发布说明还有 CSV 引号解析修复。 |
| [#281](https://github.com/Kong/insomnia/issues/281) | [Feature Request] Swagger (OpenAPI) import | closed | 已核实当前交付；历史曾标为 #367 重复项，2019-02-07 按完成关闭；当时评论仍反映 OAS3 限制，不能据关闭日期宣称所有格式已支持，当前规范/导入文档确认现行能力。 |
| [#203](https://github.com/Kong/insomnia/issues/203) | [Feature] Per-request Variables | closed | 已关闭但原提案交付未核实；2021-06-30 被标为 completed 并锁定，所读页面未提供可核对的合并 PR/版本链。当前请求文档有 Path Parameters，但不足以证明原提议的通用请求变量面板完整交付。 |
| [#1389](https://github.com/Kong/insomnia/issues/1389) | [Feature Request] Allow local encryption of passwords/user selected data | open | 开放提案、部分相关能力已实现；Secret 变量有本地加密，但只覆盖私有全局子环境，不能据此宣称证书密码或任意字段全被加密。 |

建议小 PR：本地串行集合回归命令；发送前变量来源与覆盖预览；受限 OAuth2 客户端凭据取令牌。

### Hoppscotch

Hoppscotch 是以 Web 为中心，覆盖 Tauri 桌面和 CLI 的 API 开发生态；对 Reva 最有价值的是数据驱动回归、会话秘密隔离和可认证流式调试，而非复制云端协作服务。

仓库：https://github.com/hoppscotch/hoppscotch。许可：MIT。查询：2026-11-16T14:30:47Z。

[完整研究与编号来源](api-tools/hoppscotch.md) · [结构化数据](api-tools/hoppscotch.json)

已实现特色：多协议实时调试；GraphQL 内省与模式浏览；继承认证与 OAuth2 授权流程；分层变量与秘密值隔离；预请求与响应测试脚本；GUI 数据驱动集合运行器；本地 CLI 与 CI 结果；多工具迁移与 OpenAPI 导出；共享工作区与角色；由响应示例建立 Mock。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#3145](https://github.com/hoppscotch/hoppscotch/issues/3145) | [feature]: REST features for SSE | open | 开放提案；完整 REST 配置未验证交付 |
| [#4528](https://github.com/hoppscotch/hoppscotch/issues/4528) | [feature]: Set/unset secret variables from scripts | open | 开放提案；秘密变量本身已实现，脚本设置/删除秘密值未验证交付 |
| [#5478](https://github.com/hoppscotch/hoppscotch/issues/5478) | [feature]: Automatic OAuth 2.0 Token Generation on Collection Run | open | 开放提案；作者声称提供实现不等于已合并或已发布 |
| [#5163](https://github.com/hoppscotch/hoppscotch/issues/5163) | [feature]: Collection runner via GUI with ability to import files and choose which requests should be run | closed | 已关闭；已核实交付 |
| [#3821](https://github.com/hoppscotch/hoppscotch/issues/3821) | [feature]: Export to OpenAPI format | closed | 已关闭；已核实交付 OpenAPI 3.1 导出 |
| [#6285](https://github.com/hoppscotch/hoppscotch/issues/6285) | [feature]: Add file support to Collection Runner | closed | 已关闭为重复；关闭时不代表交付，后由规范议题 #5163 的发布证据确认同类需求交付 |
| [#5980](https://github.com/hoppscotch/hoppscotch/issues/5980) | [feature]: Mock Server — Variable resolution and dynamic response body substitution | open | 开放提案；响应变量替换未验证交付，当前核对到的格式化函数返回静态正文 |
| [#6411](https://github.com/hoppscotch/hoppscotch/issues/6411) | [feature]: Add detailed response timing breakdown similar to Postman | open | 开放提案；未验证交付详细阶段时间轴，Reva 当前已有相关实现 |

建议小 PR：增加纯内存会话秘密变量覆盖层；新增原生集合的无窗口顺序回归命令；为 HTTP 增加可认证的 SSE 实时响应视图。

### Yaak

本地优先、无需账号的多协议 API 桌面客户端，适合交互式调试、模板化请求链和 Git 文件协作。与 Reva 方向接近，但运行栈与商业分发不同；其 gRPC、SSE、OAuth2、秘密字段保护比当前 Reva 完整，而 Reva 已有的任意前置脚本和响应断言并非 Yaak 已实现能力。当前 GitHub 禁用普通 issue，检索为零；以下问题条目来自官方反馈站并保留 YK 编号，不冒充 GitHub issue。

仓库：https://github.com/mountain-loop/yaak。许可：MIT。查询：2026-11-16T14:33:27Z。

[完整研究与编号来源](api-tools/yaak.md) · [结构化数据](api-tools/yaak.json)

已实现特色：gRPC 反射、proto 编辑与四种调用模式；SSE 与渐进响应展示；OAuth2 生命周期及继承认证；响应引用模板形成请求链；工作区、子环境与文件夹变量；秘密字段加密与操作系统密钥链；目录双向同步及可视化 Git 操作；多格式批量迁移和导入预览；共享本地数据的 HTTP 批量 CLI；插件化认证、模板与菜单动作。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [YK-1110](https://yaak.app/feedback/posts/yaakcli-to-support-grpc) | Yaakcli to support GRPC | Backlog | 开放提议；当前 CLI 明确未实现 gRPC 发送，GUI gRPC 已实现不能代替此项交付。 |
| [YK-104](https://yaak.app/feedback/posts/multi-tab-support) | Multi-tab Support | Under Consideration | 开放提议；未发现该请求标签页功能的发布证明，不能将多个工作区窗口当成已完成。Reva 已有请求标签页。 |
| [YK-211](https://yaak.app/feedback/posts/automated-testing-scripting-collection-run-and-report) | Automated Testing : scripting, collection run and report | Needs Triage | 开放的组合需求；CLI 批量 HTTP 发送已实现，但任意脚本和完整断言报告未交付，不能整体标为完成。 |
| [YK-186](https://yaak.app/feedback/posts/export-to-openapi) | Export to OpenAPI | Backlog | 开放提议；评论提及 PR #324 只是线索，本次未验证其合并/发布，不能作为已交付证据；当前文档只承诺原生 JSON 和 curl 导出。 |
| [YK-71](https://yaak.app/feedback/posts/import-from-bruno) | import from bruno | Released | 已验证交付 v2026.9.0，经发行说明关联 PR #706 和 importer-bruno 源码交叉确认。曾在 2025-09-17 以中转方案关闭、2026-08-17 重新打开；历史关闭本身并不代表当时已实现原生导入。 |
| [YK-413](https://yaak.app/feedback/posts/support-for-importing-zip-or-multiple-files-and-only-allow-importable-file-forma) | Support for Importing zip or multiple files and only allow importable file formats. | Released | 核心批量导入已验证交付 v2026.9.0，发行说明引用 PR #706；官方文档确认文件/目录/URL 批量入口。未逐项实测文件选择器过滤和所有 ZIP 变体，不能把整个原始愿望清单视为逐项验收。 |

建议小 PR：增加默认排除秘密的可分享集合导出；复用现有脚本语义提供最小 HTTP 集合 CLI；为 SSE 增加有界事件查看与即时取消。

### HTTPie CLI

HTTPie CLI 是免品牌账号的 Python 终端 HTTP 客户端，以可读请求语法、离线构造、持久会话、流式输出和 shell 组合见长；适合补充 Reva 的终端工作流。Reva 已有集合、变量、JavaScript、WebSocket 和本地 GUI，不能把这些现有功能当作待补缺口。

仓库：https://github.com/httpie/cli。许可：BSD-3-Clause。查询：2026-11-16T22:29:59+08:00。

[完整研究与编号来源](api-tools/httpie.md) · [结构化数据](api-tools/httpie.json)

已实现特色：按类型构建嵌套 JSON；离线构造并输出请求；可复用的本地会话；逐行响应与 SSE 自动流式显示；下载、管道与续传；内置 Basic、Digest、Bearer 认证；代理及客户端 TLS 证书；终端格式化与交换元信息；shell/CI 退出码与标准流；插件接口与管理器。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#1697](https://github.com/httpie/cli/issues/1697) | Basic WebSocket Client Support in CLI | open | 开放提案，未核实交付 |
| [#1425](https://github.com/httpie/cli/issues/1425) | Support httpie in Chrome DevTools | open | 开放提案，未核实交付 |
| [#1215](https://github.com/httpie/cli/issues/1215) | Support Bearer Authentication | closed | 已验证交付：3.0.0 |
| [#946](https://github.com/httpie/cli/issues/946) | Support for client side certificate/key password | closed | 已验证交付：3.1.0 |
| [#376](https://github.com/httpie/cli/issues/376) | Server-sent events explicitly requires --stream | closed | 已验证交付：3.0.0 |
| [#243](https://github.com/httpie/cli/issues/243) | Response time | closed | 已验证交付：3.0.0 |

建议小 PR：增加安全的离线请求预览；增加默认关闭的临时 Cookie 会话；为 SSE 增加可取消的增量文本显示。

### Hurl

Hurl 是以可提交到 Git 的文本描述 HTTP 会话及断言的 Rust 命令行测试工具。它适合作为 Reva 的持续集成和复杂会话测试搭档；Reva 已有脚本、GraphQL、WebSocket、原生导出和可选加密同步，不能将这些现有能力误列为缺失。抓取时最新正式版为 8.1.0，发布于 2026-10-28；报告的已实现判断以该标签和公开文档交叉验证。

仓库：https://github.com/Orange-OpenSource/hurl。许可：Apache-2.0。查询：2026-11-16T14:30:38+00:00。

[完整研究与编号来源](api-tools/hurl.md) · [结构化数据](api-tools/hurl.json)

已实现特色：文本会话、响应捕获和请求串联；面向 HTTP 内容与元数据的声明式断言；有类型的模板输入与日志秘密遮蔽；围绕断言结果重试、延迟和重复；以测试文件为单位的并行执行器；持续集成报告及明确退出码；libcurl 协议、认证和 TLS 控制；格式检查与受限 curl 互通；分层默认配置；耗时门槛和结构化网络阶段指标。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#115](https://github.com/Orange-OpenSource/hurl/issues/115) | Feature Requests: Imports | open | 开放提案；有外部分支原型，未核实主线交付 |
| [#1907](https://github.com/Orange-OpenSource/hurl/issues/1907) | include response bodies in the json output | open | 原提案仍开放；替代的目录报告能力已实现 |
| [#3219](https://github.com/Orange-OpenSource/hurl/issues/3219) | Support curl ;type magic string in multipart/form-data request | open | 开放提案；文件分段类型已有，文本分段扩展未核实交付 |
| [#5397](https://github.com/Orange-OpenSource/hurl/issues/5397) | implicit body placeholders | open | 开放提案；语义 JSON 差异和省略字段语法尚未核实交付 |
| [#3629](https://github.com/Orange-OpenSource/hurl/issues/3629) | Add default config file feature | closed | 已关闭且核实交付；最终路径与初始提案不同 |
| [#5335](https://github.com/Orange-OpenSource/hurl/issues/5335) | Add --tlsv1.0 --tlsv1.1 --tlsv1.2 --tlsv1.3 | closed | 已关闭且核实交付；最终选项为 --tls-min |

建议小 PR：提供显式、仅内存的响应捕获变量；导出受限 Hurl 测试文件，把 CI 执行交给专用工具；基于现有 pm.test 生成语义 JSON 断言片段。

### Karate

Karate 是 Java 21 上把 API 流程、结构断言、有状态 mock、并行回归和 Gatling 集成在同一 DSL 的 MIT 开源框架。对 Reva 最有用的是可重复执行与诊断工作流，复杂负载与状态机 mock 适合通过文件交接给专业测试工程。

仓库：https://github.com/karatelabs/karate。许可：MIT。查询：2026-11-16T22:41:28+08:00。

[完整研究与编号来源](api-tools/karate.md) · [结构化数据](api-tools/karate.json)

已实现特色：面向 API 的 Gherkin 与结构断言；可复用的标记式结构校验；数据驱动的独立用例；请求链与初始化复用；虚拟线程并行与失败即停；有状态 HTTP mock 与状态快照；独立执行与可归档报告；复用功能用例进行负载测试；开源 WebSocket 异步断言；可配置认证与登录步骤复用。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#3131](https://github.com/karatelabs/karate/issues/3131) | Add a 'Copy' CTA button to copy the HTTP Request/Response in HTML report | open | 开放提案；未找到交付证据 |
| [#3107](https://github.com/karatelabs/karate/issues/3107) | on a callonce/callSingle failure- Limit the call marker only on the executing scenario in the HTML report | open | 开放；有 fixed 标签和引用提交，但合并及发版未核实 |
| [#3095](https://github.com/karatelabs/karate/issues/3095) | Karate mock server variables | closed | 已核实交付：v2.1.4 的 mock.get(name, defaultValue) |
| [#3075](https://github.com/karatelabs/karate/issues/3075) | Expose bounded request-header and body limits in the native HTTP server API | closed | 已核实交付：v2.1.4 |
| [#3073](https://github.com/karatelabs/karate/issues/3073) | Provide sorting by Feature File Path in Karate v2.x Test Execution Report | closed | 已核实交付：v2.1.4 路径排序；未确认全部提案 UI |
| [#3007](https://github.com/karatelabs/karate/issues/3007) | Feature request: built-in browser video recording for UI tests | closed | 关闭但未在社区核心交付：wontfix |

建议小 PR：增加原生集合的串行无界面回归入口；让深层 JSON 断言报告首个差异路径；预览并导出单个 HTTP 请求的 Karate 测试骨架。

### REST Assured

REST Assured 是面向 JVM 工程和 CI 的代码式 HTTP 验证库，Reva 是本地免账户交互客户端。可借鉴回归执行、可读断言与会话隔离；复杂 JVM/Spring/契约测试宜留在专项工程。当前稳定版为 6.3.0，另有已发布的 7.0.0-RC1。

仓库：https://github.com/rest-assured/rest-assured。许可：Apache-2.0。查询：2026-11-16T22:37:40+08:00。

[完整研究与编号来源](api-tools/rest-assured.md) · [结构化数据](api-tools/rest-assured.json)

已实现特色：声明式 HTTP 回归测试；JSON 与 XML 路径提取；多草案 JSON Schema 校验；请求及响应规范复用；可编程过滤器与失败日志；认证与证书测试；会话与 CSRF 流程；Spring 内进程控制器测试；客户端耗时提取与阈值断言；按客户端隔离并行默认值。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#462](https://github.com/rest-assured/rest-assured/issues/462) | Add support for asynchronous REST calls | open | 开放提案；通用异步交付未核实。评论称已有 PR 不是合并或发布证据，本报告未计入已实现功能。 |
| [#287](https://github.com/rest-assured/rest-assured/issues/287) | Make specifications immutable | open | 开放提案；原始“全部 setter 返回新规范”未核实交付。RC1 的发送副本和不可变客户端只解决相关共享问题，不等于 builder 全面不可变。 |
| [#589](https://github.com/rest-assured/rest-assured/issues/589) | Make it possible to configure how parameters are merged or overwritten | closed | 已核实交付；2.6.0 变更日志明确引用 #589，现行 ParamConfig 文档演示 MERGE/REPLACE。 |
| [#551](https://github.com/rest-assured/rest-assured/issues/551) | Validate all assertions at the same time with overloaded body/content method | closed | 已核实交付，范围限定为同一 body 的多个期望；2.5.0 变更日志说明会列出所有失败。不是跨全部链式调用的通用软断言。 |
| [#550](https://github.com/rest-assured/rest-assured/issues/550) | Add support for soft assertions | closed | 已关闭，原提案未交付；维护者评论指出该设计会耦合 JUnit。本次未找到该 Rule/soft API 的发布证据，#551 的有限聚合不能代替此提案。 |
| [#493](https://github.com/rest-assured/rest-assured/issues/493) | Support measuring time  | closed | 已核实交付；2.8.0 日志明确引用 #493，现行响应校验文档提供 time/timeIn、时间匹配器及预热说明。 |
| [#534](https://github.com/rest-assured/rest-assured/issues/534) | Support baseURL/port setting for parallel testing | open | 议题仍开放，但相关能力已核实在 7.0.0-RC1 交付；候选版日志明确引用 #534，并有 RestAssuredClient 文档。不能按旧评论判断当前完全不能并行，也不能称稳定版已全面支持。 |

建议小 PR：原生集合的串行无界面回归执行；执行作用域内可清空的 Cookie 会话；从响应字段生成可编辑断言片段。

### Schemathesis

Schemathesis 是本地可运行的契约驱动 API 测试工具，长于生成边界/无效输入、自适应复用真实数据、状态链、失败缩减和 CI 工件；适合补足 Reva 的批量探索能力。Reva 已有脚本、GraphQL、WebSocket 与可选加密同步，应继续承担免账号桌面调试，而非重建同一模糊测试引擎。

仓库：https://github.com/schemathesis/schemathesis。许可：MIT。查询：2026-11-16T22:38:10.316001+08:00。

[完整研究与编号来源](api-tools/schemathesis.md) · [结构化数据](api-tools/schemathesis.json)

已实现特色：从多种契约生成真实请求；四阶段生成与失败缩减；响应驱动的自适应测试；自动推导多步骤状态链；契约与对象级授权检查；声明式认证及逐操作配置；失败工件、重放与独立测试导出；可审阅的 CI 失败基线；可离线审阅的多格式报告；可控流量与 Python 扩展。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#5751](https://github.com/schemathesis/schemathesis/issues/5751) | [FEATURE] Dictionaries filled from an API operation, with parent and child drawn together | open | 开放提案；未验证交付，不能把现有资源池等同于提案中的 from-operation 字典 |
| [#5549](https://github.com/schemathesis/schemathesis/issues/5549) | [FEATURE] Classify 4xx responses by body, not only by status code | open | 开放提案；未验证按正文配置的统一 4xx outcome 规则已交付 |
| [#5417](https://github.com/schemathesis/schemathesis/issues/5417) | [FEATURE] Apply captured resource ids to array items and `$ref` nested objects in request bodies | closed | 已验证交付：PR #5423 已合并，4.32.0 变更记录明确列出数组项与 $ref 对象填充 |
| [#3620](https://github.com/schemathesis/schemathesis/issues/3620) | Feature proposal: declarative JWT auth with fetching new tokens | closed | 已验证交付：PR #3623 已合并，4.13.0 记录及 4.33.4 认证文档确认声明式动态认证；不是浏览器 OAuth2 全流程 |
| [#1722](https://github.com/schemathesis/schemathesis/issues/1722) | [FEATURE] Automatically delay requests based on the `Retry-After` header value | closed | 已验证交付：PR #4289 已合并，4.23.0 明确发布 --rate-limit=auto；pytest 不自动等待/重试 429 |
| [#2999](https://github.com/schemathesis/schemathesis/issues/2999) | [FEATURE] Output tests run to recreate manually without having to use seed. | closed | 已验证交付的具体范围：PR #5347 已合并，4.32.0 发布独立 pytest 失败文件；不是任意格式完整运行导出，也不是自动生成修复后应通过的回归断言 |
| [#3511](https://github.com/schemathesis/schemathesis/issues/3511) | [FEATURE] Test only untested new, or changed APIs | closed | 已关闭但该自动差异检测未交付：维护者在关闭评论明确认定超出项目范围；现有 include/exclude 可接受外部 schema diff 的结果，COMPLETED 标记不能证明交付 |

建议小 PR：导入时保留最小响应契约并提示状态码/媒体类型漂移；增加免 GUI 的确定性集合顺序执行入口；导出可审阅的 Schemathesis 测试交接包。

### Grafana k6

k6 是以代码编写场景的本地性能测试引擎和 CLI，重点在受控负载、指标及失败门禁，适合补充 Reva 的交互调试。检索到的最新正式版为 v2.4.0，发布于 2026-11-12；Reva 已有 HTTP、GraphQL、WebSocket、隔离的前后置脚本、OpenAPI 导入及可选加密同步，真实缺口在可重复的集合执行和结构化运行结果。建议先做本地顺序回归与报告，再提供可审阅的 k6 脚本导出；高并发、到达率调度、浏览器性能及分布式压测交给 k6。

仓库：https://github.com/grafana/k6。许可：AGPL-3.0。查询：2026-11-16T22:38:00+08:00。

[完整研究与编号来源](api-tools/k6.md) · [结构化数据](api-tools/k6.json)

已实现特色：多场景及开放、封闭负载模型；检查与性能阈值分离；协议与浏览器混合性能测试；脚本生命周期及 TypeScript；数据驱动参数化与场景环境；可聚合指标与可编程摘要；内置本地仪表盘与独立 HTML 报告；四类扩展接口；调试输出凭据头遮盖。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#5758](https://github.com/grafana/k6/issues/5758) | Add default subcommand extensions built into k6 | open | 开放提案；子命令扩展机制已存在，但默认捆绑 xk6-docs 与 explore 未核实交付，不能由命令可见性推定全量离线内置。 |
| [#4883](https://github.com/grafana/k6/issues/4883) | Browser support is limited to chromium | open | 开放需求；维护者说明 Chromium 耦合与需求评估，官方安装要求仍为 Chromium，未发现多引擎交付证据。 |
| [#4873](https://github.com/grafana/k6/issues/4873) | Simplify browser testing with k6 | open | 开放的产品方向议题；子项和部分 API 已有进展，不代表整项完成，也不代表兼容所有 Playwright 测试。 |
| [#4803](https://github.com/grafana/k6/issues/4803) | Machine readable end of test summary | closed | 已验证交付核心能力；议题评论计划 v1.4.0，但 v1.5.0 发布记录明确列出 PR #5338 与新摘要开关。不能将议题所有 nice-to-have 或 Cloud 模式的愿景都算交付。 |
| [#5396](https://github.com/grafana/k6/issues/5396) | Proxy management for the prometheus remote write | closed | 已验证交付：议题关联 PR #6403 的状态为 MERGED，v2.4.0 发布记录列出输出代理选项；同版已弃用内置 Prometheus remote-write 输出并计划 v3 移除，不能据此建议长期复制旧输出。 |

建议小 PR：有界的本地集合顺序回归；版本化运行报告和无窗口 CI 入口；可审阅的 HTTP 子集 k6 脚本导出。

### Apache JMeter

JMeter 是面向可重复负载与性能分析的 Java 测试计划引擎，Reva 是面向本地交互调试的 Go/Viva 原生 API 客户端。值得借鉴的是运行模型、数据驱动和可解释的测试结果；多机压测与统计分析更适合通过明确格式边界对接 JMeter。

仓库：https://github.com/apache/jmeter。许可：Apache-2.0。查询：2026-11-16T22:34:32.485862+08:00。

[完整研究与编号来源](api-tools/jmeter.md) · [结构化数据](api-tools/jmeter.json)

已实现特色：多协议采样器；虚拟用户与负载模型；树状流程控制与事务；CSV 数据驱动；响应提取与请求关联；JSR223 脚本扩展；结构化断言；HTTP(S) 录制代理；无界面执行与离线 HTML 分析；远程分布式执行。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#6467](https://github.com/apache/jmeter/issues/6467) | jdbc sampler to run script (multiple statements) | open | 开放提案；本次未核验交付 |
| [#6468](https://github.com/apache/jmeter/issues/6468) | JMeter HTML Report - Provision for Custom Decimal Percentiles Inc | open | 开放提案；本次未核验交付 |
| [#5083](https://github.com/apache/jmeter/issues/5083) | Let's have Plugins Manager added in JMeter by default | open | 开放提案；本次未核验交付 |
| [#6700](https://github.com/apache/jmeter/issues/6700) | Response Assertion: print each pattern in output | open | 开放提案；本次未核验交付 |
| [#4023](https://github.com/apache/jmeter/issues/4023) | Support HTTP/2 protocol | closed | 已核验交付到开发分支；尚无稳定版发布证据 |
| [#5081](https://github.com/apache/jmeter/issues/5081) | HTTP Request: Migrate to HTTPClient 5 APIs | closed | 已核验交付到开发分支；尚无稳定版发布证据 |

建议小 PR：集合串行回归与独立运行变量；受限 CSV 数据驱动回归；向 JMeter 导出严格受限的 HTTP 计划。

### WireMock

WireMock 是本地可运行的开源 API 模拟服务端，强项是精细匹配、动态响应、状态、录制、故障和服务端验证。Reva 已有脚本和多协议客户端能力，适合补上外部模拟集成及 CI 执行出口，优先复用专用引擎而非重写。

仓库：https://github.com/wiremock/wiremock。许可：Apache-2.0。查询：2026-11-16T22:34:29+08:00。

[完整研究与编号来源](api-tools/wiremock.md) · [结构化数据](api-tools/wiremock.json)

已实现特色：结构化请求匹配；JSON Schema 请求匹配；动态响应模板；场景状态机；条件代理与录制回放；请求验证与近似匹配诊断；延迟和传输故障注入；异步回调及子事件观测；嵌入式测试的生命周期与隔离；WebSocket 消息模拟。

| 议题 | 精确标题 | 当前状态 | 交付核实 |
| --- | --- | --- | --- |
| [#3031](https://github.com/wiremock/wiremock/issues/3031) | Support WebSockets | open | 部分已交付（4.x beta）；原议题仍开放，代理转发需求未交付。 |
| [#2684](https://github.com/wiremock/wiremock/issues/2684) | Allow scenario state to be scoped to arbitrary request attributes | open | 开放提案；本次未验证到任意请求属性的场景状态隔离已交付。 |
| [#2529](https://github.com/wiremock/wiremock/issues/2529) | Support JSON Schema definition from external file | open | 开放提案；相邻的内联对象支持已交付，外部文件引用未验证交付。 |
| [#2710](https://github.com/wiremock/wiremock/issues/2710) | Handlebars helper - join | closed | 已验证交付；最终函数名为 arrayJoin，不是提案中的 join。 |
| [#2714](https://github.com/wiremock/wiremock/issues/2714) | Introduce a GUI for open source | closed | 已关闭但未纳入核心；维护者拒绝核心 GUI 范围，不属于已交付或重复议题。 |
| [#2759](https://github.com/wiremock/wiremock/issues/2759) | Report webhook errors and results as sub-events | closed | 已验证交付（稳定标签 3.13.2 存在实现）；本次未追溯最早发布版本。 |

建议小 PR：从选定请求及响应导出一个 WireMock 静态桩；增加无 GUI 的串行集合执行与 CI 退出码；按需读取外部 WireMock 的未匹配请求及近似匹配。

## onboarding 已有成果

setup state、copy、settings UI、tests 四部分已经通过 [PR 12](https://github.com/runbrick/reva/pull/12) 合入当前基线；合并提交 `8b46c3dd55fd053bc7009fadfe4ffa5e2afa6e5c`。本次不重复实现，也不重建已合入 PR。原拆分和验证证据见 [onboarding PR guide](../onboarding-prs.md)。

| 部分 | 基线可达的实现提交 | 依赖 |
| --- | --- | --- |
| setup state | b729698d7d12efa71d7b9a593c7bf45a2283f791 | 无 |
| copy | acc77133f7b883c5443ed926d7552496aced9479 | 无 |
| settings UI | 2346969a597afb3413f19267a8e7c0f86b95d824 | state 与 copy |
| tests | 758a675323c35e4e7cd410f39d584ab1baa4f9ed | UI 与前置两项 |

## 子工作树提交与验证

所有 worker 均报告 gofmt -l . 空输出、go vet ./...、go tool viva vet ./...、go test ./... -count=1 退出 0；完整命令尾部保留在对应项目 Markdown。此次仅文档变更，无新增行为测试。检查宿主实际为 Go 1.27.3 Windows/amd64，go.mod 目标 1.26；不把当前检查描述为 Go 1.26 或 macOS 验证。

| 项目 | 子分支 | 原始提交 |
| --- | --- | --- |
| Bruno | runbrick/research-bruno | a94ae46765559ce7823d92de559703269e870e1b |
| Insomnia | runbrick/research-insomnia | 77353a2ed23590def549cb43e9a88be6b2b435d9 |
| Hoppscotch | runbrick/research-hoppscotch | d18dfe45902c93376b3d6bdd56ad098682ea7037 |
| Yaak | runbrick/research-yaak | 20bcaeed813ecbabc109236f52e74febdc382c41 |
| HTTPie CLI | runbrick/research-httpie | 3e10c2541eba4b37a712c254fc992ee540dbfb57 |
| Hurl | runbrick/research-hurl | abbb285643b705797db993a8aa49925d9b913878 |
| Karate | runbrick/research-karate | 9aff5f37face2f71e394d33e864baf891637b1f8 |
| REST Assured | runbrick/research-rest-assured | 57c601d1d847968179d84cbb575cfaca8461d8c9 |
| Schemathesis | runbrick/research-schemathesis | fbfd00f7c7601a085a30bffc5a7fed543a43b331 |
| Grafana k6 | runbrick/research-k6 | d12d07b5f7f00cacd1c365367a6f0bc52107f3b4 |
| Apache JMeter | runbrick/research-jmeter | 67590a8dcd2f95b7c81505676df102c00ac19575 |
| WireMock | runbrick/research-wiremock | 929c7d644954464a87272ffc668d6d8732970967 |

原始提交在当前汇总分支中通过 cherry-pick 收集，子工作树原提交保持独立。没有推送、没有创建远程 PR、没有合并 main。机器可读溯源见 [研究清单](research-manifest.json)。

## 汇总分支检查与文档验证

汇总工作树再次执行四项仓库检查，全部退出 0。Word 文档经 render_docx.py 和独立临时 LibreOffice 26.8.3 渲染为 31 页，逐页检查中文、表格、分页和页脚；修正后的未变页面以图像哈希核对一致。没有将 PDF 或页面图片作为交付物。

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
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	1.354s
ok  	github.com/runbrick/reva/internal/service/store	0.920s
ok  	github.com/runbrick/reva/internal/storage	0.984s
ok  	github.com/runbrick/reva/internal/syncing	1.492s
ok  	github.com/runbrick/reva/internal/update	1.236s
ok  	github.com/runbrick/reva/internal/vars	0.610s
ok  	github.com/runbrick/reva/internal/websocket	1.033s
Exit: 0
```
