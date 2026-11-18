# EasyPostman 对 Reva 的研究：从首个请求到可重放工作区

> 补充调研；不计入原 12 项基线。抓取窗口：`2026-11-18T06:55:57Z` 至 `2026-11-18T07:02:38Z`（本机 UTC）；主快照 `retrieved_at` 为窗口起点。Reva 比较基线：v0.3.0 `ac988d92424de2a238baa90395ebce8d88f33ecf`。分类：`desktop-client`。[结构化记录](easy-postman.json)

## 定位、版本和许可

[lakernote/easy-postman][repo] 把 Postman 风格的本地 API 调试、功能回归和 JMeter 风格的压测放在一个 Java 桌面应用里。桌面是 Swing，传输用 OkHttp，脚本用 GraalJS，Git 工作区用 JGit，命令行复用工作区和执行服务；Mock、压测核心与 MCP 协议适配分别拆分。普通请求不用账号或 Git 远程。官方原生安装包与 Windows 便携包附带运行时，独立 JAR 需要 Java 17+；打包说明不能代替实测，也不把安装包称为无 JVM 的原生二进制。[中文说明][readme] · [模块文档][arch] · [发布资产][release]

- [仓库 API][meta] 报告 `default_branch: master`，`git ls-remote --symref … HEAD` 确认为 `refs/heads/master` → `db1d014dc1d9897af1998e00fea5a794776bb2c1`。[最新发布 API][latest] 返回稳定版 **v6.3.3**，发布时间 `2026-11-14T11:26:23Z`；[发布订阅][feed] 的首条应用版本也为 v6.3.3。标签指向 `ed0baa04a477b9cc1db6f20d66c623b573ecde6d`；`target_commitish: master` 不等于目前的 `master` 提交。下文除明确标注外以 v6.3.3 的固定提交为证。
- [提交比较][compare] 显示 `master` 比 v6.3.3 超前 2、落后 0。未随该版本发布的两项是 `604b070` 的表单 `+` 解码修正和 `db1d014` 的 Bash 单引号内保留 `^`；后者可在当前 [cURL 词法器][master-curl] 直接看到。它们是解析修复，不是新的导入格式。
- GitHub 仓库元数据给出 SPDX `Apache-2.0`，与 v6.3.3 及 `master` 根 `LICENSE` 文本的 **Apache License, Version 2.0** 一致。该文件尾部的版权占位文字属于标准附录，不是额外许可限制。这里仅确认公开仓库源码是 Apache-2.0（OSI 开源）；未审计随包 JRE、第三方插件、商标或再分发义务。[标签许可][license] · [开发分支许可][master-license]

## 十项已实现能力

