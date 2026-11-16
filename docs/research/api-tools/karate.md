# Karate：对 Reva 的 API 测试工作流研究

- 研究截止：`2026-11-16T22:41:28+08:00`；观察日 2026-11-16。
- Reva 起始基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；分支 `runbrick/research-karate`。
- 范围：`karatelabs/karate` 核心框架；已发布能力以 **v2.1.4（2026-10-22）** 为基准，滚动文档和后续提案单独注明。
- 本 PR 仅新增本 Markdown 与同目录 `karate.json`；不改源码、不新增测试、不推送或合并。

## 判断与定位

Karate 是 Java 21 上把 API 流程、结构断言、有状态 mock、并行回归和 Gatling 集成在同一 DSL 的 MIT 开源框架。对 Reva 最有用的是可重复执行与诊断工作流，复杂负载与状态机 mock 适合通过文件交接给专业测试工程。

Karate 面向把一组有业务含义的调用写成可版本化、可在 CI 重复运行的测试工程；Reva 面向本机手动调试、编辑与数据管理。两者能够互补：桌面验证后交接测试骨架，专用测试工程负责复杂状态、数据集和负载。

## 架构与许可证

Suite → FeatureRuntime → ScenarioRuntime → StepExecutor 分派匹配引擎、HTTP 和其他动作；karate-js 为自研 JVM JavaScript 引擎，karate-core 提供 DSL、运行时、HTTP、mock 和报告，karate-junit6 连接测试生命周期，karate-gatling 连接负载模型，另有 image/profiling 模块。HTTP 客户端为 Apache HttpClient，mock 使用 Netty；POM 的 Java 运行目标为 21，构建编译器版本与运行要求有别。测试以本地 .feature、JS 和数据文件管理，无需 Karate 托管账号。 [4] [5] [23]

**许可证核验：MIT（SPDX: `MIT`，属于 OSI 开源许可证）。** 当前官方仓库元数据、v2.1.4 LICENSE 与 POM 一致；不是把“免费可用”直接推断成开源。[1] [2] [5]

商业边界按官方定价页与具体 API 文档核对：[6]

| 范围 | 本研究认定 |
| --- | --- |
| 核心 API/结构断言/并行/mock/报告/Gatling | MIT 核心及开源集成，可本地/CI 使用，无需 Karate 账号。 |
| WebSocket | 核心 `karate.webSocket()`/`listen` 是开源；商业 `karate.channel("websocket")` session/codec/collect 是另一套 API，不能相互替换。[7] [8] |
| IDE 与企业扩展 | IDE 高级调试、Async Protocol Pack、Agent、覆盖率/治理及企业授权/SSO 是收费产品边界；开发时授权与 CI runtime 授权不同，不归入核心 MIT 结论。 |
| Xplorer | 独立桌面产品，有免费层和 Premium；免费层也不自动意味着源码 MIT。官方文档区分集合导出与 Premium OpenAPI 导出/框架运行等，不把它的导入、OAuth GUI 或 runner 算作核心框架功能。[9] [10] |

未发现需要把核心归为 source-available 的证据；商业附加产品也没有因为可能可见源码就被标为 OSI 开源。此处没有进行依赖树法律审计。

## 已实现且有辨识度的能力

### 1. 面向 API 的 Gherkin 与结构断言

**状态：**已实现；固定版本架构与文档交叉核验。 url/path/method/status 与 match 共用步骤语言；JSON/XML 可以整体比较、包含匹配和逐元素校验，减少字段级样板代码。

主要证据：[11] https://docs.karatelabs.io/assertions/match-keyword

### 2. 可复用的标记式结构校验

**状态：**已实现。 #string、#number、## 可选字段、#notpresent、数组标记与 #? 表达式可组合，区分缺失和 null，支持跨字段约束。这是 Karate 自有 matcher 语言，不等同于标准 JSON Schema 或 OpenAPI 响应验证器。

主要证据：[12] https://docs.karatelabs.io/assertions/schema-validation

### 3. 数据驱动的独立用例

