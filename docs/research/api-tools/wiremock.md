# WireMock：用专用模拟服务补齐 Reva 的联调与自动化出口

- 研究基线：Reva `72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`（开始时由 `git rev-parse HEAD` 获取）。
- 检索完成时间：`2026-11-16T22:34:29+08:00`；观测日期为 2026-11-16，保留本机实际 UTC+08:00 偏移。
- 范围：`wiremock/wiremock` 社区核心；稳定版 **3.13.2** 与 **4.x beta** 分开判断。类别：`mock`。
- 交付：仅本页及同目录 `wiremock.json`；工作分支 `runbrick/research-wiremock`，无源码修改、无新增行为测试。

## 结论与定位

WireMock 是本地可运行的开源 API 模拟服务端，强项是精细匹配、动态响应、状态、录制、故障和服务端验证。Reva 已有脚本和多协议客户端能力，适合补上外部模拟集成及 CI 执行出口，优先复用专用引擎而非重写。

Reva 主动发请求并查看结果，WireMock 接收请求并给出可控制的结果，两者互补。把社区核心、第三方扩展及商业 Cloud 混在一起，会高估“单一开源项目”的覆盖范围。最有价值的交集是：将 Reva 中真实调试成果变成可版本管理的本地桩，再让同一组请求和断言进入 CI。

## 仓库、许可证与商业边界