| # | EasyPostman v6.3.3 固定源码所见 | 边界 |
| --- | --- | --- |
| 1 | **请求编辑器空态**：`RequestEditorEmptyStatePanel` 的点击与键盘激活调用新建请求回调，另展示操作快捷键；`RequestCurlImportController` 提供显式剪贴板 cURL 解析入口。[空态][empty] · [cURL 控制器][paste] | 嵌入真实工作区，不是独立教程；没有证据表明新手必须登录、自动发示例请求或完成引导才可操作。 |
| 2 | **迁移入口与信息保留**：集合工具栏接线了 Postman、cURL、HAR、Swagger/OpenAPI 和 ApiPost 等导入；cURL 对话框可拆多条命令。Postman parser 保留集合/文件夹前后脚本及保存响应；ApiPost 专用 parser 会解析所支持的导出结构和 `global.envs`，并从首个非空服务器 URI 生成 `baseUrl`。[导入菜单][import] · [Postman parser][postman] · [ApiPost parser][apipost] | `FEATURES_zh.md` 把 Swagger 标为开发中，但标签中的入口实际调用 JSON parser，明确分派 Swagger 2、OpenAPI 3.0/3.1。只认模板导入；不推断 YAML、完整迁移、批量补写所有 URL、schema 验证或契约生成已交付。[功能说明][features] · [Swagger parser][swagger] |
| 3 | **HTTP、增量 SSE 和 WebSocket**：官方列 HTTP/1.1、HTTP/2；SSE 有逐事件回调及事件表，阻塞路径也解析 `data/event/id/retry`；WebSocket 执行器接收文本和二进制消息。[SSE 解析器][sse] · [事件面板][sse-ui] · [WebSocket 执行器][ws] | SSE 经 HTTP 传输，不是独立请求枚举。请求枚举和文档未找到原生 gRPC 或 GraphQL 模式；发送 GraphQL JSON 不等于专用客户端。[协议模型][protocol] |
| 4 | **认证、会话与证书**：模型列继承、None、API key、Basic、Bearer、Digest；文档公开 Cookie 管理、脚本 Cookie Jar 和按主机配置客户端证书。[认证类型][auth] · [脚本 API][scripts] · [证书][certs] | Digest 与客户端证书补齐企业调试场景，但不等于 OAuth2 获取、刷新及 PKCE 流程；未找到后者的实现证据。 |
| 5 | **多作用域变量和脚本链**：`VariableResolver` 优先级排序变量提供者，最多展开 10 轮、内容不变即停止，不是任意深度或逐节点的完整环诊断。执行器用 GraalJS；脚本支持 `pm.test`、响应变量传递以及回调式 `pm.sendRequest`，工作区运行上下文携带迭代数据。[变量解析][vars] · [GraalJS][js] · [执行上下文][executor] | 部分 Postman API（如 `pm.vault`、`pm.execution`）尚缺，保留脚本不保证可执行。宿主类查找关闭，但 Context 对注入对象允许 `HostAccess.ALL`，且有明确的网络桥接；不可按 Reva 的脚本边界处置迁移脚本。[脚本边界][js-pool] · [兼容表][scripts] |
| 6 | **桌面功能测试与两种 CI 入口**：GUI 有功能测试与断言结果；`collection run` 按集合/文件夹选择，`functional run` 按 `functional_config.json` 指定选择与顺序；两者共用工作区执行器，可用 CSV/JSON 迭代、遇错停止并写 JSON 报告。[界面文档][features] · [集合 CLI][collection-cli] · [功能 CLI][functional-cli] · [报告模型][report] | 报告可含工作区/文件路径、请求名称、方法、URL、错误与断言明细；不要将它当成可公开的脱敏报表。未核实 JUnit 输出，也不把专用压测运行器算作集合 CLI 的默认行为。 |
| 7 | **本地/Git 工作区与 WebDAV 快照**：工作区文件和位置有明确模型；Git 支持克隆、拉取、提交、推送和 diff；可选 WebDAV 定时上传快照，对较新或无法判定时间的远端快照有跳过分支。[目录][storage] · [Git 实现][git] · [WebDAV 调度][dav] | Git 是按文件协作，WebDAV 是跨设备快照备份/恢复，不是多用户实时合编。即使文档提到加密，也未据此认定所有 Git 文件、WebDAV 快照和本地值都加密或可直接分享。 |
| 8 | **限定输出面上的脱敏**：MCP 适配层的 `McpSensitiveDataRedactor` 处理 URL user-info、常见敏感 query/认证字样、部分响应键值、错误文件路径及已知敏感值；环境列表不给变量值，响应有体积和明细上限。[MCP 文档][mcp-doc] · [脱敏器][mcp-mask] · [工作区适配][mcp-workspace] | 名称/模式和已知值不是 DLP、业务数据识别或凭据保险库；这不是普通 CLI 的 `WorkspaceRunReport` 输出契约。本报告只核实数据边界，不建议为 Reva 接入自动代发。 |
| 9 | **保存响应和本地 Mock**：`SavedResponse` 可关联原请求快照；`MockServerManager` 组合集合 Example 和独立路由，以 JDK `HttpServer` 匹配并提供静态/脚本响应、延迟及有界日志；有独立 `mock run`。[快照模型][saved] · [Mock 运行时][mock] · [Mock CLI][mock-cli] | 有真运行时，不只是示例展示；工作区可自托管，不能视为托管云 Mock 或完整团队访问控制。保存示例的原请求与响应都可能含秘密。 |
| 10 | **专用本地及多 worker 压测**：线程组引入固定、阶梯、爬坡和尖峰等负载计划，和桌面交互耗时是不同产品面；有 `performance run`、worker 服务、报表及趋势聚合。[线程组][plan] · [压测 CLI][perf-cli] · [worker 源码][worker] · [集群文档][cluster] | 官方明确目前聚合各 worker 报告，非全局原始样本 P99；没有吞吐、内存或所有协议的实测结果。Swagger 导入也不是性能或契约覆盖证明。 |