**状态：**已实现。 Scenario Outline/Examples 每行成为独立用例，! 列保留表达式类型，读取 CSV/JSON/YAML 或数组调用 feature；v2.1.4 KarateJs.read 的读取分派提供源码证据。

主要证据：[13] https://docs.karatelabs.io/reusability/data-driven-tests

### 4. 请求链与初始化复用

**状态：**已实现。 call 传参并取回结果，callonce 复用 feature 内初始化，karate.callSingle 复用整次运行初始化；可组合登录、造数和业务请求，配合 karate-config.js、karate.env 切换环境。

主要证据：[14] https://docs.karatelabs.io/reusability/calling-features

### 5. 虚拟线程并行与失败即停

**状态：**已实现；要求用例隔离。 Java 21 虚拟线程运行 feature/scenario；Runner 或独立 JAR 选择路径、标签和并发数。abortSuiteOnFailure 跳过队列并在步骤边界中断在途场景，不承诺立即强杀全部 I/O；并行用例不能依赖执行顺序。

主要证据：[16] https://docs.karatelabs.io/running-tests/parallel-execution

### 6. 有状态 HTTP mock 与状态快照

**状态：**已实现；新状态 API 与请求上限在 v2.1.4 交付。 DSL 按路径/方法路由、维护 CRUD 状态、延迟或代理响应；karate.start 启动本地 mock，mock.get(name, defaultValue) 在请求间取数据快照。maxInitialLineLength/maxHeaderSize/maxContentLength 可配置。只面向本机、CI 或受信内网测试。

主要证据：[17] https://docs.karatelabs.io/extensions/test-doubles

### 7. 独立执行与可归档报告

**状态：**已实现。 Java 21 独立 JAR 无需 Maven/Gradle，也支持 JUnit 集成。默认 HTML，JSONL/JUnit XML/Cucumber JSON 按需启用，包含步骤、HTTP 交换与时间线。v2.1.4 报告默认遮盖 Authorization/Cookie/Set-Cookie/Proxy-Authorization；发布说明明确控制台不在此遮盖范围。

主要证据：[18] https://docs.karatelabs.io/running-tests/test-reports

### 8. 复用功能用例进行负载测试

**状态：**已实现；开源 karate-gatling 模块。 karateFeature 复用功能流程和响应断言，Java Gatling DSL 定义用户注入、馈送数据和停顿。批量顺序执行与负载模型不同；不把单次耗时当作压力测试，也不引用营销吞吐量作为实测。

主要证据：[20] https://docs.karatelabs.io/extensions/performance-testing

### 9. 开源 WebSocket 异步断言

**状态：**已实现；与商业扩展分开。 karate.webSocket/karate.webSocketBinary 支持消息、handler 过滤、listen 等待、请求头和子协议，场景结束自动关闭。HTTP 操作可与事件等待串联；商业 karate.channel("websocket") 的 session/codec/collect 不归入核心能力。

主要证据：[7] https://docs.karatelabs.io/advanced/websocket

### 10. 可配置认证与登录步骤复用

**状态：**已实现；结论按 v2.1.4 源码收窄。 configureAuth 的固定版本分派含 basic/bearer/oauth2/ntlm，也能通过 headers/API key 和登录 feature 组合认证。滚动文档还列 JWT/Digest/AWS 等统一配置，但此固定版本分派未见，不能认定全已发布；HTTP 登录脚本也不等价于完整 OAuth 浏览器生命周期管理。

主要证据：[22] https://github.com/karatelabs/karate/blob/v2.1.4/karate-core/src/main/java/io/karatelabs/core/KarateConfig.java

第 3 项另由固定版本 `KarateJs.read` 支持；第 4 项的环境由配置文档支持；第 6、7 项的新行为由发布说明确认。[24] [15] [3]

## 与当前 Reva 的直接对比

以下本地证据均对应起始提交，已读取具体函数；“未见”依据 `cmd`、`internal` 与工作流源码检查，不依据 README 没写。文件链接固定至基线，避免以后 main 变化。

