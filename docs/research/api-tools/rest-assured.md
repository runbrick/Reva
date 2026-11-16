# REST Assured：面向 Reva 的开源 API 测试框架调研

- 检索汇总时间：`2026-11-16T22:37:40+08:00`；本机观测日期：2026-11-16。
- 起始基线：`72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910`；分支：`runbrick/research-rest-assured`。
- 类别：`framework`；规范仓库：[rest-assured/rest-assured](https://github.com/rest-assured/rest-assured)。
- 范围：仅本文件与同名 JSON；没有远程 PR、源码变更或依赖安装。

## 判断与版本边界

REST Assured 是面向 JVM 工程和 CI 的代码式 HTTP 验证库，Reva 是本地免账户交互客户端。可借鉴回归执行、可读断言与会话隔离；复杂 JVM/Spring/契约测试宜留在专项工程。当前稳定版为 6.3.0，另有已发布的 7.0.0-RC1。

以 Java 流式 DSL、Hamcrest 匹配器、请求/响应规范、过滤器及可选路径、校验和框架模块组成，通过 Maven/Gradle 测试依赖嵌入工程，JUnit/TestNG 等负责套件执行和 CI 报告。6.3.0 的核心已是 Java，但默认仍用 Apache HttpClient 4，路径模块仍引入 Groovy；7.0.0-RC1 默认改用 JDK HTTP client，Apache5、GPath/Jayway 可选，要求 Java 17。它不是带图形集合编辑器或厂商账户的独立应用。

README 记录 6.3.0 于 2026-10-23 发布，7.0.0-RC1 于 2026-11-12 发布。[S3][S4] 固定标签 POM 均要求 Java 17。[S19][S20] `Changelog next version` 的 Host 头改进尚未发布，不计入交付。不能沿用“核心全部是 Groovy + Apache4”的旧概括，也不能将候选版能力全部套给稳定版。

## 许可证与社区/商业边界

**SPDX：Apache-2.0。** 规范仓库自有源码与社区模块；OSI 开源许可证，可商业使用。未发现所列核心功能的付费门槛；第三方依赖各自授权，外部商业产品不在范围内。 LICENSE 是 Apache License Version 2.0，父 POM 同样声明 Apache 2.0，GitHub 仓库元数据也返回该 SPDX。[S1][S2][S19] 这是 OSI 开源许可，不是仅可阅源的商业许可；第三方付费插件、培训、支持或 SaaS 不算该库内置功能。后续 Go/Viva 改进借鉴流程和 API 设计即可，无须复制 Java 源码；如复制仍需履行许可证通知义务。

## 已实现的十项能力

| 能力 | 交付范围 | 实现与主要证据 |
| --- | --- | --- |
| 声明式 HTTP 回归测试 | 已实现（稳定版） | given/when/then 链组合请求与 Hamcrest 匹配器，验证状态、头、Cookie 和 JSON/XML 内容；同一 body 内多个期望可以汇报多处失败。测试框架负责执行，不是桌面编辑器。 [S8] |
| JSON 与 XML 路径提取 | 已实现；引擎拆分属于 7.0.0-RC1 | 历史 GPath 支持集合筛选、JSON/XmlPath 和 XPath；RC1 将 GPath 引擎拆为 json-path-gpath/xml-path-gpath，并提供 json-path-jayway。两种表达式语法不完全兼容。 [S5] |
| 多草案 JSON Schema 校验 | 已实现（6.1.0 起支持较新草案） | 从类路径、文件或字符串构造匹配器，支持 draft-06、draft-07、2019-09、2020-12。6.3.0 使用 networknt 2.0.8；RC1 改为 3.0.9/Jackson 3，旧 draft-03 和自定义 JsonSchemaFactory 需显式添加 java-json-tools。 [S9] |
| 请求及响应规范复用 | 已实现（稳定版） | RequestSpecBuilder/ResponseSpecBuilder 集中基础地址、认证、参数及共同断言。ParamConfig 明确重复参数的合并/替换策略；规范复用不代表所有 builder 都不可变。 [S10] |
| 可编程过滤器与失败日志 | 已实现（稳定版） | Filter/OrderedFilter 在请求前后拦截、检查或构造响应；支持失败时记录请求/响应及敏感头屏蔽。RC1 修复更多日志路径的屏蔽一致性，不应假设所有旧版本均覆盖。 [S11] |
| 认证与证书测试 | 已实现；传输兼容性依版本而定 | 支持 Basic、Digest、OAuth1 签名、已有 OAuth2 token、表单认证、客户端证书等。oauth2(accessToken) 使用已有令牌，不是完整浏览器授权/刷新生命周期；RC1 的 NTLM 需要 Apache5 模块。 [S13] |
| 会话与 CSRF 流程 | 已实现（稳定版） | SessionFilter/CookieFilter 复用会话与 Cookie，csrf 路径提取表单或头中的令牌并传播 Cookie，适合先登录后操作。它比仅显示响应 Cookie 更深入。 [S14] |
| Spring 内进程控制器测试 | 已实现（稳定版） | spring-mock-mvc 在 MockMvc 上使用相似 DSL，无须启动真实 HTTP 服务；另有 spring-web-test-client。MockMvc 的 async() 处理控制器异步结果，不证明通用网络客户端已有异步发送 API，也不是独立 Mock Server 产品。 [S15] |
| 客户端耗时提取与阈值断言 | 已实现（稳定版） | time/timeIn 与时间匹配器可作回归阈值。测量包含客户端及网络成本，应先预热 JVM；不能代替持续负载、吞吐量或分位延迟测试。 [S18] |
| 按客户端隔离并行默认值 | 已实现（7.0.0-RC1；尚非稳定版） | RestAssuredClient 提供不可变的基础 URI、端口和配置；given(client) 或线程绑定隔离测试，发送使用规范副本。共享 Filter、CookieFilter、SessionFilter 仍需隔离，不能宣称整个套件天然线程安全。 [S16] |

能力组合的特点是将测试放进工程：公共规范复用配置与约束，过滤器解决横切逻辑，代码使用 fixture、对象映射或 multipart 上传，运行器负责执行和报告。代价是 JVM 和依赖版本成本，也缺少交互式客户端的即时操作体验。[S3][S5][S10]

## 与当前 Reva 的直接对照

以下结论均检查实际实现；Reva 源码 URL 固定到起始基线，“未发现”限定于已检查的入口、模型和包。

### 协议与定位

**REST Assured：** Java/JVM HTTP 测试库；RC1 默认 JDK HTTP client，可选 Apache5。已查模块与 DSL 文档，未发现原生 WebSocket/gRPC 测试 DSL。

**Reva：** Go/Viva 本地客户端，支持 HTTP/HTTPS；GraphQL 是 JSON over HTTP 编辑器，另有原生 ws/wss 会话。

**取舍：** 保留交互式调试优势；HTTP 回归测试还需要执行层。

Reva 源码：[request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go)；[graphql.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/graphql.go)；[client.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/websocket/client.go)。

### 认证与登录状态

**REST Assured：** Basic/Digest、OAuth1、已有 OAuth2 token、表单/CSRF、证书和会话过滤器；NTLM 等受传输模块限制。

**Reva：** 模型与编辑器提供 None/Bearer/Basic/API-key、集合/目录继承；可发送手写 Cookie 头并查看响应 Cookie。http.Client 未配置 Jar；未实现自动登录 Cookie 复用、OAuth2 授权生命周期或客户端证书配置。

**取舍：** 优先按执行作用域增加 Cookie 会话，复杂认证仍可放入专项测试工程。

Reva 源码：[model.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go)；[auth.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/auth/auth.go)；[auth.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/auth.go)；[options.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/options.go)。

### 变量与可复用配置

**REST Assured：** Java 变量、pathParam、请求/响应规范及构建工具参数；没有与 Reva 等价的图形环境管理器。

**Reva：** VarsFor 合并集合变量与当前环境，后者覆盖前者；禁用环境项不遮蔽集合值。RunPre 返回的修改仅用于本次发送。

**取舍：** 批量执行需独立变量快照，并明确跨请求提取值的作用域。

Reva 源码：[env.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/env.go)；[vars.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/vars/vars.go)；[script.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/script.go)。

### 脚本与断言

**REST Assured：** 测试本身是 Java，可用 JVM 库和 Hamcrest；路径、Schema、对象映射由专用模块提供。

**Reva：** Otto 隔离执行前置与响应 JavaScript，默认两秒；pm.test、pm.expect、pm.response.json/text、日志及请求修改均已实现。pm.environment 与 pm.variables 共用本次变量，异步测试被拒绝。

**取舍：** 补充易用的字段断言入口；不能误称缺少脚本或完全兼容 Postman 沙箱。

Reva 源码：[script.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/script.go)；[api.js](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js)；[scripts.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/scripts.go)。

### Runner 与 CI

**REST Assured：** 作为 Maven/Gradle 测试依赖，由 JUnit/TestNG 等运行器执行并提供 CI 结果；并行和数据驱动由工程编排。

**Reva：** cmd/reva/main.go 直接启动 GUI；服务接口只有单次 Send，App.Send 执行当前页签。未发现用户集合批量 Runner、无界面测试命令或 JUnit 结果导出；仓库自身已有跨平台 CI。

**取舍：** 提取可取消的串行管线，先支持原生集合与机器可读结果，不复制 Java 测试框架。

Reva 源码：[main.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/cmd/reva/main.go)；[service.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/service.go)；[app.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/app.go)；[ci.yml](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/.github/workflows/ci.yml)。

### 导入导出与迁移

**REST Assured：** 已查 README、模块、DSL 文档与构建清单，未发现官方 Postman/HAR/curl 集合导入器；测试代码和资源随工程版本管理。

**Reva：** importer.Parse 解析原生/Postman/环境/HAR/curl/Swagger/OpenAPI JSON，并有审核确认和限制。集合导出原生 JSON，请求可复制 curl；OpenAPI 导入忽略响应 schema，未见 Java 测试生成器。

**取舍：** 保留现有迁移优势，不能承诺无损转换 REST Assured 断言或 Postman 脚本。

Reva 源码：[result.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/result.go)；[openapi.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/openapi.go)；[importer.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/importer.go)；[collections.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/collections.go)；[request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/request.go)。

### Mock 与契约校验

**REST Assured：** JSON Schema/XSD/DTD 匹配器，Spring MockMvc/WebTestClient 可测控制器，过滤器可替换响应；未发现独立持久化 Mock 服务管理器。

**Reva：** 格式化和 JSON.parse 不等于契约校验。OpenAPI 导入生成请求但省略响应约束；查 cmd、service、model、script 未见 Mock Server 或 Schema 校验入口。

**取舍：** 多草案契约与 Spring 组件测试宜接入 REST Assured 工程，避免在 GUI 中重做 JVM/Spring。

Reva 源码：[format.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/format/format.go)；[openapi.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/importer/openapi.go)；[model.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go)；[api.js](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js)；[service.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/service.go)。

### 性能与可观测性

**REST Assured：** 单次客户端时间断言，不是专用压测器；JVM 预热与网络延迟影响结果。

**Reva：** Response 有 Duration/Size/Truncated/Timeline，含 DNS、TCP、TLS、发送、等待、下载分段；脚本能断言 pm.response.responseTime。未见负载调度、并发统计或分位延迟。

**取舍：** Reva 已有时间阈值基础；持续压测交给 k6/JMeter 等专用工具，本次未调研它们的细节。

Reva 源码：[model.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/model/model.go)；[request.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/request.go)；[trace.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/service/request/trace.go)；[api.js](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/script/api.js)。

### 协作与本地数据

**REST Assured：** 测试源码、fixture 和普通报告存于本地或 CI，以 Git/code review 协作；核心不要求厂商账户，未核实到实时云工作区或商业团队版。

**Reva：** storage 原子写入本地 JSON，REVA_HOME 可覆写；syncing 已有 AES-GCM 加密快照，GUI 对接 WebDAV/S3/Gist。这不是实时多人协作或权限控制。

**取舍：** 通过可审阅集合与脱敏报告协作，保持本地免账户方向；明确导出与报告中的密钥处理。

Reva 源码：[storage.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/storage/storage.go)；[sync.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/syncing/sync.go)；[sync.go](https://github.com/runbrick/reva/blob/72bf5de0d58bbc08e49b4ef0254b1f5eeec4c910/internal/gui/sync.go)。

## 功能议题：需求、状态与交付分开核查

取样 7 个真实议题：3 个开放、4 个关闭。标题和标签保留原文，更新时间为 GitHub UTC 时间；已关闭不自动证明发布，开放也不自动说明相关能力仍然缺失。

### [#462 — Add support for asynchronous REST calls](https://github.com/rest-assured/rest-assured/issues/462)

- 状态：`open`；标签：`enhancement`、`imported`、`Priority-Medium`；更新：`2023-05-06T15:31:08Z`。
- 需求：通用 HTTP 调用希望有异步完成接口，减少线程阻塞；不能拿 MockMvc 的 async() 代替该需求。
- 交付核查：开放提案；通用异步交付未核实。评论称已有 PR 不是合并或发布证据，本报告未计入已实现功能。
- 证据：[原始页面/交付依据](https://github.com/rest-assured/rest-assured/issues/462)；[原始页面/交付依据](https://rest-assured.io/docs/spring/mock-mvc/)。

### [#287 — Make specifications immutable](https://github.com/rest-assured/rest-assured/issues/287)

- 状态：`open`；标签：`enhancement`、`imported`、`Priority-Medium`；更新：`2015-07-20T20:01:40Z`。
- 需求：共享规范时希望 setter 返回新对象，避免污染复用状态。
- 交付核查：开放提案；原始“全部 setter 返回新规范”未核实交付。RC1 的发送副本和不可变客户端只解决相关共享问题，不等于 builder 全面不可变。
- 证据：[原始页面/交付依据](https://rest-assured.io/docs/usage/parallel-tests/)；[原始页面/交付依据](https://rest-assured.io/release-notes/7-0/)。

### [#589 — Make it possible to configure how parameters are merged or overwritten](https://github.com/rest-assured/rest-assured/issues/589)

- 状态：`closed`；标签：`enhancement`；更新：`2015-10-03T05:45:03Z`。
- 需求：组合公共规范和个例时，需要明确重复参数是追加还是覆盖。
- 交付核查：已核实交付；2.6.0 变更日志明确引用 #589，现行 ParamConfig 文档演示 MERGE/REPLACE。
- 证据：[原始页面/交付依据](https://raw.githubusercontent.com/rest-assured/rest-assured/master/changelog.txt)；[原始页面/交付依据](https://rest-assured.io/docs/usage/configuration/)。

### [#551 — Validate all assertions at the same time with overloaded body/content method](https://github.com/rest-assured/rest-assured/issues/551)

- 状态：`closed`；标签：`enhancement`、`imported`、`Priority-Medium`；更新：`2015-10-03T06:21:30Z`。
- 需求：一次运行收集多个字段错误，避免每次只看到一个失败而反复重跑。
- 交付核查：已核实交付，范围限定为同一 body 的多个期望；2.5.0 变更日志说明会列出所有失败。不是跨全部链式调用的通用软断言。
- 证据：[原始页面/交付依据](https://raw.githubusercontent.com/rest-assured/rest-assured/master/changelog.txt)。

### [#550 — Add support for soft assertions](https://github.com/rest-assured/rest-assured/issues/550)

- 状态：`closed`；标签：`enhancement`、`imported`、`Priority-Medium`；更新：`2015-07-31T17:43:01Z`。
- 需求：期望 then().soft() 和 JUnit Rule 风格的跨断言聚合。
- 交付核查：已关闭，原提案未交付；维护者评论指出该设计会耦合 JUnit。本次未找到该 Rule/soft API 的发布证据，#551 的有限聚合不能代替此提案。
- 证据：[原始页面/交付依据](https://github.com/rest-assured/rest-assured/issues/550)；[原始页面/交付依据](https://github.com/rest-assured/rest-assured/issues/551)。

### [#493 — Support measuring time ](https://github.com/rest-assured/rest-assured/issues/493)

- 状态：`closed`；标签：`enhancement`、`imported`、`Priority-Low`；更新：`2015-11-19T06:52:44Z`。
- 需求：让回归测试检查客户端耗时预算。
- 交付核查：已核实交付；2.8.0 日志明确引用 #493，现行响应校验文档提供 time/timeIn、时间匹配器及预热说明。
- 证据：[原始页面/交付依据](https://raw.githubusercontent.com/rest-assured/rest-assured/master/changelog.txt)；[原始页面/交付依据](https://rest-assured.io/docs/usage/verifying-responses/)。

### [#534 — Support baseURL/port setting for parallel testing](https://github.com/rest-assured/rest-assured/issues/534)

- 状态：`open`；标签：`bug`、`imported`、`Priority-Medium`；更新：`2018-05-13T08:50:53Z`。
- 需求：同一 JVM 并行测试多个地址，避免静态 baseURI/port 相互污染；虽标 bug，揭示执行作用域的架构需求。
- 交付核查：议题仍开放，但相关能力已核实在 7.0.0-RC1 交付；候选版日志明确引用 #534，并有 RestAssuredClient 文档。不能按旧评论判断当前完全不能并行，也不能称稳定版已全面支持。
- 证据：[原始页面/交付依据](https://raw.githubusercontent.com/rest-assured/rest-assured/master/changelog.txt)；[原始页面/交付依据](https://rest-assured.io/docs/usage/parallel-tests/)。

参数合并、共享状态和错误聚合都应有明确语义；一个集合运行按钮不会自动带来可靠 CI 或并行测试。#550 与 #551 说明通用软断言与同一断言组的多错误展示是不同承诺。

## 三个最小后续 PR 建议

P0 为阻断基本使用，P1 为高价值工作流，P2 为可延后增强；没有仅凭对标就必须定为 P0 的需求。以下仅为设计，需独立授权相应实现文件。

### 1. 原生集合的串行无界面回归执行（P1）

**场景与缺口：** 开发者在 Reva 调通服务后，希望提交前与 CI 使用同一集合检查接口及脚本结果。 单次发送和脚本已存在，但流程在 App 中，缺少用户集合命令、聚合结果与退出码。

**最小 PR：** 新增独立 Go 执行包和单独命令，读取已导出的原生集合与显式环境，按树顺序串行执行，复用 Send、认证继承、变量和脚本。输出逐请求 JSON 及 0/1/2 成功/断言失败/配置错误退出码；首版不含并发、数据表、脚本联网或云任务。

**依赖：** 后续任务需授权 cmd、执行包与测试文件；提取或复用 prepareSend/afterResponse 语义，约定前置脚本错误、取消及敏感输出策略；不引入 JVM。

**验收：** 用 httptest 与临时导出集合验证三请求顺序；第二个 pm.test 失败后仍记录第三个结果且最终退出 1；无效输入退出 2 且零请求，取消后无后续请求。环境覆盖与禁用项语义正确，磁盘集合/环境字节不变，默认报告不含 Authorization、Cookie 或展开后的秘密。

### 2. 执行作用域内可清空的 Cookie 会话（P1）

**场景与缺口：** 先发送登录请求，再调用同域业务接口，无需手工复制 Set-Cookie。 Reva 可以查看 Cookie 和发送手写 Cookie 头，但缓存 http.Client 没有 Jar；直接添加全局 Jar 会混用项目与环境身份。

**最小 PR：** 以显式会话对象包装发送服务，使用标准库 cookiejar，按集合和环境隔离，提供启用/清空动作，默认只在内存保存；先做 Cookie 生命周期，不抓取 HTML CSRF 或引入完整 OAuth2。

**依赖：** 约定 service/request 与 GUI 状态的后续所有权；与 Runner 共用会话边界但可独立落地。PR 需明确默认启用策略、手写 Cookie 优先级，并保留跨来源重定向保护。

**验收：** 本地服务登录返回 HttpOnly、路径受限与 Secure Cookie；匹配路径自动携带，其他路径/域及明文 HTTP 不携带 Secure Cookie。另一集合/环境及新执行没有该 Cookie，清空后不携带；手写 Cookie 按约定覆盖同名项且不重复，重定向与取消测试不回归。

### 3. 从响应字段生成可编辑断言片段（P2）

**场景与缺口：** 不熟悉 JavaScript 的用户点击响应 JSON 字段，就能在下次发送时检查存在、类型和值。 已有 pm.test/pm.expect 和逐测试结果，但当前片段主要是状态、时间和变量，缺少字段生成入口。

**最小 PR：** 在 JSON 响应路径提供“添加断言”，只生成 pm.response.json()、安全转义的逐级键访问及现有 equal/pm.test API 脚本；每条断言独立 pm.test。首版只支持对象键与数组索引，展示路径和预期，不自建 GPath/Jayway 或承诺完整 JSON Schema。

**依赖：** 需要 response/scripts GUI 与中英 i18n 的后续授权；复用现有脚本 API，无新第三方依赖或模型字段，仍存于 Request.TestScript。

**验收：** 键名含引号/反斜线、数组、null 或缺失中间对象时，生成脚本不会执行插入代码且给出可读失败；两个错误字段得到两条失败结果。保存重开脚本不变，保留原有用户脚本，中英文入口均可用。

## 何时使用专项工具

已有 Java/Spring 服务的团队，宜将 JSON Schema/XSD、MockMvc、多步骤数据驱动回归、对象映射和复杂认证放入 REST Assured 工程；Reva 提供调通请求、整理原生集合和共享 curl 的入口。第一阶段可用“本地调试 + JVM 工程维护契约”的流程，不能承诺自动翻译 pm.* 脚本或无损导入 REST Assured。若以后需要集成，先约定请求、变量、认证和敏感数据的导出边界，再讨论有限模板生成。持续负载与分位延迟使用专项压测工具，避免把 responseTime 断言包装成性能平台。[S9][S15][S18]

## 验证与边界

文档变更没有新增行为测试；全部仓库检查均在本工作树运行。进程设置 `GOFLAGS=-p=2`、`GOMAXPROCS=2`、`GOPROXY=off`，没有安装依赖或增加互联网测试。实际退出码与输出尾部如下：

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
ok  	github.com/runbrick/reva/internal/service/request	2.782s
ok  	github.com/runbrick/reva/internal/service/store	2.764s
ok  	github.com/runbrick/reva/internal/storage	1.036s
ok  	github.com/runbrick/reva/internal/syncing	4.730s
ok  	github.com/runbrick/reva/internal/update	2.507s
ok  	github.com/runbrick/reva/internal/vars	2.011s
ok  	github.com/runbrick/reva/internal/websocket	1.510s
Exit: 0
```

JSON 以标准库解析并验证结构，Markdown 与 JSON 交叉核对能力及议题，执行 `git diff --check`。提交信息见分支日志，避免文档自引用提交哈希。

- 本机观测日期为 2026-11-16，时区 +08:00；上游资料在 22:27—22:31 获取和核对。retrieved_at 记录证据汇总时间，不是上游发布时间。
- 匿名 issues API 首次请求返回 403 配额耗尽，releases/latest 返回 404，未继续轮询。精确议题元数据来自直接 GitHub 页面的结构化数据；发布判断依 README、日志、发布说明和固定标签 POM，不能把 releases/latest 404 当作没有发布。
- 样本来自 enhancement 开放/关闭列表与直接页面，共 3 开放、4 关闭；旧 Google Code 导入条目可能长期不更新，样本不代表全量路线图或维护活跃度。额外纳入 #534，因为它揭示并行隔离的重要架构边界。
- 未运行 JVM/Maven 集成测试或安装依赖，也未用真实服务验证上游；交付判断依官方文档、源码和发布记录。7.0 文档有正式版本措辞，本报告按 README 公告将可得版本标为 7.0.0-RC1。
- master 与在线文档会变化；固定标签 POM 用于复核架构/依赖，全部链接对应本次取回资料。未逐项审计传递依赖许可证或安全性，Apache-2.0 结论仅限项目自有源码。
- Reva 未发现能力的结论仅限该基线中已检查的 cmd、model、service、GUI、script、importer 与相关包，不把 README 未提及当作缺失。TASK.md 与 .claude/skills/building-with-viva/SKILL.md 缺失，已依任务说明处理；无 GUI 或源码改动。
- 交付为本地可审阅文档 PR 变更集，没有推送、远程 PR、合并或修改 main，没有新增行为测试；三条建议是后续 PR 设计，不是本次实现。

## 编号证据来源

上游来源在上述窗口实际取回；Reva 源码链接在各对照节固定到基线提交。

1. **[S1]** [规范仓库](https://github.com/rest-assured/rest-assured)
2. **[S2]** [Apache 2.0 许可证原文](https://github.com/rest-assured/rest-assured/blob/master/LICENSE)
3. **[S3]** [README 与发布公告](https://github.com/rest-assured/rest-assured/blob/master/README.md)
4. **[S4]** [逐版本变更日志，含未发布区段](https://raw.githubusercontent.com/rest-assured/rest-assured/master/changelog.txt)
5. **[S5]** [模块选择与版本边界](https://rest-assured.io/docs/modules/)
6. **[S6]** [6.3.0 发布说明](https://rest-assured.io/release-notes/6-3/)
7. **[S7]** [7.0.0 候选版发布说明](https://rest-assured.io/release-notes/7-0/)
8. **[S8]** [JSON/XML 与 Hamcrest 示例](https://rest-assured.io/docs/usage/examples/)
9. **[S9]** [JSON Schema 草案与校验](https://rest-assured.io/docs/usage/json-schema/)
10. **[S10]** [请求及响应规范复用](https://rest-assured.io/docs/usage/specification-reuse/)
11. **[S11]** [过滤器与响应构造](https://rest-assured.io/docs/usage/filters/)
12. **[S12]** [日志与敏感头屏蔽](https://rest-assured.io/docs/usage/logging/)
13. **[S13]** [认证、CSRF 和令牌传递](https://rest-assured.io/docs/usage/authentication/)
14. **[S14]** [会话与 SessionFilter](https://rest-assured.io/docs/usage/sessions/)
15. **[S15]** [Spring MockMvc 集成](https://rest-assured.io/docs/spring/mock-mvc/)
16. **[S16]** [并行测试与 RestAssuredClient](https://rest-assured.io/docs/usage/parallel-tests/)
17. **[S17]** [配置及参数合并策略](https://rest-assured.io/docs/usage/configuration/)
18. **[S18]** [响应验证与耗时测量](https://rest-assured.io/docs/usage/verifying-responses/)
19. **[S19]** [6.3.0 固定标签构建清单](https://raw.githubusercontent.com/rest-assured/rest-assured/rest-assured-6.3.0/pom.xml)
20. **[S20]** [7.0.0-RC1 固定标签构建清单](https://raw.githubusercontent.com/rest-assured/rest-assured/rest-assured-7.0.0-RC1/pom.xml)
21. **[S21]** [6.3.0 Schema 模块依赖](https://raw.githubusercontent.com/rest-assured/rest-assured/rest-assured-6.3.0/modules/json-schema-validator/pom.xml)
22. **[S22]** [Schema 校验器实际源码](https://raw.githubusercontent.com/rest-assured/rest-assured/master/modules/json-schema-validator/src/main/java/io/restassured/module/jsv/JsonSchemaValidator.java)
23. **[S23]** [议题 #462：Add support for asynchronous REST calls](https://github.com/rest-assured/rest-assured/issues/462)
24. **[S24]** [议题 #287：Make specifications immutable](https://github.com/rest-assured/rest-assured/issues/287)
25. **[S25]** [议题 #589：Make it possible to configure how parameters are merged or overwritten](https://github.com/rest-assured/rest-assured/issues/589)
26. **[S26]** [议题 #551：Validate all assertions at the same time with overloaded body/content method](https://github.com/rest-assured/rest-assured/issues/551)
27. **[S27]** [议题 #550：Add support for soft assertions](https://github.com/rest-assured/rest-assured/issues/550)
28. **[S28]** [议题 #493：Support measuring time ](https://github.com/rest-assured/rest-assured/issues/493)
29. **[S29]** [议题 #534：Support baseURL/port setting for parallel testing](https://github.com/rest-assured/rest-assured/issues/534)