## 与 Reva v0.3.0 的等价行为对照

| 关注面 | Reva 已交付（以基线源码为准） | 仍有差异 |
| --- | --- | --- |
| 首次请求与保存 | [`App.NewTab`](../../../internal/gui/app.go) 启动即有请求草稿；[`onboardingEntry` / `onboardingDialog`](../../../internal/gui/onboarding.go) 是可跳过、可重播的设置指南，完成标记可落盘；[`RequestSave`](../../../internal/gui/collections.go) 提供显式保存。 | 不需要仿造空白工作区；可把导入/手动发送/保存发现性做成已有首练工作的验收。研究不能给出两产品可用性或点击数胜负。 |
| 导入与迁移 | [`ParseWithOptions`](../../../internal/importer/result.go)、[`reviewImport` / `performImport`](../../../internal/gui/importer.go) 接受原生、Postman 2.0/2.1、单独环境、HAR、单条 cURL、Swagger 2/OpenAPI 3 **JSON**（含显式 URL），有兼容性确认与有界解析。 | ApiPost/Apifox 走 Postman 或 OpenAPI 导出，不认专有项目包；Postman 脚本/保存示例提示后省略，未知认证不静默降级。对齐 EasyPostman 的实际价值是保留环境关联，不是取消确认或执行导入脚本。 |
| 认证、变量及脚本 | [`auth.Resolve`](../../../internal/auth/auth.go) 实现集合/最近文件夹认证继承及 None，支持 Bearer/Basic/header 或 query API key；[`VarsFor`](../../../internal/gui/env.go)、[`vars.Expand`](../../../internal/vars/vars.go) 体现环境覆盖集合、禁用回退和单次展开；[`Plan.Run`](../../../internal/execution/execution.go) 与 [`RunPre` / `RunTests`](../../../internal/script/script.go) 复用限时 Otto/ES5 和有限 `pm.*`。 | 无 Digest、OAuth2 生命周期、客户端证书、持久 Cookie Jar（[`client` / `responseCookies`](../../../internal/service/request/options.go) 只解析展示）以及任意网络脚本；脚本变量修改不跨请求。S3 签名不等于请求编辑器认证。 |
| HTTP、协议、SSE | [`Send`](../../../internal/service/request/request.go) 有 HTTP 响应、体积保留上限和网络时间线；[`graphQLBodySection`](../../../internal/gui/graphql.go) 编辑 query/variables 经 HTTP 请求发送；[`Dial` / `ReadMessage`](../../../internal/websocket/client.go) 支持 RFC6455。 | HTTP 实现读完才返回，不提供逐条 SSE；原生 gRPC 未找到；Reva 的 GraphQL 编辑体验不应误记为 EasyPostman 的差异优势。 |
| 回归与报告 | [`cmd/reva-run:run`](../../../cmd/reva-run/main.go) 和 [`runner.Run`](../../../internal/runner/runner.go) 已有原生集合确定性串行运行、显式环境、可取消及遇错停止。版本化 [`Result.Report`](../../../internal/runner/report.go) 只输出序号和允许的状态、时间及计数，省略名字、URL、脚本/断言文本和原始错误。 | 缺功能测试 GUI 编排、CSV/JSON 迭代、选择任意运行顺序及跨请求变量传递。CLI 面向可信原生集合，按 [README](../../../README.md) 可从源码构建，桌面发版流程列的是桌面资产。 |
| 持久化、共享和脱敏 | [`REVA_HOME` 数据布局](../../../README.md) 是逐集合/环境 JSON；[`Seal` / `Shared` / `Merge`](../../../internal/syncing/sync.go) 与 [`makeRemote` / `scheduleAutoSync`](../../../internal/gui/sync.go) 已实现加密快照及 WebDAV/S3/Gist 后端、可选自动同步。 [`preview.Build`](../../../internal/preview/preview.go) 与 [`requestPreview`](../../../internal/gui/preview.go) 已提供脱敏、只读、脚本前的变量胜出/来源、未知变量、URL 及认证来源预览。 | 预览不是线上报文或全应用 DLP；本地 JSON 和原生导出仍可含凭据，加密同步不等于本地凭据保险库。没有 EasyPostman 式工作区内置 Git 操作；Reva 的 CLI 最小化报告与 MCP 的模式脱敏是不同承诺。 |
| 示例、Mock、压测与 schema | [`responseResult` / `responseCopy`](../../../internal/gui/response.go) 提供响应查看/复制；[`HistoryEntry`](../../../internal/model/model.go) 记录请求与结果元数据，不持久化整个响应正文；[`openapi`](../../../internal/importer/openapi.go) 把规格转成请求模板。 | 没有 Example 资产/Mock 服务器、负载模型和集群报告；时间线不等于压测，导入或能解析 JSON 不等于运行时契约校验。测试替身不是产品服务。 |