| 维度 | Karate | Reva 实际能力与证据 | 机会 |
| --- | --- | --- | --- |
| 定位与架构 | Java 21 测试框架，Suite/Feature/Scenario 运行时统合 DSL、HTTP、mock 和报告。 | Go 1.26/Viva 本地桌面，cmd/reva 启动 GUI，service 隔离磁盘与网络，Send 由 UI Effect 调度；cmd 目录未见产品回归 CLI。<br>源码：[cmd/reva/main.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/cmd/reva/main.go) main；[internal/service/service.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/service.go) RequestService；[internal/gui/app.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/app.go) Send/run | 抽取可共享的发送流程以支持无窗口执行，不在 Reva 内嵌 JVM。 |
| 协议 | HTTP、JSON/XML DSL、SOAP/GraphQL 请求及开源 WebSocket；Kafka/gRPC 等商业扩展单独核验。 | requestURL 明确允许 HTTP/HTTPS；GraphQL 有 query/variables 专用编辑器，最终发送 HTTP JSON；RFC 6455 客户端支持 ws/wss、文本发送和消息读取。未见 gRPC/Kafka、SOAP 专用构造器或 XML 路径断言；XML 仍可通过 raw body 手工发送。<br>源码：[internal/service/request/request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go) requestURL/Send；[internal/gui/graphql.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/graphql.go) graphQLBodySection；[internal/websocket/client.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/websocket/client.go) Dial/WriteText/ReadMessage | 保留交互优势；跨协议自动化交给专业测试工程，避免误称 Reva 缺少 GraphQL/WebSocket。 |
| 认证 | v2.1.4 configure auth 含 Basic/Bearer/OAuth2/NTLM，登录步骤可复用；高级类型按实际版本核验。 | None/Bearer/Basic/API key，集合/文件夹继承、显式 None 屏障与跨源重定向清除敏感头。没有 OAuth2 生命周期；S3 SigV4 在同步后端，不是普通请求编辑器 AWS 认证。<br>源码：[internal/model/model.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go) AuthType；[internal/auth/auth.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/auth/auth.go) 认证继承与头生成；[internal/service/request/request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go) protectRedirects；[internal/s3/sigv4.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/s3/sigv4.go) 同步签名 | 先明确登录请求与运行内变量传递的工作流，再独立评估 OAuth2 需求。 |
| 变量、脚本和请求链 | 有类型 JS/JSON/XML 变量、环境配置、call/callonce/callSingle、数据驱动与 Java 互操作。 | 集合变量后叠加环境变量，{{name}} 最后启用值获胜；已有 Otto ES5 预请求/测试脚本、pm.test/pm.expect 和 console.log，默认 2 秒。RunPre 变量修改只供本次 send；RunTests 只返回结果/日志，无法自动将登录响应变量写到下一请求。无 pm.sendRequest 或 Promise 异步测试。<br>源码：[internal/vars/vars.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/vars/vars.go) Lookup/Apply；[internal/script/script.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/script.go) RunPre/RunTests/DefaultTimeout；[internal/script/api.js](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js) variableAPI/pm.test；[internal/gui/scripts.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/scripts.go) prepareSend/afterResponse | 顺序 runner 与变量作用域分别定义；跨请求变量应明确为运行内状态，不能悄悄写环境文件。 |
| runner、CI 与数据驱动 | 独立 JAR/JUnit、标签、并行、Examples/CSV/JSON/YAML、JUnit XML/JSONL 报告；开源框架免费运行 CI。 | 已有逐请求 GUI 发送/取消和脚本结果；cmd 只有桌面入口，GitHub Actions 检查自身代码和打包，不能执行用户集合。未见集合 runner 或运行报告导出接口。<br>源码：[cmd/reva/main.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/cmd/reva/main.go) main；[internal/gui/app.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/app.go) sendJob/run；[internal/gui/scripts.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/scripts.go) testsSection；[.github/workflows/ci.yml](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/.github/workflows/ci.yml) 自身代码 CI | 优先有稳定退出码和机器报告的串行原生集合 runner，再考虑数据行和并发。 |
| 导入导出 | 核心以 .feature/config/data 文件组织工程；独立 Xplorer 的 Postman/OpenAPI/curl 导入、GUI runner 和导出不算核心框架能力。 | importer 支持原生/环境、Postman v2、HAR、curl、Swagger2/OpenAPI3 JSON；32 MiB 上限、诊断与显式确认，导入不执行脚本。OpenAPI 无 YAML/外部引用；Postman 脚本省略并提示。集合导出当前为原生 JSON，不是 Karate。<br>源码：[internal/importer/result.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/result.go) ParseWithOptions/MaxInputBytes；[internal/importer/postman.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/postman.go) 脚本省略诊断；[internal/importer/openapi.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/openapi.go) sample；[internal/gui/importer.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/importer.go) reviewImport/performImport；[internal/gui/collections.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/collections.go) exportCollection | 提供预览后写出的单请求 Karate 骨架，明确列出兼容性损失，不承诺自动翻译脚本。 |
| mock 与结构校验 | 有状态 HTTP mock、路由/代理、状态快照；自有 matcher 支持 JSON/XML、缺失/null 与类型约束。 | service/fake 是内部内存测试替身，不是用户 HTTP mock 产品。已有 eql/include 等断言，但 eql 失败仅报通用错误；未见标准 JSON Schema 或 Karate matcher。OpenAPI sample 明确仅生成请求模板，不验证约束且省略响应定义。<br>源码：[internal/service/fake/fake.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/fake/fake.go) 内部测试 services；[internal/script/api.js](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js) deepEqual/expect；[internal/importer/openapi.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/openapi.go) sample 注释与元数据处理 | 先改善深层断言差异定位；状态机 mock、复杂契约与 schema DSL 使用专业工具。 |
| 性能 | karate-gatling 复用 feature/断言并定义注入与负载模型，机器/场景不同不能直接比吞吐。 | Response 已有 Duration、Size 和 DNS/Connect/TLS/Send/Wait/Download 时间线；保留响应体上限默认 10 MiB。这是单次观测，不是压力测试调度器。<br>源码：[internal/model/model.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go) Response/Phase；[internal/service/request/trace.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/trace.go) sendTrace；[internal/service/request/request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go) DefaultMaxBodyBytes | 通过文件交接 Karate/Gatling，保留 Reva 单次调试测量。 |
| 协作与本地数据 | feature/JS/data 本地文本可进 Git，可在本机/CI 运行且核心不要求账号；商业 SSO/Agent/治理不属于 MIT 核心。 | 已有本地缩进 JSON、REVA_HOME、原子写入和免账号模式，也已有加密备份/恢复、WebDAV/S3/GitHub/Gitee 可选同步及三方合并。配置凭据仍在本地 JSON，备份加密不代表源数据文件加密或企业权限管理。<br>源码：[internal/storage/storage.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/storage/storage.go) Dir/WriteJSON；[internal/syncing/sync.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/syncing/sync.go) Seal/Open/Merge；[internal/gui/sync.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/sync.go) syncSettings/provider | 以版本化文件、离线 runner 和可审查导出为主，不引入强制账号或云工作区。 |