规范仓库为 [wiremock/wiremock](https://github.com/wiremock/wiremock)，默认分支 `master`；旧 `tomakehurst/wiremock` 链接不应当作另一个项目。本次 GitHub 元数据及固定标签 LICENSE 均确认 **Apache-2.0**，属于 OSI 开源许可证。[1][3][58]

许可结论限定于社区核心，不能自动覆盖 WireMock Cloud 商业服务、外部扩展或所有传递依赖。本次未发现核心采用 source-available 限制许可的证据，也没有把仅可见源码的许可等同于 OSI 开源。

| 边界 | 本次核验结论 |
| --- | --- |
| 社区核心 | 精细请求匹配、JSON Schema 请求匹配、Handlebars、场景状态机、录制、故障、请求验证和 Java/JSON 管理接口均已实现。[2][8–17] |
| 商业 WireMock Cloud | 厂商比较页列出托管共享目录、版本历史、SSO/角色/审计、CSV/数据库数据源、OpenAPI 生成与持续契约验证等产品能力，不归入核心 Apache-2.0 范围。[21] |
| 状态与验证的区别 | 比较页的营销概述不能据以说“社区没有状态/验证”：共享场景状态机、请求 JSON Schema 匹配已在社区实现，与 Cloud 的高级状态及 OpenAPI 响应契约验证有别。[8][10][21][27] |
| 扩展及外部 GUI | gRPC、GraphQL 专门支持和第三方 GUI 需分别看扩展与许可证；维护者明确拒绝把 GUI 纳入核心，不等于没有外部 GUI。[20][21][33] |

本次 `releases/latest` 返回 3.13.2，发布日期 2025-11-14；发布列表首项为 4.0.0-beta.43。这不能写成“最新发布只有 3.13.2”，也不能把 beta 功能倒填进稳定版。WebSocket 官方页明确写明仅限 4.x beta。[4][22][24][59][60]

## 架构与本地工作方式

稳定版 3.13.2 是 Java/Jetty 服务端，Java DSL、JSON 映射文件和 /__admin REST 共同管理桩；mappings 与 __files 可在本地版本管理。请求经匹配、优先级和场景选中响应，由静态体、模板、代理及回调扩展处理，并保留可验证的服务事件；可以作为 JUnit 嵌入实例、独立 JAR 或 Docker 运行。WebSocket 消息框架属于已实现的 4.x beta，版本边界单独标记。

3.13.2 的 `build.gradle` 以 Java 11 为编译目标，依赖 Jetty 11.0.26、Handlebars 和 networknt JSON Schema 验证器；此信息不代表 4.x 的运行要求。`WireMockServer` 管理启停和管理端操作。普通部署将映射放入 `mappings/`、响应文件放入 `__files/`，可以随代码进入 Git。[5–7][18–20][27]

请求按条件、优先级及场景状态选桩，响应由静态体、模板或代理提供，服务事件用于验证和诊断，处理后可异步触发 webhook。JUnit 扩展负责测试进程内生命周期；JAR/Docker 让 Go 等其他语言消费独立服务。社区自托管不需要 Cloud 账号，管理端暴露、录制内容和日志由使用者管理。[6][11–19]

## 十项已经实现的特色能力

以下不是提案清单。除第 10 项明确限于 beta 外，其余按社区稳定版范围描述；源码固定到 3.13.2，在线文档可能继续更新。

| 能力 | 交付范围与实际价值 | 首要证据 |
| --- | --- | --- |
| 1. 结构化请求匹配 | **已实现（社区稳定版）**。按 URL/路径、方法、查询参数、头、Cookie、表单和 multipart 匹配；提供 JSON 等价/JSONPath、XML/XPath、逻辑组合及 Basic 认证匹配，用于决定哪个桩接收请求。 | [8](#证据来源) |
| 2. JSON Schema 请求匹配 | **已实现（社区稳定版）**。matchesJsonSchema 对入站请求作约束匹配；文档列出 V4/V6/V7/V201909/V202012，默认 V202012。社区已有这项验证，但它不等于按 OpenAPI 持续验证全部响应。 | [27](#证据来源) |
| 3. 动态响应模板 | **已实现（社区稳定版）**。Handlebars 可以引用请求路径、头和 JSON 内容，组合日期、随机值与条件输出；arrayJoin 帮助输出没有多余逗号的数组。这是服务端模板，不是客户端 pm 脚本。 | [9](#证据来源) |
| 4. 场景状态机 | **已实现（社区稳定版）**。scenarioName、requiredScenarioState、newScenarioState 把多次请求组成状态转换，可经管理接口重置。状态按场景共享，不能把开放中的按请求属性隔离提案算作已有能力。 | [10](#证据来源) |
| 5. 条件代理与录制回放 | **已实现（社区稳定版）**。可让选定请求走真实上游，其余请求由桩替代；录制与快照能过滤流量、选择捕获头、生成映射及响应文件，再脱离上游回放。它不同于只抽取 HAR 请求。 | [12](#证据来源) |
| 6. 请求验证与近似匹配诊断 | **已实现（社区稳定版）**。请求日志支持按条件检索、计数、删除和验证调用次数；未匹配请求及 near misses 用于查清实际输入与期望规则的差异。 | [13](#证据来源) |
| 7. 延迟和传输故障注入 | **已实现（社区稳定版）**。固定延迟、均匀/对数正态随机延迟、分块慢响应和异常连接/响应，可测试超时、重试及降级。文档提示 CONNECTION_RESET_BY_PEER 在 Windows 上不可靠，不能作为跨平台确定性验收。 | [14](#证据来源) |
| 8. 异步回调及子事件观测 | **已实现（社区稳定版）**。Webhook 可在处理请求后向另一地址回调；固定标签 3.13.2 实现会追加 WEBHOOK_REQUEST、WEBHOOK_RESPONSE 及错误子事件，避免模板失败和下游返回不可见。 | [16](#证据来源) |
| 9. 嵌入式测试的生命周期与隔离 | **已实现（社区稳定版）**。JUnit Jupiter 扩展管理启停、动态端口、每次测试重置和未匹配请求失败；JAR/Docker 让非 Java 项目在 CI 中消费同一套映射。它是测试依赖服务，不是客户端集合运行器。 | [17](#证据来源) |
| 10. WebSocket 消息模拟 | **已实现（仅 4.x beta，不属于 3.13.2）**。PR #3286 已合并并有明确 beta 文档：消息触发、同通道回复、定向广播、消息验证及管理接口推送。#3031 仍开放，2026-09 维护者回复说明代理转发尚未实现，不能写成完全不支持或已完整透传。 | [22](#证据来源) |

## 对照当前 Reva：以源码而非 README 缺省推断

已阅读 README、AGENTS 和 `docs/onboarding-prs.md`。任务说明已明确 TASK.md 及本地 Viva skill 缺失，本次不改 GUI。以下证据锁定开始时的 Reva commit；客户端和模拟服务端按工作流价值对照，不作同类产品打分。

### 1. 定位与架构

**WireMock：**Java/Jetty 桩服务端，通过 Java DSL、JSON 映射、响应文件和 /__admin REST 管理，可嵌入测试、独立 JAR 或 Docker 部署。3.13.2 编译目标 Java 11、依赖 Jetty 11，不能外推到全部 4.x。

**Reva：**Go/Viva 原生桌面主动客户端，GUI 经服务接口调用 Go 实现，以 ui.Effect 发送网络请求；本地无需账号。 源码：[49](#证据来源)、[42](#证据来源)、[51](#证据来源)。

**取舍：**保留原生本地方向，把模拟引擎作为用户自备外部服务。

### 2. 协议

**WireMock：**社区 3.x 以 HTTP(S)/SOAP 为基础；GraphQL/gRPC 的专用支持通过扩展，不应当作核心编辑器。4.x beta 已实现 WebSocket 模拟，代理转发仍有限。

**Reva：**已实现 HTTP(S)、GraphQL query/variables 编辑及 JSON POST、ws/wss 客户端文本发送和消息读取。检视入口及实现未找到 gRPC 客户端或 GraphQL introspection。 源码：[35](#证据来源)、[36](#证据来源)、[37](#证据来源)、[55](#证据来源)。

**取舍：**把现有客户端协议连到本地桩，不为追逐列表新增协议栈。

### 3. 鉴权

**WireMock：**Basic 以及任意头、Cookie、查询条件可匹配认证输入，不等于代用户完成 OAuth2/OIDC 令牌生命周期或运行身份服务。

**Reva：**Bearer/Basic/头或查询 API Key；集合/文件夹继承、None 阻断、显式头优先，跨源重定向剥离凭证。无 OAuth2 生命周期；WebSocket 可手填认证头。 源码：[38](#证据来源)、[54](#证据来源)、[35](#证据来源)、[37](#证据来源)。

**取舍：**导出桩时默认不固化凭证，匹配认证要求需单独选择。

### 4. 变量与脚本

**WireMock：**服务端 Handlebars 模板及 Java 扩展生成响应，场景状态可跨请求保留；不是客户端 JavaScript 的 pm API。

**Reva：**集合变量后叠加活动环境，后定义启用项优先；{{name}} 未知变量保留。Otto 执行前置脚本及响应 pm.test/pm.expect，默认 2 秒；变量使用本次发送副本，不回写持久环境，异步测试不支持。 源码：[39](#证据来源)、[56](#证据来源)、[40](#证据来源)、[41](#证据来源)。

**取舍：**集合执行器明确变量作用域，并复用现有脚本运行时。

### 5. 运行器与 CI

**WireMock：**JUnit 扩展自动启停和重置；JAR/容器在 CI 提供可复现依赖。厂商 WireMock Runner 属于 Cloud 产品链，不能与社区 JAR 混称。

**Reva：**App.Send 只发送单标签请求；cmd 只有桌面 reva 入口。底层发送服务可调用，但未找到集合遍历、CLI 执行或机器测试报告；仓库自身 CI 已测试并打包应用。 源码：[42](#证据来源)、[51](#证据来源)、[49](#证据来源)、[50](#证据来源)。

**取舍：**增加免 GUI 的串行集合执行入口和失败退出码。

### 6. 导入与导出

**WireMock：**核心使用映射 JSON、__files、管理 API、代理录制和快照；Cloud 的 OpenAPI 自动建桩和持续契约验证工作流不在核心中。

**Reva：**已有原生集合/环境、Postman v2、HAR 请求、curl、Swagger 2/OpenAPI 3 JSON 导入预览；collections.go 导出原生集合 JSON，备份另有加密封装；未找到 WireMock 映射互转。 源码：[43](#证据来源)、[44](#证据来源)、[45](#证据来源)、[48](#证据来源)。

**取舍：**先把用户选定的一次请求和响应导出为确定性的静态映射。

### 7. 模拟与状态

**WireMock：**入站桩、优先级、状态转换、故障、回调和条件代理已实现；按请求属性分区的场景状态仍有开放提案。

**Reva：**生产服务契约只有发送、集合、环境、历史和设置；检视入口未找到监听端口或桩规则服务。历史保留请求、状态和耗时，不是完整响应录制服务。 源码：[51](#证据来源)、[53](#证据来源)、[42](#证据来源)。

**取舍：**导出和诊断外部 WireMock；复杂状态、代理和回调交给专用引擎。

### 8. Schema 验证

**WireMock：**社区核心已能按 JSON Schema 验证请求以选桩；Cloud 的 OpenAPI 响应契约验证和漂移检测属于另一层产品能力。

**Reva：**OpenAPI 导入生成请求模板和示例，没有转换响应定义为运行时约束；pm.expect/JSON.parse 可作手写断言，未发现 JSON Schema 验证器。 源码：[44](#证据来源)、[41](#证据来源)、[40](#证据来源)。

**取舍：**先向配置了 Schema 匹配的本地服务发送请求，不把导入成功当契约测试通过。

### 9. 性能与诊断

**WireMock：**可作为负载测试的依赖，配置线程/异步响应及关闭或限制请求日志；延迟故障用于测试韧性，本身不是负载生成器。

**Reva：**HTTP 连接池、超时和取消、默认保留最多 10 MiB 响应，已有 DNS/连接/TLS/下载等阶段计时；未发现负载调度或吞吐分位数报告。 源码：[35](#证据来源)、[57](#证据来源)、[52](#证据来源)。

**取舍：**保留交互诊断，压测由专门负载工具驱动模拟服务。

### 10. 协作与本地数据

**WireMock：**社区映射和文件可进 Git，自托管不需要 Cloud 账号；Cloud 共享目录、SSO、角色和审计是商业产品能力，不随核心许可证自动开放。

**Reva：**本地缩进 JSON、REVA_HOME 和原子写入；已有可选 AES-GCM 加密备份、WebDAV/S3/GitHub/Gitee 远端及三方合并；未见云端角色/审计服务。 源码：[46](#证据来源)、[47](#证据来源)、[48](#证据来源)。

**取舍：**先让映射进入用户已有目录/Git，不新增托管账号和协作平台。

## 六个真实功能议题：状态与交付分开核验

取样方式：仅一次 `state=all&labels=enhancement&per_page=100` 获取样本，排除 PR 后选六项，再逐页读取正文/讨论。结果为 **3 open、3 closed**，标签均为 `enhancement`；这是定性样本，不是完整路线图。以下保留英文原题，更新时间是 GitHub 元数据值，不是发帖时间。[28]

| 议题（原题） | 状态 / 标签 | 最后更新（UTC） | 交付判断 |
| --- | --- | --- | --- |
| [#3031 Support WebSockets](https://github.com/wiremock/wiremock/issues/3031) | `open` / `enhancement` | `2026-09-15T14:17:07Z` | 部分已交付（4.x beta）；原议题仍开放，代理转发需求未交付。 |
| [#2684 Allow scenario state to be scoped to arbitrary request attributes](https://github.com/wiremock/wiremock/issues/2684) | `open` / `enhancement` | `2024-04-16T11:49:42Z` | 开放提案；本次未验证到任意请求属性的场景状态隔离已交付。 |
| [#2529 Support JSON Schema definition from external file](https://github.com/wiremock/wiremock/issues/2529) | `open` / `enhancement` | `2024-01-23T20:37:05Z` | 开放提案；相邻的内联对象支持已交付，外部文件引用未验证交付。 |
| [#2710 Handlebars helper - join](https://github.com/wiremock/wiremock/issues/2710) | `closed` / `enhancement` | `2024-05-17T17:46:31Z` | 已验证交付；最终函数名为 arrayJoin，不是提案中的 join。 |
| [#2714 Introduce a GUI for open source](https://github.com/wiremock/wiremock/issues/2714) | `closed` / `enhancement` | `2024-06-19T08:52:08Z` | 已关闭但未纳入核心；维护者拒绝核心 GUI 范围，不属于已交付或重复议题。 |
| [#2759 Report webhook errors and results as sub-events](https://github.com/wiremock/wiremock/issues/2759) | `closed` / `enhancement` | `2024-06-20T15:53:39Z` | 已验证交付（稳定标签 3.13.2 存在实现）；本次未追溯最早发布版本。 |

### #3031：需求与证据

**用户需要：**同一上游同时提供 HTTP 和双向消息，希望局部替换 HTTP，保留 WebSocket 透传。

**交付证据：**https://github.com/wiremock/wiremock/pull/3286 已于 2026-01-08 合并；https://wiremock.org/docs/websockets/ 明示仅 4.x beta；议题 2026-09-15 维护者回复说明模拟已实现、代理尚未实现。

### #2684：需求与证据

**用户需要：**并行用户和线程应互不干扰，希望按 Cookie/JWT 等请求属性分区场景状态。

**交付证据：**https://wiremock.org/docs/stateful-behaviour/ 证实已有场景状态及重置；议题明确指出状态全局共享，未见关联的已合并交付记录。

### #2529：需求与证据

**用户需要：**希望 Schema 作为独立文件复用和编辑，避免在映射中复制、转义。

**交付证据：**https://wiremock.org/docs/request-matching/ 展示 3.4+ 的内联 JSON 对象；议题提到 PR #2566 仅为相邻改进，不能证明外部文件引用目标完成。

### #2710：需求与证据

**用户需要：**循环生成 JSON 数组时避免末尾逗号和冗长条件。

**交付证据：**https://wiremock.org/docs/response-templating/ 展示 arrayJoin；https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/extension/responsetemplating/helpers/JoinHelper.java 和 https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/extension/responsetemplating/helpers/WireMockHelpers.java 在稳定标签 3.13.2 中提供实现及注册。

### #2714：需求与证据

**用户需要：**以 GUI 编辑桩、查看请求响应并监控流量，降低配置门槛。

**交付证据：**议题 2024-06-19 回复明确表示已有外部 GUI，核心聚焦 Java/JSON 接口；https://www.wiremock.io/comparison 把厂商 UI 列为 Cloud 产品能力。

### #2759：需求与证据

**用户需要：**回调模板异常及下游返回不能静默，需要在原请求事件下追踪。

**交付证据：**https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/org/wiremock/webhooks/Webhooks.java 明确调用 appendSubEvent 记录 WEBHOOK_REQUEST、WEBHOOK_RESPONSE 以及 SubEvent.error，独立证实实现存在。

这些样本表明：服务端并行状态隔离仍难，可读配置及回调观测是实际交付细节，开放/关闭都不能单独证明功能缺失或已发布。#3031 已有 beta 实现而仍开放；#2714 标成 completed 却明确不纳入核心；#2710 最终函数名也与提案不同。

## 给 Reva 的三个可独立评审的改进

本次未发现必须阻断当前版本的 P0。前两个是 P1 的工作流增量，诊断集成为 P2；不要求新增账号、WebView 或自研模拟引擎。以下为后续 PR 建议，本次没有实现。

### 1. 从选定请求及响应导出一个 WireMock 静态桩（P1）

- **用户场景：**接口尚不稳定时，在 Reva 成功发送一次请求，把选定响应交给同事或本地应用离线联调。
- **现有缺口：**已有原生请求集合导出，但没有把当前响应与请求变成可运行桩的出口；历史也没有完整响应快照。
- **最小 PR：**为当前有完整响应的 HTTP 标签提供显式导出预览，写入一份 UTF-8 mapping JSON：方法、urlPath、按键分组的查询条件、响应状态及文本体；重复查询值用 hasExactly 匹配。默认不匹配 Authorization/Cookie/API Key、不输出 Set-Cookie，敏感头需单独勾选。首版仅支持 JSON/文本及无请求体或纯文本请求；截断、二进制、未解析变量及 multipart 明确拒绝，不静默降级。导出不启动服务、不访问网络。
- **依赖与边界：**复用 model.Request/Response、vars 和发送快照；新增纯 Go 转换包、现有保存对话框、zh/en i18n。必须使用本次已展开的发送快照，避免后来编辑造成请求响应错配；不增加 Java 依赖。
- **有意义的验收：**httptest 获取 JSON 响应后导出，验证方法、urlPath、同名查询值 hasExactly、状态/正文；Authorization/Cookie/Set-Cookie 默认不出现。未解析变量、截断、二进制均报错且不写文件；以固定离线映射样例核对语义，Go 测试不下载或启动 WireMock。

### 2. 增加无 GUI 的串行集合执行与 CI 退出码（P1）

- **用户场景：**在 Reva 编辑集合后，CI 启动用户自备 WireMock，再执行同一组请求和 JS 断言，无需桌面或账号。
- **现有缺口：**HTTP 和脚本库已存在，但鉴权继承、变量与结果汇总由 GUI 串起，没有命令行集合执行入口。
- **最小 PR：**抽取可取消的共享单请求流水线，新增 cmd/reva-run；只读一个原生集合文件及可选环境文件，深度优先串行执行，复用超时、继承鉴权及 pm.test；输出有界 JSON 报告，区分通过、断言失败和执行错误，任一失败非零退出。首版不做并发、循环、GUI 运行器或完整 Postman 兼容；变量保留现有每次发送副本语义并写入帮助。
- **依赖与边界：**将 gui.App.Send、prepareSend、afterResponse、VarsFor 和鉴权继承的业务组合提成非 GUI helper，保持既有优先级和显式头规则，不复制第二套。与静态桩导出独立，不新增 JS 或 WireMock 依赖。
- **有意义的验收：**用两个 httptest 路由及嵌套原生集合验证 Basic 继承、环境覆盖、前置脚本改头和发送顺序；成功退出 0，断言失败、脚本超时、网络超时非零且报告可区分原因。运行前后 REVA_HOME 及输入文件字节不变，报告不输出凭证/完整请求体，取消后不发送下一请求。

### 3. 按需读取外部 WireMock 的未匹配请求及近似匹配（P2）

- **用户场景：**请求得到模拟服务默认 404 时，在 Reva 直接查看路径、头或 JSON 条件哪里不匹配。
- **现有缺口：**HTTP 状态、正文和阶段计时解释不了服务端桩期望，用户仍要切工具查管理接口。
- **最小 PR：**增加可选外部模拟诊断面板，用户明确填写管理地址后点击获取，仅调用 /__admin/requests/unmatched 和 /__admin/near-misses/request，按脱敏后的本次发送快照查询并有界显示差异。提交前删除认证头、Cookie 及已知认证查询参数，预览说明这些匹配条件未比较；提供复制脱敏摘要。首版只读取，不自动探测、不后台轮询、不删日志、不启动引擎，不把业务凭证自动带给管理端。
- **依赖与边界：**复用 ui.Effect、context 超时、响应展示与 zh/en i18n；小型独立 Admin API 客户端，契约先固定 3.13.2。用户自备进程/容器，首版不涵盖 4.x beta 消息日志。
- **有意义的验收：**httptest 覆盖合法 unmatched/near-misses、空结果、HTML、超时、超大 JSON 和跨源重定向；验证只有点击才请求、取消及时、体积限制与错误提示有效。业务 Bearer/Cookie/API Key 不出现在管理调用和默认摘要；关闭面板不产生后台请求。

应交给专用工具的工作包括复杂状态和会话模型、录制代理、故障及异步回调引擎、gRPC/GraphQL 专用模拟、OpenAPI 全响应契约验证及高并发负载生成。WireMock 负责模拟依赖，负载工具负责压测，Reva 负责本地编辑和工作流出口。不要把 WireMock 当作负载生成器，或把 Cloud Runner 当作社区集合运行命令。

## 检查、评审与交付边界

文档变更不新增行为测试。四项仓库检查均在本工作树执行，进程内设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`，使用已有缓存，未安装仓库依赖；测试未访问互联网。结果及尾部：

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
ok  	github.com/runbrick/reva/internal/script	0.253s
?   	github.com/runbrick/reva/internal/service	[no test files]
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	0.813s
ok  	github.com/runbrick/reva/internal/service/store	0.298s
ok  	github.com/runbrick/reva/internal/storage	0.575s
ok  	github.com/runbrick/reva/internal/syncing	0.843s
ok  	github.com/runbrick/reva/internal/update	0.051s
ok  	github.com/runbrick/reva/internal/vars	0.603s
ok  	github.com/runbrick/reva/internal/websocket	0.038s
Exit: 0
```

另以 Python 标准库解析 JSON 并核对必需字段、10 项能力、6 个议题、3 个建议，执行 `git diff --check`，检查仅含两份授权文档。提交后可用 `git log -1 --format=%H -- docs/research/api-tools/wiremock.md` 获取交付 commit，避免自引用哈希。没有推送、创建远程 PR 或合并；这是可本地评审的文档 PR 提交。

## 局限与不确定性

1. 本次仅为文档和源码研究，未安装或运行 WireMock/JVM/Docker，未作吞吐、内存、实际兼容性或 GUI 截图测量。已实现判断来自官方文档、固定标签源码和合并记录，不是实机认证。

2. GitHub latest 本次返回稳定版 3.13.2（2025-11-14）；发布列表首项为预发布 4.0.0-beta.43。动态文档和 master 可能领先稳定版；WebSocket 单独标记仅限 4.x beta，其余重点能力以 3.13.2 源码/README 交叉检查。未穷举全部发布标签或推断将来稳定版日期。

3. 议题仅从一次 state=all、labels=enhancement、per_page=100 样本中剔除 PR 后选择六项，再逐页读取；不是全量需求统计，未带 enhancement 的需求可能遗漏。更新时间是 GitHub 元数据，与检索时间分开。

4. arrayJoin 和 webhook 子事件以稳定标签源码证明交付，但本次未追溯最早发布版本。开放议题可部分完成；关闭也可表示不纳入范围，均已逐项说明。

5. Cloud 边界根据厂商比较页及社区文档交叉核验，已纠正营销概述可能造成的“所有状态/验证都仅限 Cloud”误读。未审计 Cloud 服务条款、全部扩展及其许可证，不把商业服务标成 Apache-2.0，也没有证据把社区核心归为 source-available。

6. Reva 缺口限定于基线下已检视的入口、模型、服务、GUI、导入、脚本、同步与 CI；不是未来路线承诺。现有 GraphQL、WebSocket、脚本和同步全部计入对照。

7. 本次没有网络访问型测试；检查设置 GOPROXY=off，使用已缓存的工具链和依赖，未安装依赖。TASK.md 及本地 Viva skill 在任务说明中明确缺失，本次不改 GUI。

## 证据来源

官方文档和 raw.githubusercontent.com 优先，GitHub API 仅用于仓库/稳定发布元数据及一次议题列表；议题详情和预发布经直接页面读取。编号固定，Reva 链接指向研究基线。

1. [规范仓库，默认分支 master](https://github.com/wiremock/wiremock)
2. [3.13.2 自述及社区能力](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/README.md)
3. [3.13.2 核心 Apache-2.0 许可证](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/LICENSE.txt)
4. [稳定发布 3.13.2，2025-11-14](https://github.com/wiremock/wiremock/releases/tag/3.13.2)
5. [构建目标与 Jetty、模板和验证器依赖](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/build.gradle)
6. [服务端生命周期与管理接口源码](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/WireMockServer.java)
7. [桩定义、优先级及持久化](https://wiremock.org/docs/stubbing/)
8. [请求匹配与 JSON Schema](https://wiremock.org/docs/request-matching/)
9. [响应模板及 arrayJoin](https://wiremock.org/docs/response-templating/)
10. [场景状态机及重置](https://wiremock.org/docs/stateful-behaviour/)
11. [条件代理](https://wiremock.org/docs/proxying/)
12. [录制、快照与回放](https://wiremock.org/docs/record-playback/)
13. [请求日志、验证及近似匹配](https://wiremock.org/docs/verifying/)
14. [延迟及传输故障](https://wiremock.org/docs/simulating-faults/)
15. [异步回调](https://wiremock.org/docs/webhooks-and-callbacks/)
16. [3.13.2 回调子事件源码](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/org/wiremock/webhooks/Webhooks.java)
17. [JUnit Jupiter 生命周期与测试隔离](https://wiremock.org/docs/junit-jupiter/)
18. [独立 JAR 运行](https://wiremock.org/docs/standalone/java-jar/)
19. [官方 Docker 运行与挂载目录](https://wiremock.org/docs/standalone/docker/)
20. [扩展机制](https://wiremock.org/docs/extending-wiremock/)
21. [厂商社区版与 Cloud 比较页](https://www.wiremock.io/comparison)
22. [明确限于 4.x beta 的 WebSocket 模拟](https://wiremock.org/docs/websockets/)
23. [已合并 WebSocket 实现 PR #3286](https://github.com/wiremock/wiremock/pull/3286)
24. [本次发布列表首项预发布 4.0.0-beta.43](https://github.com/wiremock/wiremock/releases/tag/4.0.0-beta.43)
25. [3.13.2 数组连接实现](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/extension/responsetemplating/helpers/JoinHelper.java)
26. [3.13.2 arrayJoin 注册](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/extension/responsetemplating/helpers/WireMockHelpers.java)
27. [3.13.2 JSON Schema 匹配实现](https://raw.githubusercontent.com/wiremock/wiremock/3.13.2/src/main/java/com/github/tomakehurst/wiremock/matching/MatchesJsonSchemaPattern.java)
28. [一次取得的 enhancement 议题列表，含 PR 后续剔除](https://api.github.com/repos/wiremock/wiremock/issues?state=all&labels=enhancement&per_page=100)
29. [议题 #3031 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/3031)
30. [议题 #2684 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/2684)
31. [议题 #2529 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/2529)
32. [议题 #2710 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/2710)
33. [议题 #2714 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/2714)
34. [议题 #2759 正文、讨论及关闭语义](https://github.com/wiremock/wiremock/issues/2759)
35. [Reva：HTTP 发送、变量与鉴权、响应限制](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go)
36. [Reva：GraphQL 查询和变量编辑](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/graphql.go)
37. [Reva：ws/wss 客户端](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/websocket/client.go)
38. [Reva：鉴权验证及显式头优先](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/auth/auth.go)
39. [Reva：集合与环境变量优先级](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/env.go)
40. [Reva：Otto 脚本与测试运行时](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/script.go)
41. [Reva：pm API 和断言范围](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js)
42. [Reva：单标签发送流水线](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/app.go)
43. [Reva：导入分类、限制与预览入口](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/result.go)
44. [Reva：OpenAPI 转换及约束边界](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/openapi.go)
45. [Reva：原生集合 JSON 导出](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/collections.go)
46. [Reva：本地 JSON 与原子存储](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/storage/storage.go)
47. [Reva：可选备份及多种远端同步](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/sync.go)
48. [Reva：AES-GCM 备份和三方合并](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/syncing/sync.go)
49. [Reva：桌面程序入口](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/cmd/reva/main.go)
50. [Reva：当前 CI 检查与打包](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/.github/workflows/ci.yml)
51. [Reva：服务契约，无桩服务和集合执行器](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/service.go)
52. [Reva：请求阶段计时](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/trace.go)
53. [Reva：请求、脚本、鉴权及设置模型](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go)
54. [Reva：集合与文件夹鉴权继承](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/tree.go)
55. [Reva：WebSocket 交互入口](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/websocket.go)
56. [Reva：变量替换](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/vars/vars.go)
57. [Reva：传输选项与连接池缓存](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/options.go)
58. [本次规范仓库及 SPDX 元数据](https://api.github.com/repos/wiremock/wiremock)
59. [本次 latest 返回的稳定发布元数据](https://api.github.com/repos/wiremock/wiremock/releases/latest)
60. [本次发布列表，包含 4.x beta](https://github.com/wiremock/wiremock/releases)