## 六条真实功能议题

从 GitHub Issues 的 `state=all` 列表默认排序前 100 条抽样，排除 PR，再核查条目、维护者回复、标签树或发行说明。`closed/completed` 是 GitHub 状态，不自动证明特定交付；“已发行”表示有发布说明或现行稳定标签源码，未运行资产。

| 议题 | 状态与需求 | 交付判断 |
| --- | --- | --- |
| [#95][i95] | **closed/completed**：ApiPost 迁移时同时带入环境并给路径加 `{{baseUrl}}`。 | **部分已发行**：维护者指向 v4.3.14；v6.3.3 的 [ApiPost parser][apipost] 证实导入环境和 `baseUrl`，但请求 URL 直接取源 `url`，未确认批量重写所有路径。 |
| [#97][i97] | **open / 提案**：GUI 保存多套 JVM 内存配置并在切换后重启。 | 未找到交付证据；构建文档的手动 JVM 参数不等价。这个诉求是 JVM/重负载的运维摩擦，不可直接外推到 Reva。 |
| [#100][i100] | **closed/completed**：希望支持嵌套变量、环/深度限制和实际值提示。 | **核心已发行**：维护者指向 v4.3.21；v6.3.3 [实现][vars] 确认多轮、上限和稳定值停机，不能据此证明完整环诊断或全部悬浮提示。 |
| [#136][i136] | **closed/completed**：用自定义 WebDAV 跨设备同步配置。 | **已发行**：[v6.0.8][r608] 说明明确新增上传/恢复快照，提报者确认使用；非实时协作。 |
| [#152][i152] | **closed/completed**：根据请求条件返回不同的 mock 响应，并能用脚本。 | **已发行**：[v6.0.19][r6019] 列出静态/Code Mock、匹配、延迟及会话状态；[标签运行时][mock] 可核对。 |
| [#156][i156] | **closed/completed**：为 `/xxx/{id}` 增加独立的可编辑路径参数。 | **关闭但交付未核实**：评论无维护者发布确认；现有通用 `{{…}}` 替换不等价于独立路径参数编辑字段。未据此断言缺失。 |

## 三个可独立评审的后续增量

1. **P1：导入集合后接续导入环境。** 先做 Postman 集合审查结果上的「导入另一个环境文件」入口：用现有单实体导入两次确认，显示缺失变量的名称与当前准备状态；不自动激活环境，不改写 URL，不将值写回集合。验收：离线 fixture 导入前无写入/网络/脚本，确认集合后导入独立环境，退出、冲突、取消与重试不误激活；用户选用环境并在预览确认作用域后才手动发送，发送行为用本地 `httptest`。维护代价是两步状态和中英提示；避免在首版承担专有格式和跨文件原子事务。有足够多的实际 ApiPost 原生迁移样本再单独做格式适配。类比数据库迁移的预检和显式 apply：二阶减少用户手抄 `baseUrl`/令牌，三阶避免默默绑定业务环境，未来新增导入器能共用同一验收标准。
2. **P1：桌面只读运行摘要。** 对已保存的集合和用户明确选择（或不选）的环境复用 `runner.Run`，同一工作流显示启动、取消以及安全的序号/结果/断言计数；不放出请求、脚本或任意错误文本，不先做 CSV、并行或报告设计器。验收：GUI Driver 与本地服务器验证显式启动、确定顺序、遇错停止和取消、无历史/环境副作用，同一个 fixture 与 `reva-run` 状态和计数一致，截图/输出无秘密。维护代价是 Effect 生命周期和序号映射；保留 CLI v1 schema。类比编辑器运行单测并读 CI 的同一结果：二阶减少「桌面能过，CI 不过」的语义漂移，三阶使后续 CI 格式适配建立在稳定、最小化的结果上。
3. **P1：有界交互式 SSE 检视。** 只为实际 `text/event-stream` 响应增加事件视图，沿用显式发送、认证和取消，按事件数/字节数有界，支持断开但不自动重连；不承诺更改 CLI、WebSocket 或保存格式。验收：`httptest` 分批 flush，首条在连接关闭前出现；覆盖 CRLF、多行 data、注释、超限和主动取消，普通 HTTP 与下载不回归。维护成本是传输到 UI 的背压及边界测试；类比受限日志 tail：二阶能区分首事件延迟和连接总耗时，三阶避免把长连接事件误当普通响应永久归档或纳入无界报告。

**不改变当前 onboarding 的优先顺序。** EasyPostman 的空态用操作本身提示下一步，支持已有的「可跳过、可重播、低摩擦首练」方向；在进行中的工作验证用户能从草稿或导入走到显式发送、看响应、保存即可。不要为了追平对方引入启动向导门槛、账号、自动访问示例站、Git、压测或 MCP。上述三个增量均在首练之后独立评审，不替代现有 onboarding PR。

**继续交给外部工具：** 大并发、分布式负载和可比百分位用 k6/JMeter；共享或有状态 Mock 用 WireMock，运行时 schema/契约覆盖用 Schemathesis 等；完整 Postman 脚本兼容、大批迁移、Git 合并及跨项目权限不要变成桌面核心职责。可先提供可审查的、明确过一遍脱敏边界的手动格式交接，不自动执行第三方程序或发出网络请求。

## 取证限制

- 仅用 GitHub API、发布记录、固定 raw 源码与项目自带文档；部分公开 API 限流或超时后改用固定 raw/发布订阅。没有安装、构建或运行 EasyPostman，也没有用互联网目标做请求测试；未验证签名、第三方依赖许可证、包体、平台外观和实际吞吐。源码存在、发布标签存在与逐包行为验收是不同证据。
- 需求是六条定性样本，不代表全量路线图、需求频率或首次发行考古；缺失判断受当时可见的模型、入口、文档和固定代码范围约束。“已发布”不暗示所有语言、平台或版本的完整兼容。
- 本研究只读 Reva v0.3.0 代码，不推断其他分支；本工作树未找到 `TASK.md`、`.claude/skills/building-with-viva/SKILL.md` 或 `docs/` 至本目录的 `AGENTS.md`，不涉及 Viva/Go 实现变更。

[repo]: https://github.com/lakernote/easy-postman
[meta]: https://api.github.com/repos/lakernote/easy-postman
[latest]: https://api.github.com/repos/lakernote/easy-postman/releases/latest
[release]: https://github.com/lakernote/easy-postman/releases/tag/v6.3.3
[feed]: https://github.com/lakernote/easy-postman/releases.atom
[compare]: https://github.com/lakernote/easy-postman/compare/v6.3.3...db1d014dc1d9897af1998e00fea5a794776bb2c1
[license]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/LICENSE
[master-license]: https://github.com/lakernote/easy-postman/blob/db1d014dc1d9897af1998e00fea5a794776bb2c1/LICENSE
[master-curl]: https://github.com/lakernote/easy-postman/blob/db1d014dc1d9897af1998e00fea5a794776bb2c1/easy-postman-app/src/main/java/com/laker/postman/service/curl/CurlCommandTokenizer.java
[readme]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/README_zh.md
[arch]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/ARCHITECTURE_MODULES_zh.md
[features]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/FEATURES_zh.md
[empty]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/panel/collections/editor/RequestEditorEmptyStatePanel.java
[paste]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/panel/collections/editor/request/RequestCurlImportController.java
[import]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/panel/collections/tree/CollectionTreeToolbar.java
[postman]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-collection-core/src/main/java/com/laker/postman/collection/importer/postman/PostmanCollectionParser.java
[apipost]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/apipost/ApiPostCollectionParser.java
[swagger]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/swagger/SwaggerParser.java
[sse]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-http-runtime/src/main/java/com/laker/postman/http/runtime/sse/SseEventReader.java
[sse-ui]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/panel/collections/editor/request/sub/SSEResponsePanel.java
[ws]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/panel/collections/editor/request/WebSocketRequestExecutor.java
[protocol]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-request-core/src/main/java/com/laker/postman/request/model/RequestItemProtocolEnum.java
[auth]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-request-core/src/main/java/com/laker/postman/request/model/RequestAuthTypes.java
[certs]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/CLIENT_CERTIFICATES.md
[scripts]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/SCRIPT_API_REFERENCE_zh.md
[vars]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/variable/VariableResolver.java
[js]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/js/JsScriptExecutor.java
[js-pool]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/js/JsContextPool.java
[executor]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/workspace/cli/WorkspaceRunExecutor.java
[collection-cli]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/COLLECTION_CLI_zh.md
[functional-cli]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/FUNCTIONAL_CLI_zh.md
[report]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/workspace/cli/WorkspaceRunReport.java
[storage]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/util/WorkspaceStorageUtil.java
[git]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/plugin/git/GitWorkspacePluginService.java
[dav]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/service/sync/WebDavSyncScheduler.java
[mcp-doc]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/MCP_SERVER_DESIGN_zh.md
[mcp-mask]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/mcp/McpSensitiveDataRedactor.java
[mcp-workspace]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/mcp/McpWorkspaceToolService.java
[saved]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-request-core/src/main/java/com/laker/postman/request/model/SavedResponse.java
[mock]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-mock-core/src/main/java/com/laker/postman/mock/runtime/LocalMockServer.java
[mock-cli]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/mock/cli/MockRunCliCommand.java
[plan]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-performance-core/src/main/java/com/laker/postman/performance/core/threadgroup/PerformanceCoreThreadGroupPlanner.java
[perf-cli]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/performance/cli/PerformanceRunCliCommand.java
[worker]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/easy-postman-app/src/main/java/com/laker/postman/performance/worker/PerformanceWorkerServer.java
[cluster]: https://github.com/lakernote/easy-postman/blob/ed0baa04a477b9cc1db6f20d66c623b573ecde6d/docs/PERFORMANCE_CLUSTER_LOAD_TEST_zh.md
[i95]: https://github.com/lakernote/easy-postman/issues/95
[i97]: https://github.com/lakernote/easy-postman/issues/97
[i100]: https://github.com/lakernote/easy-postman/issues/100
[i136]: https://github.com/lakernote/easy-postman/issues/136
[i152]: https://github.com/lakernote/easy-postman/issues/152
[i156]: https://github.com/lakernote/easy-postman/issues/156
[r608]: https://github.com/lakernote/easy-postman/releases/tag/v6.0.8
[r6019]: https://github.com/lakernote/easy-postman/releases/tag/v6.0.19