## 功能议题：需求、状态与交付分开核验

采样范围：一次 enhancement 列表页的前 25 条，随后读取下列六条正文与维护者评论。包含 **2 open、4 closed**；排除例行缺陷修复，#3107 因其共享初始化诊断语义而保留。精确标题/标签保留英文，更新时刻为 GitHub `updatedAt`，非抓取时间。

### #3131 — Add a 'Copy' CTA button to copy the HTTP Request/Response in HTML report

- 链接：https://github.com/karatelabs/karate/issues/3131 [25]
- 状态：`open`；标签：`enhancement`；更新：`2026-11-14T01:18:11Z`。
- 用户需求：需要把报告里的单次 HTTP 交换复制进 JIRA 等工单作为验证证据。
- 交付判断：**开放提案；未找到交付证据**。正文明确请求复制按钮；页面无关联 PR 或里程碑，已发布 v2.1.4 早于提案，不据此认定后续实现。
- 交叉证据：[25] https://github.com/karatelabs/karate/issues/3131

### #3107 — on a callonce/callSingle failure- Limit the call marker only on the executing scenario in the HTML report

- 链接：https://github.com/karatelabs/karate/issues/3107 [26]
- 状态：`open`；标签：`enhancement`, `fixed`；更新：`2026-11-05T05:22:09Z`。
- 用户需求：共享初始化失败时，用户需要在大量失败场景中迅速找到真正执行 callonce/callSingle 的根因轨迹。
- 交付判断：**开放；有 fixed 标签和引用提交，但合并及发版未核实**。页面引用 de48861，里程碑 2.1.5；仍 open，最新已发布为 2.1.4。标签与引用提交均不能证明已发布。
- 交叉证据：[26] https://github.com/karatelabs/karate/issues/3107；[32] https://github.com/karatelabs/karate/commit/de48861ba614d92b31f1509fd1ec28ebf9ad0a41

### #3095 — Karate mock server variables

- 链接：https://github.com/karatelabs/karate/issues/3095 [27]
- 状态：`closed`；标签：`enhancement`, `fixed`；更新：`2026-10-22T07:59:12Z`。
- 用户需求：需要断言 mock 内部累积状态，不想新增仅为测试服务的 HTTP 端点；快照避免后续请求或调用者修改状态。
- 交付判断：**已核实交付：v2.1.4 的 mock.get(name, defaultValue)**。发布说明明确列 #3095，官方 Reading Mock State 说明请求间快照和默认值。旧 getVariable 活对象语义不能和新 API 混淆。
- 交叉证据：[27] https://github.com/karatelabs/karate/issues/3095；[3] https://github.com/karatelabs/karate/releases/tag/v2.1.4；[17] https://docs.karatelabs.io/extensions/test-doubles

### #3075 — Expose bounded request-header and body limits in the native HTTP server API

- 链接：https://github.com/karatelabs/karate/issues/3075 [28]
- 状态：`closed`；标签：`enhancement`, `fixed`；更新：`2026-10-22T07:59:35Z`。
- 用户需求：代理/Cookie/认证元数据可超出固定头上限，需要正值有限的 HTTP/1.x 头、行和正文上限与明确拒绝状态。
- 交付判断：**已核实交付：v2.1.4**。发布说明明确列 #3075，覆盖 maxInitialLineLength/maxHeaderSize/maxContentLength、mock CLI/karate.start 入口与 431/414/400；不外推为 HTTP/2 同一设置。
- 交叉证据：[28] https://github.com/karatelabs/karate/issues/3075；[3] https://github.com/karatelabs/karate/releases/tag/v2.1.4；[17] https://docs.karatelabs.io/extensions/test-doubles

### #3073 — Provide sorting by Feature File Path in Karate v2.x Test Execution Report

- 链接：https://github.com/karatelabs/karate/issues/3073 [29]
- 状态：`closed`；标签：`enhancement`, `fixed`；更新：`2026-10-22T07:59:42Z`。
- 用户需求：大套件按服务目录组织，报告需按 feature 路径排序，不能仅按自由填写的标题排序。
- 交付判断：**已核实交付：v2.1.4 路径排序；未确认全部提案 UI**。发布说明明确列 #3073 和 feature 文件路径排序；这不证明一定新增独立列或目录过滤。
- 交叉证据：[29] https://github.com/karatelabs/karate/issues/3073；[3] https://github.com/karatelabs/karate/releases/tag/v2.1.4；[18] https://docs.karatelabs.io/running-tests/test-reports

### #3007 — Feature request: built-in browser video recording for UI tests

- 链接：https://github.com/karatelabs/karate/issues/3007 [30]
- 状态：`closed`；标签：`enhancement`, `wontfix`；更新：`2026-08-14T04:03:31Z`。
- 用户需求：无头 CI 的浏览器旅程失败后，需要视频解释之前页面、弹窗和操作序列。
- 交付判断：**关闭但未在社区核心交付：wontfix**。维护者 2026-08-14 明确建议第三方 Docker 录像容器，并提到商业 karate-agent。关闭原因为 completed 仍不等于功能实现；商业录像不归给 MIT 核心。
- 交叉证据：[30] https://github.com/karatelabs/karate/issues/3007

## 建议的三个最小 PR

本节是建议，尚未实现。没有修改任何未归属文件；涉及公共发送流程的建议应由协调者另行分配所有权。没有 P0：本研究未发现需要以文档 PR 紧急修复的线上事故。

### 1. 增加原生集合的串行无界面回归入口（P1）

- 用户场景：开发者在桌面调好冒烟集合，提交后希望 CI 重跑相同请求/脚本，失败时阻止发布。
- 现有缺口：现有 RequestService/Otto 可复用，但预处理和结果处理在 GUI，产品入口不能执行用户集合。
- 最小 PR：新增 cmd/reva-run，显式指定原生集合和可选环境；按树顺序串行执行 HTTP。将认证继承、Options.Resolve、预脚本、Send、测试的公共流程提到 service helper，GUI 同用。首版输出有界 JSON 与 0/1/2 退出码（成功/用例失败/配置错误），失败或取消停止后续请求；不写 history/集合/环境，不包含 CSV、并行或跨请求变量写回。
- 依赖：复用 model/auth/vars/request/script，不增依赖；需要协调 app.go/scripts.go 调用边界和新增 cmd/service 文件的所有权；定义环境覆盖顺序、取消和截断响应处理，保持脚本 2 秒限制。
- 有意义的验收：用 httptest 验证继承 API key、环境覆盖集合变量、预脚本改头；第二请求断言失败返回 1、JSON 定位请求、第三请求未发送。配置错误返回 2，全成功返回 0，取消停止后续发送；运行前后数据目录字节相同，全程离线。

### 2. 让深层 JSON 断言报告首个差异路径（P1）

- 用户场景：订单响应几十行，eql 失败后用户需要知道第二项 total 缺失、类型或值变化。
- 现有缺口：deepEqual 只返回布尔，eql 仅抛通用错误；已有测试结果 UI 无法指出结构差异。
- 最小 PR：保持 pm.expect(...).to.eql API，比较时记录首个路径和差异种类：缺键、多键、类型、长度、值。默认只显示路径/种类，不输出敏感值；保持原有相等语义、循环对象和 2 秒预算，不引入 schema DSL 或声称标准 JSON Schema。沿用现有错误显示区，新增文案须有 zh/en。
- 依赖：修改 internal/script/api.js 与必要脚本测试，复用现有 Result.Error；保持 ES5/Otto 兼容，含点/括号/中文键的路径须明确转义；结构化错误模型可以另开 PR，首版不改公共结果接口。
- 有意义的验收：离线脚本测试覆盖 orders[1].total、中文键、缺键与 null、循环对象；错误含确定的首个路径与种类、不含敏感值。headless harness 验证 Tests 区能显示定位，成功 eql 行为保持。

### 3. 预览并导出单个 HTTP 请求的 Karate 测试骨架（P2）

- 用户场景：用户把调试好的请求交给测试工程师扩展为 mock、数据驱动或 Gatling 流程，日常仍用 Reva。
- 现有缺口：原生 JSON 导出不能直接交接 .feature；在 Go 桌面重写状态机 mock 和负载引擎投入过大。
- 最小 PR：首版只导出选定请求的 method、URL、启用 query/header、无体或 raw/JSON 文本；不执行、不解析活动环境秘密。变量/认证给出待填写说明，预览列出脚本、继承认证、文件体与选项等未转换内容，确认后只创建新文件，同名拒绝。期望由用户显式补充，不自动假设 200 或固化整个响应；不翻译脚本、不安装 JVM。
- 依赖：纯 Go 转义/模板 helper，复用 model 和保存流程；预览解释认证继承且不自动展开秘密；未覆盖内容提供结构化诊断；链接独立 JAR 文档，由用户在 Reva 外运行，专业工程维护 mock/负载。
- 有意义的验收：离线输出测试覆盖引号、换行、三引号与 {{name}} 保留及待处理诊断；脚本/文件体不得静默丢失。headless 验证预览/取消无写入，确认只创建新文件且同名拒绝；产物不含展开 token，导出不发送网络请求。

专业工具边界：优先把 Karate 用于状态机 mock、可复用登录/造数、跨协议等待、数据驱动及 Gatling 负载；Reva 保留本地 Go/Viva 编辑、手动发送和现有同步。导出骨架不复制 Karate 源码，也不在 GUI 内安装 JVM、运行未知脚本或偷偷发送请求。用户需要标准 JSON Schema/OpenAPI 契约验证时应另选专用验证器；Karate 标记匹配并不自动满足该标准。

## 验证记录

文档研究与测试分开。检查使用现有模块缓存；仅对当前检查进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`、`GOSUMDB=off`，未安装仓库依赖。未新增行为测试；未启动真实窗口或访问外网测试服务器。

```text
$ gofmt -l .
(空输出)
Exit: 0
$ go vet ./...
(空输出)
Exit: 0
$ go tool viva vet ./...
(空输出)
Exit: 0
$ go test ./... -count=1
?   	github.com/runbrick/reva/internal/service/fake	[no test files]
ok  	github.com/runbrick/reva/internal/service/request	0.883s
ok  	github.com/runbrick/reva/internal/service/store	0.453s
ok  	github.com/runbrick/reva/internal/storage	0.818s
ok  	github.com/runbrick/reva/internal/syncing	0.961s
ok  	github.com/runbrick/reva/internal/update	0.072s
ok  	github.com/runbrick/reva/internal/vars	0.782s
ok  	github.com/runbrick/reva/internal/websocket	0.064s
Exit: 0
```

JSON 已用 Python 标准库解析，并核对必需字段、10 项能力、6 条议题与 3 条建议；Markdown 与 JSON 来自同一研究事实。Markdown 表格/代码围栏、源码路径存在性、`git diff --cached --check` 和仅包含两个归属文件的暂存范围检查均通过（退出码 0）。

## 限制与不确定性

1. 观察日按执行环境实际时间为 2026-11-16，retrieved_at 保留本机实际 +08:00 偏移；这是研究窗口截止时间而非发布日期。
2. 已发布基准为 v2.1.4（2026-10-22）及该标签源码；官网为滚动文档。README_V2.md 仍有浏览器将来加入、依赖即将提供等旧表述，优先采用固定源码与发布说明。
3. 首次 REST issues 列表返回 403 rate limit exceeded；随后只获取一次 enhancement 列表页前 25 条（同时含 open/closed），用嵌入元数据取得精确标题/状态/标签/updatedAt，再直接读取六条议题。不是全库普查，也未凭标签推断交付。
4. 未安装或执行 Karate/JVM/Gatling，不声称实测吞吐、GUI 易用性或迁移保真；已实现结论来自官方文档、固定源码与发布。Reva 缺口只针对本次基线源码。
5. MIT 结论仅覆盖核心 LICENSE/POM，未审计所有依赖、插件、Xplorer 或商业扩展许可。免费不等于开源，仅源码可见或商业授权也不自动是 OSI 开源。
6. 认证滚动文档列有超出 v2.1.4 configureAuth 分派的类型，报告仅认定固定源码明确支持的类型；Java/HTTP 自定义方案不等价于内置凭据生命周期。
7. 六条议题含两条 open、三条已核实交付、一条 wontfix；#3107 引用提交合并和 2.1.5 发版未核实。#3007 用于说明专业工具与企业边界，不建议 Reva 扩展浏览器自动化。
8. TASK.md 与本地 .claude Viva 技能缺失；依任务豁免只做文档，不写 GUI、不安装依赖、不新增行为测试。仓库检查使用缓存并禁用 Go 模块在线访问，研究 HTTP 与测试分开。

## 编号证据来源

1. [Karate 官方仓库](https://github.com/karatelabs/karate)
2. [v2.1.4 MIT 许可证](https://github.com/karatelabs/karate/blob/v2.1.4/LICENSE)
3. [v2.1.4 发布说明（2026-10-22）](https://github.com/karatelabs/karate/releases/tag/v2.1.4)
4. [v2.1.4 架构设计](https://github.com/karatelabs/karate/blob/v2.1.4/docs/DESIGN.md)
5. [v2.1.4 模块与 Java 目标版本](https://github.com/karatelabs/karate/blob/v2.1.4/pom.xml)
6. [官方定价与产品边界](https://karatelabs.io/pricing)
7. [开源 WebSocket API](https://docs.karatelabs.io/advanced/websocket)
8. [商业 WebSocket 扩展及两种 API 区别](https://docs.karatelabs.io/extensions/websocket)
9. [Xplorer 授权边界](https://docs.karatelabs.io/xplorer/licensing)
10. [Xplorer 导出能力，独立桌面产品](https://docs.karatelabs.io/xplorer/karate-export)
11. [结构匹配](https://docs.karatelabs.io/assertions/match-keyword)
12. [标记式结构校验](https://docs.karatelabs.io/assertions/schema-validation)
13. [数据驱动测试](https://docs.karatelabs.io/reusability/data-driven-tests)
14. [调用复用与初始化缓存](https://docs.karatelabs.io/reusability/calling-features)
15. [配置环境](https://docs.karatelabs.io/core-syntax/configuration)
16. [并行隔离与失败即停](https://docs.karatelabs.io/running-tests/parallel-execution)
17. [有状态 mock 与状态读取](https://docs.karatelabs.io/extensions/test-doubles)
18. [测试报告](https://docs.karatelabs.io/running-tests/test-reports)
19. [独立 JAR 与 CI 执行](https://docs.karatelabs.io/getting-started/standalone-execution)
20. [Gatling 集成](https://docs.karatelabs.io/extensions/performance-testing)
21. [认证滚动文档](https://docs.karatelabs.io/http-requests/headers-auth)
22. [v2.1.4 configureAuth 支持类型](https://github.com/karatelabs/karate/blob/v2.1.4/karate-core/src/main/java/io/karatelabs/core/KarateConfig.java)
23. [v2.1.4 HTTP/NTLM 实现](https://github.com/karatelabs/karate/blob/v2.1.4/karate-core/src/main/java/io/karatelabs/http/ApacheHttpClient.java)
24. [v2.1.4 数据读取与 mock API](https://github.com/karatelabs/karate/blob/v2.1.4/karate-core/src/main/java/io/karatelabs/core/KarateJs.java)
25. [#3131 Add a 'Copy' CTA button to copy the HTTP Request/Response in HTML report](https://github.com/karatelabs/karate/issues/3131)
26. [#3107 on a callonce/callSingle failure- Limit the call marker only on the executing scenario in the HTML report](https://github.com/karatelabs/karate/issues/3107)
27. [#3095 Karate mock server variables](https://github.com/karatelabs/karate/issues/3095)
28. [#3075 Expose bounded request-header and body limits in the native HTTP server API](https://github.com/karatelabs/karate/issues/3075)
29. [#3073 Provide sorting by Feature File Path in Karate v2.x Test Execution Report](https://github.com/karatelabs/karate/issues/3073)
30. [#3007 Feature request: built-in browser video recording for UI tests](https://github.com/karatelabs/karate/issues/3007)
31. [本次功能议题采样列表（状态与更新元数据）](https://github.com/karatelabs/karate/issues?q=is%3Aissue%20label%3Aenhancement)
32. [#3107 引用提交（仅核对议题链接，未核实合并）](https://github.com/karatelabs/karate/commit/de48861ba614d92b31f1509fd1ec28ebf9ad0a41)
33. [v2.1.4 HTTP 构建器认证应用与 OAuth 预览处理](https://github.com/karatelabs/karate/blob/v2.1.4/karate-core/src/main/java/io/karatelabs/http/HttpRequestBuilder.java)
